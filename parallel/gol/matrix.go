package gol

import (
	"uk.ac.bris.cs/gameoflife/util"
)

type Matrix struct {
	width              int
	height             int
	pixels             [][]uint8
	surrounding_counts [][]int8
}

// Make matrix object with empty data
func MakeMatrix(p Params) Matrix {
	matrix := &Matrix{
		width:              p.ImageWidth,
		height:             p.ImageHeight,
		pixels:             make([][]uint8, p.ImageHeight),
		surrounding_counts: make([][]int8, p.ImageHeight),
	}
	pixel_data := make([]uint8, matrix.width*matrix.height)
	count_data := make([]int8, matrix.width*matrix.height)
	for i := 0; i != matrix.width; i++ {
		matrix.pixels[i] = pixel_data[0:matrix.width]
		matrix.surrounding_counts[i] = count_data[0:matrix.width]
		pixel_data = pixel_data[matrix.width:]
		count_data = count_data[matrix.width:]
	}
	return *matrix
}

// Make matrix object by providing pixel array
// Ownership of pixel array is transferred to matrix object
func MakeMatrixFromData(p Params, pixel_data []uint8) Matrix {
	matrix := &Matrix{
		width:              p.ImageWidth,
		height:             p.ImageHeight,
		pixels:             make([][]uint8, p.ImageHeight),
		surrounding_counts: make([][]int8, p.ImageHeight),
	}
	count_data := make([]int8, matrix.width*matrix.height)
	for i := 0; i != matrix.width; i++ {
		matrix.pixels[i] = pixel_data[0:matrix.width]
		matrix.surrounding_counts[i] = count_data[0:matrix.width]
		pixel_data = pixel_data[matrix.width:]
		count_data = count_data[matrix.width:]
	}
	return *matrix
}

// Get cellition of eight surrounding cells
func (matrix *Matrix) getSurrounding(cell util.Cell) [8]util.Cell {
	if cell.X == 0 || cell.Y == 0 || cell.X == matrix.width-1 || cell.Y == matrix.height-1 {
		return [8]util.Cell{
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
		return [8]util.Cell{
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
func (matrix *Matrix) checkAndFlip(cell util.Cell, next_matrix *Matrix, flipping_buffer *[]util.Cell) int {
	if matrix.pixels[cell.Y][cell.X] == 0 {
		if matrix.surrounding_counts[cell.Y][cell.X] == 3 {
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 255
			for _, surrounding := range matrix.getSurrounding(cell) {
				next_matrix.surrounding_counts[surrounding.Y][surrounding.X]++
			}
			*flipping_buffer = append(*flipping_buffer, cell)
			return 1
		} else {
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 0
			return 0
		}
	} else {
		switch matrix.surrounding_counts[cell.Y][cell.X] {
		case 2:
			fallthrough
		case 3:
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 255
			return 0
		default:
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 0
			for _, surrounding := range matrix.getSurrounding(cell) {
				next_matrix.surrounding_counts[surrounding.Y][surrounding.X]--
			}
			*flipping_buffer = append(*flipping_buffer, cell)
			return -1
		}
	}
}

// Check and flip cells if conditions satisfied
// Return alive cell count difference
func (matrix *Matrix) checkAndFlipUnsafe(cell util.Cell, next_matrix *Matrix, flipping_buffer *[]util.Cell, unsafe_buffer *[]util.Cell) int {
	if matrix.pixels[cell.Y][cell.X] == 0 {
		if matrix.surrounding_counts[cell.Y][cell.X] == 3 {
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 255
			*flipping_buffer = append(*flipping_buffer, cell)
			*unsafe_buffer = append(*unsafe_buffer, cell)
			return 1
		} else {
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 0
			return 0
		}
	} else {
		switch matrix.surrounding_counts[cell.Y][cell.X] {
		case 2:
			fallthrough
		case 3:
			// Copying
			next_matrix.pixels[cell.Y][cell.X] = 255
			return 0
		default:
			// Flipping
			next_matrix.pixels[cell.Y][cell.X] = 0
			*flipping_buffer = append(*flipping_buffer, cell)
			*unsafe_buffer = append(*unsafe_buffer, cell)
			return -1
		}
	}
}
