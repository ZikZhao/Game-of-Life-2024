package main

import (
	"fmt"
	"os"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

func Benchmark_128_16000(b *testing.B) {

	os.Stdout = nil // Disable all program output apart from benchmark results

	for threads := 1; threads <= 16; threads++ {
		p := gol.Params{
			Turns:       16000,
			Threads:     threads,
			ImageWidth:  128,
			ImageHeight: 128,
		}
		name := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events {
				}
			}
		})
	}
}

func Benchmark_256_4000(b *testing.B) {

	os.Stdout = nil // Disable all program output apart from benchmark results

	for threads := 1; threads <= 16; threads++ {
		p := gol.Params{
			Turns:       4000,
			Threads:     threads,
			ImageWidth:  256,
			ImageHeight: 256,
		}
		name := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events {
				}
			}
		})
	}
}

func Benchmark_512_1000(b *testing.B) {

	os.Stdout = nil // Disable all program output apart from benchmark results

	for threads := 1; threads <= 16; threads++ {
		p := gol.Params{
			Turns:       1000,
			Threads:     threads,
			ImageWidth:  512,
			ImageHeight: 512,
		}
		name := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events {
				}
			}
		})
	}
}
