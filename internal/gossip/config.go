package gossip

import (
	"os"
	"strings"
	"time"
)

const (
	GossipPort   = ":4221"
	MinInterval  = 1 * time.Second
	MaxInterval  = 5 * time.Second
	MaxQueueSize = 256
)

func SeedsFromEnv() []string {
	seeds := os.Getenv("GOSSIP_SEEDS")
	if seeds == "" {
		return nil
	}
	return strings.Split(seeds, ",")
}
