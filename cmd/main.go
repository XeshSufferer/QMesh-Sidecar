package main

import (
	"fmt"
	"log"

	"QMesh-Sidecar/internal"
	"QMesh-Sidecar/internal/gossip"
)

func main() {
	seeds := gossip.SeedsFromEnv()
	if len(seeds) == 0 {
		seeds = gossip.SeedsFromFile("/shared/gossip-seeds")
	}
	gossipInstance := gossip.NewGossip(seeds)

	if err := gossipInstance.Start(); err != nil {
		log.Fatalf("Failed to start gossip: %v", err)
	}

	listener := internal.NewListener(gossipInstance)
	gossipInstance.SetMemberListener(listener)

	go listener.StartListen()

	// Start transparent proxy
	transparentProxy := internal.NewTransparentProxy(listener.GetTrie())
	go func() {
		if err := transparentProxy.Start("3128"); err != nil {
			log.Printf("Failed to start transparent proxy: %v", err)
		}
	}()

	fmt.Println("QMesh Sidecar started!")
	select {}
}
