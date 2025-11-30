package main

// Get positions of surrounding cells of a specific cell (identical to that of Matrix)
func getSurrounding(width, height int, cell Cell) [8]Cell {
	if cell.X == 0 || cell.Y == 0 || cell.X == width-1 || cell.Y == height-1 {
		return [8]Cell{
			{X: (cell.X - 1 + width) % width, Y: (cell.Y - 1 + height) % height},
			{X: cell.X, Y: (cell.Y - 1 + height) % height},
			{X: (cell.X + 1) % width, Y: (cell.Y - 1 + height) % height},
			{X: (cell.X - 1 + width) % width, Y: cell.Y},
			{X: (cell.X + 1) % width, Y: cell.Y},
			{X: (cell.X - 1 + width) % width, Y: (cell.Y + 1) % height},
			{X: cell.X, Y: (cell.Y + 1) % height},
			{X: (cell.X + 1) % width, Y: (cell.Y + 1) % height},
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

// Decompress matrix data and get surrounding counts at the same time
func decompressMatrix(bp *BrokerParams) (matrix [][]uint8, surrounding_counts [][]int8) {
	pixel_data := make([]uint8, bp.ImageWidth*bp.ImageHeight)
	// Create 2D slice on pixel data
	matrix = make([][]uint8, bp.ImageHeight)
	for y := 0; y != bp.ImageHeight; y++ {
		matrix[y] = pixel_data[y*bp.ImageWidth : (y+1)*bp.ImageWidth]
	}
	// Create surrounding count matrix
	surrounding_counts = make([][]int8, bp.ImageHeight)
	for y := 0; y != bp.ImageHeight; y++ {
		surrounding_counts[y] = make([]int8, bp.ImageWidth)
	}
	// Load matrix data
	if bp.Pixels != nil {
		// Compressed pixel data
		for i := range pixel_data {
			if bp.Pixels[i/8]&(1<<(i%8)) == 0 {
				pixel_data[i] = 0
			} else {
				pixel_data[i] = 255
				this_cell := Cell{X: i % bp.ImageWidth, Y: i / bp.ImageWidth}
				for _, cell := range getSurrounding(bp.ImageWidth, bp.ImageHeight, this_cell) {
					surrounding_counts[cell.Y][cell.X]++
				}
			}
		}
	} else {
		// Compressed positions of initial alive cells
		for i := 0; i != len(bp.Initials); i += bp.SizeInt * 2 {
			pos := Cell{}
			for j := 0; j != bp.SizeInt; j++ {
				pos.X |= int(bp.Initials[i+j]) << (j * 8)
			}
			for j := 0; j != bp.SizeInt; j++ {
				pos.Y |= int(bp.Initials[i+bp.SizeInt+j]) << (j * 8)
			}
			matrix[pos.Y][pos.X] = 255
			for _, cell := range getSurrounding(bp.ImageWidth, bp.ImageHeight, Cell{X: pos.X, Y: pos.Y}) {
				surrounding_counts[cell.Y][cell.X]++
			}
		}
	}
	return matrix, surrounding_counts
}

// Decompress cells flipped events
func decompressFlipped(data []byte, size_int int) []Cell {
	flipped := make([]Cell, len(data)/(size_int*2))
	flipped_index := 0
	for i := 0; i != len(data); i += size_int * 2 {
		for j := 0; j != size_int; j++ {
			flipped[flipped_index].X |= int(data[i+j]) << (j << 3)
		}
		for j := 0; j != size_int; j++ {
			flipped[flipped_index].Y |= int(data[i+size_int+j]) << (j << 3)
		}
		flipped_index++
	}
	return flipped
}
