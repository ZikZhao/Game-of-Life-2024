package gol

import (
	"fmt"
	"math"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	keyPresses <-chan rune
}

type WorkerParams struct {
	p           Params            // Input parameters
	matrix      Matrix            // Read from this matrix
	next_matrix Matrix            // Write to this matrix
	start       util.Cell         // Top-left corner of cell partition allocated
	end         util.Cell         // Bottom-right corner of cell partition allocated (not inclusive)
	running     *bool             // Volatile variable to instruct routines to stop when set to false (read-write protected by condition variable)
	cond        *sync.Cond        // Condition variable for worker routines wait for distributor collecting results
	result_chan chan<- TurnResult // Result send to distributor after each turn
	event_chan  chan<- Event      // CellsFlipped event channel
}

type TurnResult struct {
	count_diff     int         // Difference in alive cell count
	unsafe_flipped []util.Cell // Slice of flipping cells at unsafe boundaries (cells flipped but surrounding counts not updated)
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, io *ioState, c distributorChannels) {

	defer io.quit()

	// Read file
	operation := ioOperation{
		command:  ioInput,
		filename: fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight),
	}
	io.sendIoRequest(&operation)
	io.waitIoRequest() // Wait for last pending request completing

	// Create cell matrix and load pixel data
	matrix := MakeMatrixFromData(p, operation.data)
	next_matrix := MakeMatrix(p)
	count := 0
	{
		flipping_buffer := make([]util.Cell, 1024)
		for i := 0; i != p.ImageHeight; i++ {
			for j := 0; j != p.ImageWidth; j++ {
				if matrix.pixels[i][j] != 0 {
					count++
					flipping_buffer = append(flipping_buffer, util.Cell{X: j, Y: i})
					for _, cell := range matrix.getSurrounding(util.Cell{X: j, Y: i}) {
						matrix.surrounding_counts[cell.Y][cell.X]++
					}
				}
			}
		}
		c.events <- CellsFlipped{0, flipping_buffer}
	} // This scope removes flipping_buffer reference to help garbage collection

	// Create goroutines
	blocks := divideToBlocks(p)
	nthread := len(blocks)
	running_flag := true // Exit all routines when set to false
	pause_flag := false  // Skip cond.broadcast when set to true
	cond := sync.NewCond(new(sync.Mutex))
	result_chan := make(chan TurnResult)
	result_buffer := make([]TurnResult, nthread)
	for i := 0; i != nthread; i++ {
		wp := WorkerParams{
			p:           p,
			matrix:      matrix,
			next_matrix: next_matrix,
			start:       blocks[i].start,
			end:         blocks[i].end,
			running:     &running_flag,
			cond:        cond,
			result_chan: result_chan,
			event_chan:  c.events,
		}
		go worker(wp)
		<-result_chan // Make sure goroutine is ready
	}

	// Write file function
	write := func(turn int) {
		filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
		operation := &ioOperation{
			command:  ioOutput,
			filename: filename,
			data:     matrix.pixels[0][0 : p.ImageWidth*p.ImageHeight],
		}
		io.sendIoRequest(operation)
		io.waitIoRequest() // Wait for last pending request completing
		c.events <- ImageOutputComplete{turn, filename}
	}

	// Alive timer
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	// Evaluate each turn
	turn := 0
	c.events <- StateChange{turn, Executing}
	for turn != p.Turns {
		// Broadcast as critical section to prevent any routine not in waiting state before broadcast
		cond.L.Lock()
		cond.Broadcast()
		cond.L.Unlock()
		// Get results for current turn
		for thread_index := 0; thread_index != nthread; thread_index++ {
			result_buffer[thread_index] = <-result_chan
		}
		// All routines completed current turn
		for thread_index := 0; thread_index != nthread; thread_index++ {
			count += result_buffer[thread_index].count_diff
			for _, cell := range result_buffer[thread_index].unsafe_flipped {
				if matrix.pixels[cell.Y][cell.X] == 0 {
					for _, surrounding := range matrix.getSurrounding(cell) {
						next_matrix.surrounding_counts[surrounding.Y][surrounding.X]++
					}
				} else {
					for _, surrounding := range matrix.getSurrounding(cell) {
						next_matrix.surrounding_counts[surrounding.Y][surrounding.X]--
					}
				}
			}
		}
		// Swap current and next matrix
		matrix, next_matrix = next_matrix, matrix
		// Turn completed
		turn++
		c.events <- TurnComplete{turn}
		// Handle events
	handle:
		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{turn, count}
		case char := <-c.keyPresses:
			switch char {
			case 's':
				write(turn)
			case 'q':
				goto quit
			case 'p':
				pause_flag = !pause_flag
				if pause_flag {
					c.events <- StateChange{turn, Paused}
				} else {
					c.events <- StateChange{turn, Executing}
				}
			}
		default:
		}
		if pause_flag {
			goto handle
		}
	}

