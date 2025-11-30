package main

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
	"time"
)

func main() {
	// Create worker instance
	instance := &Worker{
		running:     new(bool),
		cond:        sync.NewCond(new(sync.Mutex)),
		result_chan: make(chan TurnResult),
		flag:        sync.WaitGroup{},
	}
	instance.flag.Add(1)

	// Register functions
	rpc.Register(instance)
	rpc.HandleHTTP()

	// Start RPC handling service
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 2000})
	if err != nil {
		log.Panic(err.Error())
	}
	go http.Serve(listener, nil)

	// Registering worker node to broker
	go func() {
		for {
			var conn *net.TCPConn
			for {
				log.Print("Registering worker to broker")
				conn, err = net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.IPv4(172, 31, 46, 226), Port: 2002})
				if err == nil {
					log.Print("Worker registered")
					break
				}
				time.Sleep(time.Second * 1)
			}
			conn.SetKeepAlive(true)
			conn.SetReadDeadline(*new(time.Time))
			conn.Read(make([]byte, 1))
			log.Print("Broker disconnected")
			conn.Close()
		}
	}()

	instance.flag.Wait()
}

type Worker struct {
	wp          WorkerParams
	matrix      Matrix
	next_matrix Matrix
	running     *bool
	cond        *sync.Cond
	result_chan chan TurnResult

	flag sync.WaitGroup
}

type LogicWorkerParams struct {
	matrix      Matrix
	next_matrix Matrix
	running     *bool
	cond        *sync.Cond
	result_chan chan<- TurnResult
	start       Cell
	end         Cell
}

func (worker *Worker) Init(wp WorkerParams, reply *struct{}) error {

	log.Printf("Init: %dx%dx%d-%d (%d blocks assigned)",
		wp.ImageWidth, wp.ImageWidth, wp.Turns, wp.Threads, len(wp.Partition))

	// Cancel last task if not completed
	*worker.running = false
	worker.cond.Broadcast()

	// Load matrix data
	worker.wp = wp
	worker.matrix = MakeMatrixFromData(wp)
	worker.next_matrix = MakeMatrix(wp)

	// Reset worker instance status
	worker.running = new(bool)
	*worker.running = true
	for thread_index := 0; thread_index != len(wp.Partition); thread_index++ {
		block := wp.Partition[thread_index]
		lwp := LogicWorkerParams{
			matrix:      worker.matrix,
			next_matrix: worker.next_matrix,
			running:     worker.running,
			cond:        worker.cond,
			result_chan: worker.result_chan,
			start:       block.Start,
			end:         block.End,
		}
		go logic_worker(lwp)
		<-worker.result_chan // Make sure logic worker is ready
	}

	return nil
}

func (worker *Worker) Next(adjustment Adjustment, flipped_data *[]byte) error {

	// Apply adjustments from other boundaries of other partitions
	worker.matrix.applyAdjustment(adjustment)

	// Broadcast as critical section to prevent any routine not in waiting state before broadcast
	worker.cond.L.Lock()
	worker.cond.Broadcast()
	worker.cond.L.Unlock()

	// Get results for current turn
	result_buffer := make([]TurnResult, len(worker.wp.Partition))
	for thread_index := 0; thread_index != len(worker.wp.Partition); thread_index++ {
		result_buffer[thread_index] = <-worker.result_chan
	}

	// All routines completed current turn
	// Collect flipping flipping cells and update unsafe boundaries
	flipped_total := 0
	for thread_index := 0; thread_index != len(worker.wp.Partition); thread_index++ {
		flipped_total += len(result_buffer[thread_index].flipped)
	}
	*flipped_data = make([]byte, flipped_total*worker.wp.SizeInt*2)
	flipped_data_view := (*flipped_data)[:]
	for thread_index := 0; thread_index != len(worker.wp.Partition); thread_index++ {
		turn_result := result_buffer[thread_index]
		flipped_data_view = compressFlippedTo(turn_result.flipped, flipped_data_view, worker.wp.SizeInt)
		for _, cell := range turn_result.unsafe_flipped {
			worker.matrix.updateUnsafe(cell, &worker.next_matrix)
		}
	}

	// Swap current and next matrix
	worker.matrix, worker.next_matrix = worker.next_matrix, worker.matrix

	return nil
}

func (worker *Worker) Kill(struct{}, *struct{}) error {

	log.Print("Kill")
	worker.flag.Done()
	return nil
}

func logic_worker(lwp LogicWorkerParams) {

	// Wait until Init routine finishes initialisation
	flipping_buffer := make([]Cell, 0, 1024)
	unsafe_flipping_buffer := make([]Cell, 0, 64)
	lwp.cond.L.Lock()
	lwp.result_chan <- TurnResult{} // notify distributor that this routine is ready
	lwp.cond.Wait()
	lwp.cond.L.Unlock()

	// Work for each turn
	for *lwp.running {

		// Update surrounding counts
		for y := lwp.start.Y; y != lwp.end.Y; y++ {
			for x := lwp.start.X; x != lwp.end.X; x++ {
				lwp.next_matrix.surrounding_counts[y][x] = lwp.matrix.surrounding_counts[y][x]
			}
		}

		// Safe region (absolutely no data races)
		for y := lwp.start.Y + 1; y != lwp.end.Y-1; y++ {
			for x := lwp.start.X + 1; x != lwp.end.X-1; x++ {
				lwp.matrix.checkAndFlip(Cell{X: x, Y: y}, &lwp.next_matrix, &flipping_buffer)
			}
		}

		// Unsafe boundaries (changing surrounding count of surrounding cells causes data races)
		for x := lwp.start.X; x != lwp.end.X-1; x++ {
			lwp.matrix.checkAndFlipUnsafe(Cell{X: x, Y: lwp.start.Y},
				&lwp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}
		for y := lwp.start.Y; y != lwp.end.Y-1; y++ {
			lwp.matrix.checkAndFlipUnsafe(Cell{X: lwp.end.X - 1, Y: y},
				&lwp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}
		for x := lwp.start.X + 1; x != lwp.end.X; x++ {
			lwp.matrix.checkAndFlipUnsafe(Cell{X: x, Y: lwp.end.Y - 1},
				&lwp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}
		for y := lwp.start.Y + 1; y != lwp.end.Y; y++ {
			lwp.matrix.checkAndFlipUnsafe(Cell{X: lwp.start.X, Y: y},
				&lwp.next_matrix, &flipping_buffer, &unsafe_flipping_buffer)
		}

		// Switch next matrix to current matrix
		lwp.matrix, lwp.next_matrix = lwp.next_matrix, lwp.matrix

		// Send turn result to caller
		lwp.cond.L.Lock()
		lwp.result_chan <- TurnResult{
			flipped:        flipping_buffer,
			unsafe_flipped: unsafe_flipping_buffer,
		}

		// Clear slice
		flipping_buffer = flipping_buffer[0:0]
		unsafe_flipping_buffer = unsafe_flipping_buffer[0:0]

		// Wait for other workers completing current turn
		lwp.cond.Wait()
		lwp.cond.L.Unlock()
	}
}
