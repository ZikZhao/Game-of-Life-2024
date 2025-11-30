package main

type Matrix struct {
	width              int
	height             int
	pixels             [][]uint8
	surrounding_counts [][]int8
}

// Make matrix object by providing pixel array
// Ownership of pixel array is transferred to matrix object
func MakeMatrixFromData(pixels [][]uint8, surrounding_counts [][]int8) Matrix {
	return Matrix{
		width:              len(pixels[0]),
		height:             len(pixels),
		pixels:             pixels,
		surrounding_counts: surrounding_counts,
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
func (matrix *Matrix) flip(cell Cell) {
	if matrix.pixels[cell.Y][cell.X] == 0 {
		matrix.pixels[cell.Y][cell.X] = 255
		for _, surrounding := range matrix.getSurrounding(cell) {
			matrix.surrounding_counts[surrounding.Y][surrounding.X]++
		}
	} else {
		matrix.pixels[cell.Y][cell.X] = 0
		for _, surrounding := range matrix.getSurrounding(cell) {
			matrix.surrounding_counts[surrounding.Y][surrounding.X]--
		}
	}
}

// Update matrix from given slice of flipped cells and
// put cells in adjustment buffers if they are exchange target of that worker
func (broker *Broker) updateMatrixAndGetAdjustments(flipped []Cell, adjust []Adjustment) {

	for _, cell := range flipped {
		broker.matrix.flip(cell)

		// Append cell to exchange targets
		exchange_targets := broker.exchange_graph[cell.Y][cell.X]
		if exchange_targets != 0 {
			if broker.matrix.pixels[cell.Y][cell.X] == 0 {

				// Flipped to black, decrementing surrounding counts
				for node_index := 0; node_index != len(adjust); node_index++ {
					if exchange_targets&(1<<node_index) != 0 {
						adjust[node_index].Decrement = append(adjust[node_index].Decrement, cell)
					}
				}
			} else {

				// Flipped to white, incrementing surrounding counts
				for node_index := 0; node_index != len(adjust); node_index++ {
					if exchange_targets&(1<<node_index) != 0 {
						adjust[node_index].Increment = append(adjust[node_index].Increment, cell)
					}
				}
			}
		}
	}
}
