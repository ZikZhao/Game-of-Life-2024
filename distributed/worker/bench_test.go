// go test -run a$ -bench Benchmark_512_1000 -benchtime=10x -cpuprofile cpu.prof
// go test -run a$ -bench Benchmark_5120_50 -benchtime=10x -cpuprofile cpu.prof

package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strings"
	"sync"
	"testing"
)

type null_writer struct{}

func (w null_writer) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func partitioning(width, height int, thread_count int) Partition {
	if thread_count == 1 {
		blocks := make([]Block, 1)
		blocks[0] = Block{
			Start: Cell{X: 0, Y: 0},
			End:   Cell{X: width, Y: height},
		}
		return blocks
	}
	// Floor to nearest composite number
	nthread := 2
	if thread_count < 4 {
		nthread = thread_count
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
			if number == thread_count {
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
	part_width := float64(width) / float64(horizontal)
	part_height := float64(height) / float64(vertical)
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

func Benchmark_512_1000(b *testing.B) {

	os.Stdout = nil              // Disable all program output apart from benchmark results
	log.SetOutput(null_writer{}) // Disable log

	data, _ := os.ReadFile("bench_images/512x512.pgm")
	fields := strings.Fields(string(data))
	if fields[0] != "P5" {
		panic("Not a pgm file")
	}
	pixel_data := []byte(fields[4])

	matrix := make([][]uint8, 512)
	for y := range matrix {
		matrix[y] = pixel_data[y*512 : (y+1)*512]
	}

	surrounding_counts := make([][]int8, 512)
	for y := range matrix {
		surrounding_counts[y] = make([]int8, 512)
	}

	for y := range matrix {
		for x, value := range matrix[y] {
			if value != 0 {
				for _, surrounding := range [8]Cell{
					{X: (x - 1 + 512) % 512, Y: (y - 1 + 512) % 512},
					{X: x, Y: (y - 1 + 512) % 512},
					{X: (x + 1) % 512, Y: (y - 1 + 512) % 512},
					{X: (x - 1 + 512) % 512, Y: y},
					{X: (x + 1) % 512, Y: y},
					{X: (x - 1 + 512) % 512, Y: (y + 1) % 512},
					{X: x, Y: (y + 1) % 512},
					{X: (x + 1) % 512, Y: (y + 1) % 512},
				} {
					surrounding_counts[surrounding.Y][surrounding.X]++
				}
			}
		}
	}

	instance := &Worker{
		running:     new(bool),
		cond:        sync.NewCond(new(sync.Mutex)),
		result_chan: make(chan TurnResult),
	}

	// Register functions
	rpc.Register(instance)
	rpc.HandleHTTP()

	// Start RPC handling service
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 2000})
	if err != nil {
		log.Panic(err.Error())
	}
	go http.Serve(listener, nil)

	client, _ := rpc.DialHTTP("tcp", ":2000")

	adjustments := Adjustment{make([]Cell, 0), make([]Cell, 0)}

	for threads := 1; threads <= 16; threads++ {

		partition := partitioning(512, 512, threads)

		wp := WorkerParams{
			Turns:             1000,
			Threads:           threads,
			ImageWidth:        512,
			ImageHeight:       512,
			Pixels:            matrix,
			SurroundingCounts: surrounding_counts,
			Partition:         partition,
			SizeInt:           2,
		}
		name := fmt.Sprintf("512x512x1000-%d", threads)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				client.Call("Worker.Init", wp, &struct{}{})
				for turn := 0; turn != 1000; turn++ {
					var result []byte
					client.Call("Worker.Next", adjustments, &result)
				}
			}
		})
	}
}
