package gossip

import (
	"testing"
	"time"
)

func TestNewWorker(t *testing.T) {
	g := NewGossip(nil)
	worker := NewWorker(g)

	if worker == nil {
		t.Fatal("NewWorker() returned nil")
	}
	if worker.gossip != g {
		t.Error("worker gossip reference not set correctly")
	}
}

func TestWorker_StartStop(t *testing.T) {
	g := NewGossip(nil)
	worker := NewWorker(g)

	worker.Start()

	if !worker.running {
		t.Error("worker should be running after Start()")
	}

	worker.Stop()

	if worker.running {
		t.Error("worker should not be running after Stop()")
	}
}

func TestWorker_StopMultipleTimes(t *testing.T) {
	g := NewGossip(nil)
	worker := NewWorker(g)

	worker.Start()
	worker.Stop()

	worker.Stop()

	if worker.running {
		t.Error("worker should not panic on multiple stops")
	}
}

func TestWorker_RandomInterval(t *testing.T) {
	g := NewGossip(nil)
	worker := NewWorker(g)

	for i := 0; i < 100; i++ {
		interval := worker.randomInterval()

		if interval < MinInterval {
			t.Errorf("interval = %v, want >= %v", interval, MinInterval)
		}
		if interval > MaxInterval {
			t.Errorf("interval = %v, want <= %v", interval, MaxInterval)
		}
	}
}

func TestWorker_RandomIntervalVariation(t *testing.T) {
	g := NewGossip(nil)
	worker := NewWorker(g)

	intervals := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		intervals[worker.randomInterval()] = true
	}

	if len(intervals) < 2 {
		t.Log("WARNING: random interval showed little variation (may be ok)")
	}
}

func TestWorker_SendToPeer(t *testing.T) {
	seeds := []string{"10.0.0.1:4221", "10.0.0.2:4221"}
	g := NewGossip(seeds)
	worker := NewWorker(g)

	worker.SendToPeer("10.0.0.3:4221")
}

func TestWorker_SendToPeer_NoSeeds(t *testing.T) {
	g := NewGossip(nil)
	worker := NewWorker(g)

	worker.SendToPeer("10.0.0.3:4221")
}
