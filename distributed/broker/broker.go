package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sync"
)

func main() {

	// Create broker singleton
	broker := &Broker{
		cond: sync.NewCond(new(sync.Mutex)),
		flag: sync.WaitGroup{},
	}
	broker.flag.Add(1)

	// Register RPC service
	rpc.Register(broker)
	rpc.HandleHTTP()

	// Start RPC handling service
	listener, err := net.Listen("tcp", ":2000")
	if err != nil {
		log.Panic(err.Error())
	}
	go http.Serve(listener, nil)

	// Accept connection request from local controller
	go func() {
		listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 2001})
		if err != nil {
			log.Panic(err.Error())
		}
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				log.Panic(err.Error())
			}
			broker.cond.L.Lock()
			broker.local_conn = &Connection{conn: conn, mutex: new(sync.Mutex)}
			log.Printf("Connection to %s established", conn.RemoteAddr().String())
			broker.cond.Signal()
			broker.cond.L.Unlock()
		}
	}()

	// Accepting connection requests from worker nodes and monitor their status
	go monitorNodes()

	broker.flag.Wait()
	broker.cond.L.Lock()
	broker.cond.L.Unlock()

	shutdownNodes()
}

type Broker struct {
	cond           *sync.Cond
	local_conn     *Connection
	event_chan     chan byte
	bp             BrokerParams
	turn           int
	matrix         Matrix
	exchange_graph [][]byte

	flag sync.WaitGroup
}

func (broker *Broker) Init(bp BrokerParams, reply *struct{}) error {

	log.Printf("Init: %dx%dx%d-%d", bp.ImageWidth, bp.ImageWidth, bp.Turns, bp.Threads)

	// Wait for connection
	broker.cond.L.Lock()
	if broker.local_conn == nil {
		broker.cond.Wait()
	}

	// Reset broker instance status
	broker.bp = bp
	broker.turn = 0
	broker.event_chan = make(chan byte, 1)

	// Partitioning
	nodes := getAvailableNodes()
	if len(nodes) == 0 {
		return errors.New("no worker nodes available")
	}
	blocks := divideToBlocks(bp)
	assignments := partitioning(nodes, blocks)
	broker.exchange_graph = getExchangeGraph(bp.ImageWidth, bp.ImageHeight, assignments)

	// Decompress pixel data
	pixels, surrounding_counts := decompressMatrix(&bp)
	broker.matrix = MakeMatrixFromData(pixels, surrounding_counts)

	// Dispatch matrix data
	call_chan := make(chan *rpc.Call, len(assignments))
	defer close(call_chan)
	for _, assignment := range assignments {
		// Transmit rows in partition only
		pixels_in_partition := make([][]uint8, broker.bp.ImageHeight)
		surrounding_counts_in_partition := make([][]int8, broker.bp.ImageHeight)
		for _, block := range assignment.Partition {
			for y := block.Start.Y; y != block.End.Y; y++ {
				if pixels_in_partition[y] == nil {
					pixels_in_partition[y] = pixels[y]
					surrounding_counts_in_partition[y] = surrounding_counts[y]
				}
			}
		}
		wp := WorkerParams{
			Turns:             bp.Turns,
			Threads:           bp.Threads,
			ImageWidth:        bp.ImageWidth,
			ImageHeight:       bp.ImageHeight,
			Pixels:            pixels_in_partition,
			SurroundingCounts: surrounding_counts_in_partition,
			Partition:         assignment.Partition,
			SizeInt:           bp.SizeInt,
		}
		var reply struct{}
		assignment.Node.client.Go("Worker.Init", wp, &reply, call_chan)
	}

	// Check if all RPC calls succeeded
	for range assignments {
		call := <-call_chan
		if call.Error != nil {
			saved_local_conn := broker.local_conn
			broker.local_conn = nil
			broker.cond.L.Unlock()
			return broker.recover_evaluation(saved_local_conn)
		}
	}

	// Create loop goroutine
	go broker.loop(assignments)

	return nil
}

func (broker *Broker) Resume(struct{}, *struct{}) error {

	log.Print("Resume")
	broker.event_chan <- EVENT_RESUME
	broker.cond.Broadcast()
	return nil
}

func (broker *Broker) Pause(struct{}, *struct{}) error {

	log.Print("Pause")
	broker.event_chan <- EVENT_PAUSE
	return nil
}

func (broker *Broker) Save(struct{}, *struct{}) error {

	log.Print("Save")
	broker.event_chan <- EVENT_SAVE
	return nil
}

func (broker *Broker) Quit(struct{}, *struct{}) error {

	log.Print("Quit")
	broker.event_chan <- EVENT_QUIT
	return nil
}

func (broker *Broker) Kill(struct{}, *struct{}) error {

	log.Print("Kill")
	broker.event_chan <- EVENT_KILL
	return nil
}

