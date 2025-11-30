package main

import "math"

// Divide matrix into blocks (identical to that in parallel)
func divideToBlocks(bp BrokerParams) []Block {
	if bp.Threads == 1 {
		blocks := make([]Block, 1)
		blocks[0] = Block{
			Start: Cell{X: 0, Y: 0},
			End:   Cell{X: bp.ImageWidth, Y: bp.ImageHeight},
		}
		return blocks
	}
	// Floor to nearest composite number
	nthread := 2
	if bp.Threads < 4 {
		nthread = bp.Threads
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
			if number == bp.Threads {
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
	horizontal := 1
	vertical := 1
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
	// Return blocks
	blocks := make([]Block, horizontal*vertical)
	part_width := float64(bp.ImageWidth) / float64(horizontal)
	part_height := float64(bp.ImageHeight) / float64(vertical)
	for y := 0; y != vertical; y++ {
		for x := 0; x != horizontal; x++ {
			start := Cell{
				X: int(math.Round(float64(x) * part_width)),
				Y: int(math.Round(float64(y) * part_height))}
			end := Cell{
				X: int(math.Round(float64(x+1) * part_width)),
				Y: int(math.Round(float64(y+1) * part_height))}
			blocks[y*horizontal+x] = Block{Start: start, End: end}
		}
	}
	return blocks
}

// Simple function grouping a set of blocks and assign them to nodes
func partitioning(nodes map[Node]struct{}, blocks []Block) []AssignedPartition {

	var assigned_partitions []AssignedPartition
	node_counts := len(nodes)
	block_counts := len(blocks)

	// Convert map to slice
	nodes_slice := make([]Node, 0, len(nodes))
	for node := range nodes {
		nodes_slice = append(nodes_slice, node)
	}

	// Assign a collection of blocks to each node
	if block_counts <= node_counts {
		// Blocks not enough to be assigned to every worker node
		assigned_partitions = make([]AssignedPartition, block_counts)
		for i := 0; i != len(blocks); i++ {
			assigned_partitions[i] = AssignedPartition{nodes_slice[i], blocks[i : i+1]}
		}
		return assigned_partitions
	} else {
		// At least one worker node gets multiple blocks to evaluate
		assigned_partitions = make([]AssignedPartition, node_counts)
		avg_blocks := float64(len(blocks)) / float64(node_counts)
		for i := 0; i != node_counts; i++ {
			start_index := int(math.Round(float64(i) * avg_blocks))
			end_index := int(math.Round(float64(i+1) * avg_blocks))
			assigned_partitions[i] = AssignedPartition{nodes_slice[i], blocks[start_index:end_index]}
		}
	}
	return assigned_partitions
}

// Produce a exchange graph which contains exchange targets for each cell
func getExchangeGraph(width, height int, assignments []AssignedPartition) [][]byte {

	// Create 2D exchange graph
	exchange_graph := make([][]byte, height)
	for y := 0; y != height; y++ {
		exchange_graph[y] = make([]byte, width)
	}

	// Convert assignments to partition slice (discard node information)
	partitions := make([]Partition, 0, len(assignments))
	for i := 0; i != len(assignments); i++ {
		partitions = append(partitions, assignments[i].Partition)
	}

	// Function getting unsafe boundary of a block
	getBoundary := func(block Block) []Cell {
		width := block.End.X - block.Start.X
		height := block.End.Y - block.Start.Y
		boundary := make([]Cell, 4+(width+height-4)*2)
		boundary_view := boundary[:]
		for x := block.Start.X; x != block.End.X-1; x++ {
			boundary_view[0] = Cell{X: x, Y: block.Start.Y}
			boundary_view = boundary_view[1:]
		}
		for y := block.Start.Y; y != block.End.Y-1; y++ {
			boundary_view[0] = Cell{X: block.End.X - 1, Y: y}
			boundary_view = boundary_view[1:]
		}
		for x := block.Start.X + 1; x != block.End.X; x++ {
			boundary_view[0] = Cell{X: x, Y: block.End.Y - 1}
			boundary_view = boundary_view[1:]
		}
		for y := block.Start.Y + 1; y != block.End.Y; y++ {
			boundary_view[0] = Cell{X: block.Start.X, Y: y}
			boundary_view = boundary_view[1:]
		}
		return boundary
	}

	// Function identifying every exchange target for a cell
	// If any surrounding cells of this cell are in another partition,
	// then that partition is an exchange target of this cell
	identifyTarget := func(cell Cell, living_partition_index int) {
		for _, surrounding := range getSurrounding(width, height, cell) {
		partition_loop:
			for partition_index, partition := range partitions {
				if partition_index == living_partition_index {
					continue
				}
				for _, block := range partition {
					if block.Start.X <= surrounding.X && surrounding.X < block.End.X &&
						block.Start.Y <= surrounding.Y && surrounding.Y < block.End.Y {
						exchange_graph[cell.Y][cell.X] |= 1 << partition_index
						break partition_loop
					}
				}
			}
		}
	}

	// For each cell at unsafe boundaries, iterating its surrounding cells
	// any surrounding cells that is in another partition set its flag in exchange graph
	for partition_index, partition := range partitions {
		for _, block := range partition {
			for _, boundary_cell := range getBoundary(block) {
				identifyTarget(boundary_cell, partition_index)
			}
		}
	}

	return exchange_graph
}
