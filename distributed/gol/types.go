// Definitions of types that are shared across broker or workers

package gol

import "uk.ac.bris.cs/gameoflife/util"

type Block struct {
	Start util.Cell // Top-left corner of block
	End   util.Cell // Bottom-right corner of block (not inclusive)
}

type Partition []Block // A set of blocks

type BrokerParams struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	Pixels      []byte // Compressed pixel data
	Initials    []byte // Compressed positions of initial alive cells (used when Pixels is nil)
	SizeInt     int    // Minimum number of bytes to represent the whole range of width and height
}
