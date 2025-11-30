// Definitions of types that are shared across broker or workers

package main

type Cell struct {
	X, Y int
}

type Block struct {
	Start Cell // Top-left corner of block
	End   Cell // Bottom-right corner of block (not inclusive)
}

type Partition []Block // A set of blocks

type WorkerParams struct {
	Turns             int
	Threads           int
	ImageWidth        int
	ImageHeight       int
	Pixels            [][]uint8 // Incomplete 2D slice storing pixels
	SurroundingCounts [][]int8  // Incomplete 2D slice storing surrounding counts
	Partition         Partition // Assigned task partition
	SizeInt           int       // Minimum number of bytes to represent the whole range of width and height
}

type TurnResult struct {
	flipped        []Cell // Slice of all the flipping cells
	unsafe_flipped []Cell // Slice of flipping cells at unsafe boundaries (cells flipped but surrounding counts not updated)
}

// Slice of cells flipped that is used to adjust surrounding counts in other partitions
type Adjustment struct {
	Increment []Cell // Surrounding counts of surrounding cells in the slice should be incremented
	Decrement []Cell // Surrounding counts of surrounding cells in the slice should be decremented
}
