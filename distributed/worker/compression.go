package main

// Compress and write slice of flipped cells to a byte slice
func compressFlippedTo(flipped []Cell, dest []byte, size_int int) []byte {
	for _, cell := range flipped {
		for j := 0; j != size_int; j++ {
			dest[0] = byte(cell.X >> (j << 3))
			dest = dest[1:]
		}
		for j := 0; j != size_int; j++ {
			dest[0] = byte(cell.Y >> (j << 3))
			dest = dest[1:]
		}
	}
	return dest
}
