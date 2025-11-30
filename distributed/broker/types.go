// Definitions of types that are shared across broker or workers

package main

import (
	"net"
	"net/rpc"
)

type Cell struct {
	X, Y int
}

type Block struct {
	Start Cell // Top-left corner of block
	End   Cell // Bottom-right corner of block (not inclusive)
}

type Partition []Block // A set of blocks

// Structure representing a worker node
type Node struct {
	ip     string // Private IP address
	conn   *net.TCPConn
	client *rpc.Client
}

// Structure that binds a partition with a worker node
type AssignedPartition struct {
	Node      Node
	Partition Partition
}

type BrokerParams struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
	Pixels      []byte // Compressed pixel data
	Initials    []byte // Compressed positions of initial alive cells (used when Pixels is nil)
	SizeInt     int    // Minimum number of bytes to represent the whole range of width and height
}

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

// Slice of cells flipped that is used to adjust surrounding counts in other partitions
type Adjustment struct {
	Increment []Cell // Surrounding counts of surrounding cells in the slice should be incremented
	Decrement []Cell // Surrounding counts of surrounding cells in the slice should be decremented
}
