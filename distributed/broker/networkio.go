package main

import (
	"encoding/binary"
	"log"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"
)

const (
	EVENT_TURN_COMPLETE = iota
	EVENT_PAUSE
	EVENT_RESUME
	EVENT_SAVE
	EVENT_QUIT
	EVENT_KILL
	EVENT_FLIPPED
)

// Structure representing a connection to local controller
type Connection struct {
	conn  *net.TCPConn
	mutex *sync.Mutex // synchronise writing functions
}

var nodes = make(map[Node]struct{})
var mutex = new(sync.Mutex) // synchronise access to available nodes

// Write compressed slice of flipped cells to connection to local controller
func (conn *Connection) writeCompressedFlipped(flipped_data []byte) {

	if len(flipped_data) == 0 {
		return
	}

	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	// Write type of message
	_, err := conn.conn.Write([]byte{EVENT_FLIPPED})
	if err != nil {
		log.Panic(err.Error())
	}

	// Write length of cell slice
	var length_bytes [8]byte
	binary.PutVarint(length_bytes[:], int64(len(flipped_data)))
	_, err = conn.conn.Write(length_bytes[:])
	if err != nil {
		log.Panic(err.Error())
	}

	// Write cell slice
	_, err = conn.conn.Write(flipped_data)
	if err != nil {
		log.Panic(err.Error())
	}
}

func (conn *Connection) writeEvent(event byte) {

	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	// Write type of message
	_, err := conn.conn.Write([]byte{event})
	if err != nil {
		log.Panic(err.Error())
	}
}

// Accept connection request from worker node
func monitorNodes() {

	// Listen on connection requests from worker node
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 2002})
	if err != nil {
		log.Panic(err.Error())
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Panic(err.Error())
		}
		ip := strings.Split(conn.RemoteAddr().String(), ":")[0]
		client, err := rpc.DialHTTP("tcp", ip+":2000")
		if err != nil {
			log.Panic(err.Error())
		}
		go func() {
			// Append to available worker node list
			mutex.Lock()
			node := Node{
				ip:     conn.RemoteAddr().String(),
				conn:   conn.(*net.TCPConn),
				client: client,
			}
			nodes[node] = struct{}{}
			mutex.Unlock()
			// Blocking read on connection until connnection is reset
			conn.SetReadDeadline(*new(time.Time))
			data := make([]byte, 10000)
			conn.Read(data)
			mutex.Lock()
			delete(nodes, node)
			mutex.Unlock()
			log.Printf("Worker node %s disconnected", ip)
		}()
		log.Printf("Worker node %s registered", ip)
	}
}

// Function retrieving available worker nodes
func getAvailableNodes() map[Node]struct{} {

	mutex.Lock()
	copied := make(map[Node]struct{})
	for node := range nodes {
		copied[node] = struct{}{}
	}
	mutex.Unlock()
	return copied
}

// Function send shutdown commands to all available nodes
func shutdownNodes() {

	mutex.Lock()
	for node := range nodes {
		err := node.client.Call("Worker.Kill", struct{}{}, &struct{}{})
		if err != nil {
			log.Panic(err)
		}
	}
	mutex.Unlock()
}
