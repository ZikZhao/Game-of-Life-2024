package main

type Matrix struct {
	width              int
	height             int
	pixels             [][]uint8
	surrounding_counts [][]int8
	partition          Partition
}

// Make matrix object with empty data
func MakeMatrix(wp WorkerParams) Matrix {
	matrix := Matrix{
		width:              wp.ImageWidth,
		height:             wp.ImageHeight,
		pixels:             make([][]uint8, wp.ImageHeight),
		surrounding_counts: make([][]int8, wp.ImageHeight),
		partition:          wp.Partition,
	}
	for i := 0; i != matrix.width; i++ {
		matrix.pixels[i] = make([]uint8, len(wp.Pixels[i]))
		matrix.surrounding_counts[i] = make([]int8, len(wp.Pixels[i]))
	}
	return matrix
}

// Make matrix object by providing pixel array
// Ownership of pixel array is transferred to matrix object
func MakeMatrixFromData(wp WorkerParams) Matrix {
	return Matrix{
		width:              wp.ImageWidth,
		height:             wp.ImageHeight,
		pixels:             wp.Pixels,
		surrounding_counts: wp.SurroundingCounts,
		partition:          wp.Partition,
	}
}

// Get positions of eight surrounding cells
func (matrix *Matrix) getSurrounding(cell Cell) [8]Cell {
	if cell.X == 0 || cell.Y == 0 || cell.X == matrix.width-1 || cell.Y == matrix.height-1 {
		return [8]Cell{
			{X: (cell.X - 1 + matrix.width) % matrix.width, Y: (cell.Y - 1 + matrix.height) % matrix.height},
			{X: cell.X, Y: (cell.Y - 1 + matrix.height) % matrix.height},
			{X: (cell.X + 1) % matrix.width, Y: (cell.Y - 1 + matrix.height) % matrix.height},
			{X: (cell.X - 1 + matrix.width) % matrix.width, Y: cell.Y},
			{X: (cell.X + 1) % matrix.width, Y: cell.Y},
			{X: (cell.X - 1 + matrix.width) % matrix.width, Y: (cell.Y + 1) % matrix.height},
			{X: cell.X, Y: (cell.Y + 1) % matrix.height},
			{X: (cell.X + 1) % matrix.width, Y: (cell.Y + 1) % matrix.height},
		}
	} else {
		return [8]Cell{
			{X: cell.X - 1, Y: cell.Y - 1},
			{X: cell.X, Y: cell.Y - 1},
			{X: cell.X + 1, Y: cell.Y - 1},
			{X: cell.X - 1, Y: cell.Y},
			{X: cell.X + 1, Y: cell.Y},
			{X: cell.X - 1, Y: cell.Y + 1},
			{X: cell.X, Y: cell.Y + 1},
			{X: cell.X + 1, Y: cell.Y + 1},
		}
	}
}

// Check and flip cells if conditions satisfied
// Changes surrounding counts of surrounding cells if flipped
// Return alive cell count difference
func (matrix *Matrix) checkAndFlip(cell Cell, next_matrix *Matrix, flipping_buffer *[]Cell) {
	if matrix.pixels[cell.Y][cell.X] == 0 {
		if matrix.surrounding_counts[cell.Y][cell.X] == 3 {
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 255
			for _, surrounding := range matrix.getSurrounding(cell) {
				next_matrix.surrounding_counts[surrounding.Y][surrounding.X]++
			}
			*flipping_buffer = append(*flipping_buffer, cell)
		} else {
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 0
		}
	} else {
		switch matrix.surrounding_counts[cell.Y][cell.X] {
		case 2:
			fallthrough
		case 3:
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 255
		default:
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 0
			for _, surrounding := range matrix.getSurrounding(cell) {
				next_matrix.surrounding_counts[surrounding.Y][surrounding.X]--
			}
			*flipping_buffer = append(*flipping_buffer, cell)
		}
	}
}

// Check and flip cells if conditions satisfied
// Return alive cell count difference
func (matrix *Matrix) checkAndFlipUnsafe(cell Cell, next_matrix *Matrix, flipping_buffer *[]Cell, unsafe_buffer *[]Cell) {
	if matrix.pixels[cell.Y][cell.X] == 0 {
		if matrix.surrounding_counts[cell.Y][cell.X] == 3 {
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 255
			*flipping_buffer = append(*flipping_buffer, cell)
			*unsafe_buffer = append(*unsafe_buffer, cell)
		} else {
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 0
		}
	} else {
		switch matrix.surrounding_counts[cell.Y][cell.X] {
		case 2:
			fallthrough
		case 3:
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 255
		default:
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 0
			*flipping_buffer = append(*flipping_buffer, cell)
			*unsafe_buffer = append(*unsafe_buffer, cell)
		}
	}
}

// Check if cell is in assigned partition
func (matrix *Matrix) inPartition(cell Cell) bool {
	for _, block := range matrix.partition {
		if block.Start.X <= cell.X && cell.X < block.End.X &&
			block.Start.Y <= cell.Y && cell.Y < block.End.Y {
			return true
		}
	}
	return false
}

// Update surrounding counts of surrounding cells for those in assigned partition
func (matrix *Matrix) updateUnsafe(cell Cell, next_matrix *Matrix) {
	if matrix.pixels[cell.Y][cell.X] == 0 {
		for _, surrounding := range matrix.getSurrounding(cell) {
			if matrix.inPartition(surrounding) {
				next_matrix.surrounding_counts[surrounding.Y][surrounding.X]++
			}
		}
	} else {
		for _, surrounding := range matrix.getSurrounding(cell) {
			if matrix.inPartition(surrounding) {
				next_matrix.surrounding_counts[surrounding.Y][surrounding.X]--
			}
		}
	}
}

// Update surrounding counts of surrounding cells of cells not in partition
func (matrix *Matrix) applyAdjustment(adjustment Adjustment) {
	for _, cell := range adjustment.Increment {
		for _, surrounding := range matrix.getSurrounding(cell) {
			if matrix.inPartition(surrounding) {
				matrix.surrounding_counts[surrounding.Y][surrounding.X]++
			}
		}
	}
	for _, cell := range adjustment.Decrement {
		for _, surrounding := range matrix.getSurrounding(cell) {
			if matrix.inPartition(surrounding) {
				matrix.surrounding_counts[surrounding.Y][surrounding.X]--
			}
		}
	}
}
