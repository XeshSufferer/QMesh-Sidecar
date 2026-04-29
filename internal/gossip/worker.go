package gossip

import (
	"log"
	"math/rand/v2"
	"time"
)

type Worker struct {
	gossip  *Gossip
	stopCh  chan struct{}
	running bool
}

func NewWorker(g *Gossip) *Worker {
	return &Worker{
		gossip: g,
		stopCh: make(chan struct{}),
	}
}

func (w *Worker) Start() {
	w.running = true
	go w.run()
}

func (w *Worker) Stop() {
	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
}

func (w *Worker) run() {
	ticker := time.NewTicker(w.randomInterval())
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.gossip.Broadcast()
			ticker.Reset(w.randomInterval())
		}
	}
}

func (w *Worker) randomInterval() time.Duration {
	delta := rand.Int64N(int64(MaxInterval - MinInterval))
	return MinInterval + time.Duration(delta)
}

func (w *Worker) SendToPeer(peer string) {
	seeds := w.gossip.GetSeeds()
	for _, seed := range seeds {
		log.Printf("Sending gossip to seed: %s", seed)
	}
}
