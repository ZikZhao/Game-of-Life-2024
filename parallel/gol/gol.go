package gol

import "sync"

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	io := &ioState{
		params: p,
		cond:   sync.NewCond(new(sync.Mutex)),
	}
	io.cond.L.Lock()
	go startIo(io) // transfer ownership of lock to startIo

	distributorChannels := distributorChannels{
		events:     events,
		keyPresses: keyPresses,
	}
	distributor(p, io, distributorChannels)
}
