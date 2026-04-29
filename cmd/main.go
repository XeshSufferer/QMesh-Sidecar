package main

import (
	"fmt"
	"log"

	"QMesh-Sidecar/internal"
	"QMesh-Sidecar/internal/gossip"
)

func main() {
	seeds := gossip.SeedsFromEnv()
	gossipInstance := gossip.NewGossip(seeds)

	if err := gossipInstance.Start(); err != nil {
		log.Fatalf("Failed to start gossip: %v", err)
	}

	listener := internal.NewListener(gossipInstance)
	gossipInstance.SetMemberListener(listener)

	go listener.StartListen()

	fmt.Println("QMesh Sidecar started!")
	select {}
}
