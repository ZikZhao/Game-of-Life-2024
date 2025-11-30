package gol

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"net"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

// Event type enumerations
const (
	EVENT_TURN_COMPLETE = iota
	EVENT_PAUSE
	EVENT_RESUME
	EVENT_SAVE
	EVENT_QUIT
	EVENT_KILL
	EVENT_FLIPPED
)

// Connection object
type Connection struct {
	conn        *net.TCPConn
	result_chan chan []util.Cell
	event_chan  chan byte
}

// Establish a new connection to broker
func NewConnection(size_int int) *Connection {
	conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.IPv4(54, 209, 41, 143), Port: 2001})
	if err != nil {
		log.Panic(err)
	}
	conn_obj := &Connection{
		conn:        conn,
		result_chan: make(chan []util.Cell),
		event_chan:  make(chan byte),
	}
	log.Print("Connection to 54.209.41.143:2001 established")
	go conn_obj.Monitor(size_int)
	return conn_obj
}

// Repeatedly read data from connection until closed by broker
func (conn *Connection) Monitor(size_int int) {

	defer func() {
		conn.conn.Close()
		log.Printf("Connection to %s closed", conn.conn.RemoteAddr().String())
		close(conn.result_chan)
		close(conn.event_chan)
	}()

	conn.conn.SetReadDeadline(*new(time.Time))
	buffer := bufio.NewReader(conn.conn)

	for {
		message, err := buffer.ReadByte()
		if err != nil {
			if err == io.EOF {
				return
			}
			panic("")
		}
		switch message {
		case EVENT_FLIPPED:
			// Reading flipping data
			var length_bytes [8]byte
			_, err := io.ReadFull(buffer, length_bytes[:])
			if err != nil {
				log.Panic(err)
			}
			data_length, _ := binary.Varint(length_bytes[:])
			if data_length == 0 {
				conn.result_chan <- []util.Cell{}
			} else {
				flipped_data := make([]byte, data_length)
				_, err = io.ReadFull(buffer, flipped_data)
				if err != nil {
					log.Panic(err)
				}
				flipped := decompressFlipped(flipped_data, size_int)
				conn.result_chan <- flipped
			}
		case EVENT_TURN_COMPLETE:
			fallthrough
		case EVENT_PAUSE:
			fallthrough
		case EVENT_RESUME:
			fallthrough
		case EVENT_SAVE:
			fallthrough
		case EVENT_QUIT:
			fallthrough
		case EVENT_KILL:
			conn.event_chan <- message
		default:
			panic("")
		}
	}
}