quit:
	// Set flag variable to exit all worker routines
	cond.L.Lock()
	running_flag = false
	cond.Broadcast()
	cond.L.Unlock()
	cells := make([]util.Cell, count)
	cells_index := 0
	for i := 0; i != p.ImageHeight; i++ {
		for j := 0; j != p.ImageWidth; j++ {
			if matrix.pixels[i][j] != 0 {
				cells[cells_index] = util.Cell{X: j, Y: i}
				cells_index++
			}
		}
	}
	c.events <- FinalTurnComplete{turn, cells}

	// Write file
	write(turn)

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func worker(wp WorkerParams) {
	// Wait until distributor routine finishes initialisation
	flipping_buffer := make([]util.Cell, 0, 1024)
	unsafe_flipping_buffer := make([]util.Cell, 0, 64)
	turn := 0
	wp.cond.L.Lock()
	wp.result_chan <- TurnResult{} // notify distributor that this routine is ready
	wp.cond.Wait()
	wp.cond.L.Unlock()
	// Work for each turn
	for *wp.running {
		count_diff := 0
		// Clean up surrounding counts
		for y := wp.start.Y; y != wp.end.Y; y++ {
			for x := wp.start.X; x != wp.end.X; x++ {
				wp.next_matrix.surrounding_counts[y][x] = wp.matrix.surrounding_counts[y][x]
			}
		}
		// Safe region (absolutely no data races)
		for y := wp.start.Y + 1; y != wp.end.Y-1; y++ {
			for x := wp.start.X + 1; x != wp.end.X-1; x++ {
				count_diff += wp.matrix.checkAndFlip(util.Cell{X: x, Y: y}, &wp.next_matrix, &flipping_buffer)
			}
		}
		// Unsafe boundaries (changing surrounding count of surrounding cells causes data races)
		for x := wp.start.X; x != wp.end.X-1; x++ {
			count_diff += wp.matrix.checkAndFlipUnsafe(util.Cell{X: x, Y: wp.start.Y},
				&wp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}
		for y := wp.start.Y; y != wp.end.Y-1; y++ {
			count_diff += wp.matrix.checkAndFlipUnsafe(util.Cell{X: wp.end.X - 1, Y: y},
				&wp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}
		for x := wp.start.X + 1; x != wp.end.X; x++ {
			count_diff += wp.matrix.checkAndFlipUnsafe(util.Cell{X: x, Y: wp.end.Y - 1},
				&wp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}
		for y := wp.start.Y + 1; y != wp.end.Y; y++ {
			count_diff += wp.matrix.checkAndFlipUnsafe(util.Cell{X: wp.start.X, Y: y},
				&wp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}
		// Switch next matrix to current matrix
		wp.matrix, wp.next_matrix = wp.next_matrix, wp.matrix
		// Send CellsFlipped event
		copied := make([]util.Cell, len(flipping_buffer))
		copy(copied, flipping_buffer)
		wp.event_chan <- CellsFlipped{turn, copied}
		turn++
		// Send turn result to distributor
		wp.cond.L.Lock()
		wp.result_chan <- TurnResult{
			count_diff:     count_diff,
			unsafe_flipped: unsafe_flipping_buffer,
		}
		// Clear slice
		flipping_buffer = flipping_buffer[0:0]
		unsafe_flipping_buffer = unsafe_flipping_buffer[0:0]
		// Wait for other workers completing current turn
		wp.cond.Wait()
		wp.cond.L.Unlock()
	}
}

func divideToBlocks(p Params) []struct{ start, end util.Cell } {
	if p.Threads == 1 {
		result := make([]struct{ start, end util.Cell }, 1)
		result[0] = struct{ start, end util.Cell }{
			start: util.Cell{X: 0, Y: 0},
			end:   util.Cell{X: p.ImageWidth, Y: p.ImageHeight},
		}
		return result
	}
	// Floor to nearest composite number
	nthread := 2
	if p.Threads < 4 {
		nthread = p.Threads
	} else {
		for number := 2; ; number++ {
			is_prime := true
			for factor := 2; factor != number; factor++ {
				if (number % factor) == 0 {
					is_prime = false
					break
				}
			}
			if !is_prime {
				nthread = number
			}
			if number == p.Threads {
				break
			}
		}
	}
	// Factor decomposition
	factors := make([]int, 0)
	for number := nthread; number != 1; {
		for factor := 2; ; factor++ {
			if number%factor == 0 {
				number /= factor
				factors = append(factors, factor)
				break
			}
		}
	}
	// Find moderate partitioning
	i := 0
	desired := math.Pow(float64(nthread), 0.5)
	vertical := 1
	horizontal := 1
	for ; i != len(factors); i++ {
		if float64(vertical) < desired {
			vertical *= factors[len(factors)-i-1]
		} else {
			break
		}
	}
	for ; i != len(factors); i++ {
		horizontal *= factors[len(factors)-i-1]
	}
	// Return partitions
	partitions := make([]struct{ start, end util.Cell }, horizontal*vertical)
	part_width := float64(p.ImageWidth) / float64(horizontal)
	part_height := float64(p.ImageHeight) / float64(vertical)
	for y := 0; y != vertical; y++ {
		for x := 0; x != horizontal; x++ {
			start := util.Cell{
				X: int(math.Round(float64(x) * part_width)),
				Y: int(math.Round(float64(y) * part_height))}
			end := util.Cell{
				X: int(math.Round(float64(x+1) * part_width)),
				Y: int(math.Round(float64(y+1) * part_height))}
			partitions[y*horizontal+x] = struct{ start, end util.Cell }{start, end}
		}
	}
	return partitions
}
