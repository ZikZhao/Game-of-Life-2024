package gol

import "uk.ac.bris.cs/gameoflife/util"

// Get the minimum number of bytes to represent the whole range of width and height
func getSizeOfInt(width, height int) int {
	size_int := 1
	for largest := 0x7F; largest < width || largest < height; largest = (largest << 8) | 0xFF {
		size_int++
	}
	return size_int
}

// Compress initial cell matrix
func compressMatrix(bp *BrokerParams, pixels []uint8, initials []util.Cell) {
	bp.SizeInt = getSizeOfInt(bp.ImageWidth, bp.ImageHeight)
	size_pixels := len(pixels)/8 + 1
	size_initials := len(initials) * bp.SizeInt * 2
	if size_pixels <= size_initials {
		// Compressed pixel data will be sent
		bp.Pixels = make([]byte, size_pixels)
		for i := range pixels {
			bp.Pixels[i/8] |= pixels[i] & (1 << (i % 8))
		}
	} else {
		// Compressed position of initial alive cells will be sent
		bp.Initials = make([]byte, size_initials)
		view := bp.Initials[:]
		for _, cell := range initials {
			for j := 0; j != bp.SizeInt; j++ {
				view[0] = byte(cell.X >> (j * 8))
				view = view[1:]
			}
			for j := 0; j != bp.SizeInt; j++ {
				view[0] = byte(cell.Y >> (j * 8))
				view = view[1:]
			}
		}
	}
}

// Decompress cells flipping data
func decompressFlipped(data []byte, size_int int) []util.Cell {
	flipped := make([]util.Cell, len(data)/(size_int*2))
	flipped_index := 0
	for i := 0; i != len(data); i += size_int * 2 {
		for j := 0; j != size_int; j++ {
			flipped[flipped_index].X |= int(data[i+j]) << (j * 8)
		}
		for j := 0; j != size_int; j++ {
			flipped[flipped_index].Y |= int(data[i+size_int+j]) << (j * 8)
		}
		flipped_index++
	}
	return flipped
}
