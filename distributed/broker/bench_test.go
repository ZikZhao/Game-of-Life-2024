package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

var compressed []byte

// Eat all incoming bytes
type null_writer struct{}

func (w null_writer) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// Initialise a new broker and return connection to it
func initBroker() (*Broker, *net.TCPConn) {

	broker := &Broker{
		cond: sync.NewCond(new(sync.Mutex)),
	}
	mutex := new(sync.Mutex)
	listener, _ := net.ListenTCP("tcp", &net.TCPAddr{Port: 2001})
	mutex.Lock()
	go func() {
		conn, _ := listener.Accept()
		broker.local_conn = &Connection{
			conn:  conn.(*net.TCPConn),
			mutex: new(sync.Mutex),
		}
		listener.Close()
		mutex.Unlock()
	}()
	conn, _ := net.DialTCP("tcp", nil, &net.TCPAddr{Port: 2001})
	mutex.Lock()
	mutex.Unlock()
	return broker, conn
}

// Reading message from connection and count turns
// returns when last turn completed
func readAllTurns(conn *net.TCPConn, turn int) {
	buffer := bufio.NewReader(conn)
	for i := 0; i != turn; {
		message_type, _ := buffer.ReadByte()
		switch message_type {
		case EVENT_FLIPPED:
			var length_bytes [8]byte
			io.ReadFull(buffer, length_bytes[:])
			length, _ := binary.Varint(length_bytes[:])
			io.ReadFull(buffer, make([]byte, length))
		case EVENT_TURN_COMPLETE:
			i++
		}
	}
}

func TestMain(m *testing.M) {

	log.SetOutput(null_writer{}) // Disable log

	data, _ := os.ReadFile("bench_images/512x512.pgm")
	fields := strings.Fields(string(data))
	if fields[0] != "P5" {
		panic("Not a pgm file")
	}
	pixel_data := []byte(fields[4])

	compressed = make([]byte, len(pixel_data)/8+1)
	for i := range pixel_data {
		compressed[i/8] |= pixel_data[i] & (1 << (i % 8))
	}

	mutex := new(sync.Mutex)
	go func() {
		listener, _ := net.ListenTCP("tcp", &net.TCPAddr{Port: 2002})
		for {
			conn, _ := listener.Accept()
			mutex.Lock()
			ip := strings.Split(conn.RemoteAddr().String(), ":")[0]
			client, _ := rpc.DialHTTP("tcp", ip+":2000")
			node := Node{
				ip:     conn.RemoteAddr().String(),
				conn:   conn.(*net.TCPConn),
				client: client,
			}
			nodes[node] = struct{}{}
			mutex.Unlock()
		}
	}()
	time.Sleep(time.Second * 1)
	mutex.Lock()
	m.Run()
	mutex.Unlock()
}

func Benchmark_512_1000(b *testing.B) {

	os.Stdout = nil // Disable all program output apart from benchmark results

	for worker := 1; worker <= 8; worker++ {

		for threads := 1; threads <= 16; threads++ {

			name := fmt.Sprintf("512x512x1000-%d(%dw)", threads, worker)

			copied := make([]byte, len(compressed))
			copy(copied, compressed)

			bp := BrokerParams{
				Turns:       1000,
				Threads:     threads,
				ImageWidth:  512,
				ImageHeight: 512,
				Pixels:      copied,
				SizeInt:     2,
			}

			b.Run(name, func(b *testing.B) {

				if len(nodes) != worker {
					b.Skip()
				}

				for i := 0; i < b.N; i++ {
					broker, read_conn := initBroker()
					broker.Init(bp, &struct{}{})
					b.ResetTimer()
					readAllTurns(read_conn, 1000)
				}
			})
		}
	}

}
