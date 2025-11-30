package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	keyPresses <-chan rune
}

// Create RPC client
var client = func() *rpc.Client {
	c, err := rpc.DialHTTP("tcp", "54.209.41.143:2000")
	if err != nil {
		log.Panic(err.Error())
	}
	log.Print("RPC Server 54.209.41.143 connected")
	return c
}()

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, io *ioState, c distributorChannels) {

	defer io.quit()

	// Start Reading file
	operation := ioOperation{
		command:  ioInput,
		filename: fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight),
	}
	io.sendIoRequest(&operation)

	// Establish connection for data streaming
	size_int := getSizeOfInt(p.ImageWidth, p.ImageHeight)
	conn := NewConnection(size_int)

	// Wait for pending read request
	io.waitIoRequest()

	// Keep a local copy of pixel matrix
	count := 0
	matrix := make([][]uint8, p.ImageHeight)
	flipping_buffer := make([]util.Cell, 0, 1024)
	for y := 0; y != p.ImageHeight; y++ {
		matrix[y] = operation.data[y*p.ImageWidth : (y+1)*p.ImageWidth]
		for x := 0; x != p.ImageWidth; x++ {
			if matrix[y][x] != 0 {
				count++
				flipping_buffer = append(flipping_buffer, util.Cell{X: x, Y: y})
			}
		}
	}

	// Prepare task
	var reply struct{}
	bp := BrokerParams{
		Turns:       p.Turns,
		Threads:     p.Threads,
		ImageWidth:  p.ImageWidth,
		ImageHeight: p.ImageHeight,
	}
	compressMatrix(&bp, operation.data, flipping_buffer)
	err := client.Call("Broker.Init", bp, &reply)
	if err != nil {
		log.Panic(err.Error())
	}
	log.Printf("Broker.Init: %dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)

	// Write file function
	write := func(turn int) {
		filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
		operation := &ioOperation{
			command:  ioOutput,
			filename: filename,
			data:     matrix[0][0 : p.ImageWidth*p.ImageHeight],
		}
		io.sendIoRequest(operation)
		io.waitIoRequest()
		c.events <- ImageOutputComplete{turn, filename}
	}

	// Alive timer
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	// Handle events
	turn := 0
	uncomfirmed_count := count
	pause_flag := false
	c.events <- CellsFlipped{0, flipping_buffer}
	c.events <- StateChange{turn, Executing}
	for turn != p.Turns {
		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{turn, count}
		case char := <-c.keyPresses:
			switch char {
			case 's':
				err = client.Call("Broker.Save", struct{}{}, &struct{}{})
				if err != nil {
					log.Panic(err.Error())
				}
			case 'q':
				err = client.Call("Broker.Quit", struct{}{}, &struct{}{})
				if err != nil {
					log.Panic(err.Error())
				}
			case 'p':
				pause_flag = !pause_flag
				if pause_flag {
					err = client.Call("Broker.Pause", struct{}{}, &struct{}{})
					if err != nil {
						log.Panic(err.Error())
					}
				} else {
					err = client.Call("Broker.Resume", struct{}{}, &struct{}{})
					if err != nil {
						log.Panic(err.Error())
					}
				}
			case 'k':
				err = client.Call("Broker.Kill", struct{}{}, &struct{}{})
				if err != nil {
					log.Panic(err.Error())
				}
			}
		case flipped := <-conn.result_chan:
			for _, cell := range flipped {
				if matrix[cell.Y][cell.X] == 0 {
					matrix[cell.Y][cell.X] = 255
					uncomfirmed_count++
				} else {
					matrix[cell.Y][cell.X] = 0
					uncomfirmed_count--
				}
			}
			c.events <- CellsFlipped{turn, flipped}
		case event := <-conn.event_chan:
			switch event {
			case EVENT_TURN_COMPLETE:
				c.events <- TurnComplete{turn}
				count = uncomfirmed_count
				turn++
				log.Printf("Turn result [%d] collected", turn)
			case EVENT_RESUME:
				c.events <- StateChange{turn, Executing}
				log.Print("Continuing")
			case EVENT_PAUSE:
				c.events <- StateChange{turn, Paused}
			case EVENT_SAVE:
				write(turn)
			case EVENT_KILL:
				log.Print("Remote system shutdowns")
				fallthrough
			case EVENT_QUIT:
				goto quit
			}
		}
	}

quit:
	cells := make([]util.Cell, count)
	cells_index := 0
	for i := 0; i != p.ImageHeight; i++ {
		for j := 0; j != p.ImageWidth; j++ {
			if matrix[i][j] != 0 {
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
