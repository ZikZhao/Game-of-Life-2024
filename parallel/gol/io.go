package gol

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

// ioState is the internal ioState of the io goroutine.
type ioState struct {
	params    Params
	operation *ioOperation
	cond      *sync.Cond
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//
//	ioOutput 	= 0
//	ioInput 	= 1
//	ioCheckIdle = 2
const (
	ioOutput ioCommand = iota
	ioInput
	// ioCheckIdle This constant is no longer needed because distributor is awakened after sync
	ioQuit
)

type ioOperation struct {
	command   ioCommand
	filename  string
	data      []byte
	completed bool
}

// writePgmImage receives an array of bytes and writes it to a pgm file.
func (io *ioState) writePgmImage() {
	_ = os.Mkdir("out", os.ModePerm)

	file, ioError := os.Create("out/" + io.operation.filename + ".pgm")
	util.Check(ioError)
	defer file.Close()

	_, _ = file.WriteString("P5\n")
	//_, _ = file.WriteString("# PGM file writer by pnmmodules (https://github.com/owainkenwayucl/pnmmodules).\n")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageWidth))
	_, _ = file.WriteString(" ")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageHeight))
	_, _ = file.WriteString("\n")
	_, _ = file.WriteString(strconv.Itoa(255))
	_, _ = file.WriteString("\n")

	_, ioError = file.Write(io.operation.data)
	util.Check(ioError)

	ioError = file.Sync()
	util.Check(ioError)

	fmt.Println("File", io.operation.filename, "output done!")
}

// readPgmImage opens a pgm file and sends its data as an array of bytes.
func (io *ioState) readPgmImage() {

	data, ioError := os.ReadFile("images/" + io.operation.filename + ".pgm")
	util.Check(ioError)

	fields := strings.Fields(string(data))

	if fields[0] != "P5" {
		panic("Not a pgm file")
	}

	width, _ := strconv.Atoi(fields[1])
	if width != io.params.ImageWidth {
		panic("Incorrect width")
	}

	height, _ := strconv.Atoi(fields[2])
	if height != io.params.ImageHeight {
		panic("Incorrect height")
	}

	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic("Incorrect maxval/bit depth")
	}

	io.operation.data = []byte(fields[4])

	fmt.Println("File", io.operation.filename, "input done!")
}

// startIo should be the entrypoint of the io goroutine.
func startIo(io *ioState) {

	for {
		io.cond.Wait()
		switch io.operation.command {
		case ioInput:
			io.readPgmImage()
		case ioOutput:
			io.writePgmImage()
		case ioQuit:
			io.cond.L.Unlock()
			return
		}
		io.operation.completed = true
		io.cond.Signal()
	}
}

// Initiate an IO request
func (io *ioState) sendIoRequest(operation *ioOperation) {
	io.cond.L.Lock()
	io.operation = operation
	io.cond.Signal()
	io.cond.L.Unlock()
}

// Wait until last IO operation completed
func (io *ioState) waitIoRequest() {

	io.cond.L.Lock()
	for !io.operation.completed {
		io.cond.Wait()
	}
	io.cond.L.Unlock()
}

// Send a signal to IO thread to quit
func (io *ioState) quit() {
	io.cond.L.Lock()
	io.operation = &ioOperation{command: ioQuit}
	io.cond.Signal()
	io.cond.L.Unlock()
}