func (broker *Broker) loop(assignments []AssignedPartition) {

	defer broker.cond.L.Unlock()
	defer func() {
		if broker.local_conn != nil {
			broker.local_conn.conn.Close()
			log.Print("Connection closed: " + broker.local_conn.conn.RemoteAddr().String())
			broker.local_conn = nil
		}
	}()
	defer close(broker.event_chan)
	defer func() { recover() }()

	// Create buffer for storing RPC results
	call_buffer := make([]*rpc.Call, len(assignments))

	// Create buffers for adjustment
	adjustment_buffers := make([]Adjustment, len(assignments))
	for i := 0; i != len(assignments); i++ {
		adjustment_buffers[i] = Adjustment{
			Increment: make([]Cell, 0, 1024),
			Decrement: make([]Cell, 0, 1024),
		}
	}

	// Channels for asynchorous call
	call_chan := make(chan *rpc.Call, len(assignments))
	defer close(call_chan)

	// Evaluate all turns
	pause_flag := false
	for ; broker.turn != broker.bp.Turns; broker.turn++ {

		// Instruct worker nodes to evaluate next turn
		for i, assignment := range assignments {
			var flipped []byte
			assignment.Node.client.Go("Worker.Next", adjustment_buffers[i], &flipped, call_chan)
		}

		// Clear adjustment buffers
		for i := range assignments {
			adjustment_buffers[i].Increment = adjustment_buffers[i].Increment[0:0]
			adjustment_buffers[i].Decrement = adjustment_buffers[i].Decrement[0:0]
		}

		// Check if all RPC calls succeeded
		successful := true
		for i := range assignments {
			call := <-call_chan
			if call.Error != nil {
				log.Print(call.Error.Error())
				successful = false
			}
			call_buffer[i] = call
		}

		if !successful {
			// Recover task when any worker nodes getting offline
			saved_local_conn := broker.local_conn
			broker.local_conn = nil
			go broker.recover_evaluation(saved_local_conn)
			return
		}

		// Apply flipping results
		for i := range assignments {
			flipped_data := *call_buffer[i].Reply.(*[]byte)
			flipped := decompressFlipped(flipped_data, broker.bp.SizeInt)
			broker.local_conn.writeCompressedFlipped(flipped_data)
			broker.updateMatrixAndGetAdjustments(flipped, adjustment_buffers)
		}
		broker.local_conn.writeEvent(EVENT_TURN_COMPLETE)

		// Handle events from local controller
		for {
			select {
			case event := <-broker.event_chan:
				broker.local_conn.writeEvent(event)
				switch event {
				case EVENT_PAUSE:
					pause_flag = true
				case EVENT_RESUME:
					pause_flag = false
				case EVENT_QUIT:
					return
				case EVENT_KILL:
					broker.flag.Done()
					return
				}
			default:
			}
			if !pause_flag {
				break
			}
		}
	}
}

// Recover evaluation task from unexpected faliure of RPC to worker
func (broker *Broker) recover_evaluation(saved_local_conn *Connection) error {

	broker.cond.L.Lock()

	log.Printf("Recover: %dx%dx%d-%d (from %d)", broker.bp.ImageWidth, broker.bp.ImageWidth,
		broker.bp.Turns, broker.bp.Threads, broker.turn)

	// Reset broker status
	bp := broker.bp
	broker.event_chan = make(chan byte, 1)
	broker.local_conn = saved_local_conn

	// Partitioning
	nodes := getAvailableNodes()
	if len(nodes) == 0 {
		return errors.New("no worker nodes available")
	}
	blocks := divideToBlocks(bp)
	assignments := partitioning(nodes, blocks)
	broker.exchange_graph = getExchangeGraph(bp.ImageWidth, bp.ImageHeight, assignments)

	// Dispatch matrix data
	call_chan := make(chan *rpc.Call, len(assignments))
	defer close(call_chan)
	for _, assignment := range assignments {
		// Transmit rows in partition only
		pixels_in_partition := make([][]uint8, bp.ImageHeight)
		surrounding_counts_in_partition := make([][]int8, bp.ImageHeight)
		for _, block := range assignment.Partition {
			for y := block.Start.Y; y != block.End.Y; y++ {
				if pixels_in_partition[y] == nil {
					pixels_in_partition[y] = broker.matrix.pixels[y]
					surrounding_counts_in_partition[y] = broker.matrix.surrounding_counts[y]
				}
			}
		}
		wp := WorkerParams{
			Turns:             bp.Turns,
			Threads:           bp.Threads,
			ImageWidth:        bp.ImageWidth,
			ImageHeight:       bp.ImageHeight,
			Pixels:            pixels_in_partition,
			SurroundingCounts: surrounding_counts_in_partition,
			Partition:         assignment.Partition,
			SizeInt:           bp.SizeInt,
		}
		var reply struct{}
		assignment.Node.client.Go("Worker.Init", wp, &reply, call_chan)
	}

	// Check if all RPC calls succeeded
	for range assignments {
		call := <-call_chan
		if call.Error != nil {
			saved_local_conn := broker.local_conn
			broker.local_conn = nil
			broker.cond.L.Unlock()
			return broker.recover_evaluation(saved_local_conn)
		}
	}

	// Create loop goroutine
	go broker.loop(assignments)

	return nil
}
