package gossip

import (
	"bytes"
	"log"
	"net"
	"sync"
	"time"
)

type GossipListener struct {
	pool   *sync.Pool
	gossip *Gossip
}

func NewGossipListener(g *Gossip) *GossipListener {
	return &GossipListener{
		pool: &sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, 1024))
			},
		},
		gossip: g,
	}
}

func (l *GossipListener) Listen() {
	addr, err := net.ResolveUDPAddr("udp", GossipPort)
	if err != nil {
		log.Fatal(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	buffer := make([]byte, 65535)
	for {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("Gossip UDP error: %v", err)
			continue
		}

		l.handleMessage(buffer[:n], addr, conn)
	}
}

func (l *GossipListener) handleMessage(data []byte, addr *net.UDPAddr, conn *net.UDPConn) {
	// Message handling is done in Gossip.listen()
	// This method is kept for potential future use
	_ = l
	_ = conn
	_ = addr
	_ = data
}
