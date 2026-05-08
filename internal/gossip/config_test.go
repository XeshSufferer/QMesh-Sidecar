package gossip

import (
	"os"
	"testing"
	"time"
)

func TestGossipPort(t *testing.T) {
	if GossipPort != ":4221" {
		t.Errorf("GossipPort = %q, want %q", GossipPort, ":4221")
	}
}

func TestMinInterval(t *testing.T) {
	if MinInterval != 1*time.Second {
		t.Errorf("MinInterval = %v, want %v", MinInterval, 1*time.Second)
	}
}

func TestMaxInterval(t *testing.T) {
	if MaxInterval != 5*time.Second {
		t.Errorf("MaxInterval = %v, want %v", MaxInterval, 5*time.Second)
	}
}

func TestMaxQueueSize(t *testing.T) {
	if MaxQueueSize != 256 {
		t.Errorf("MaxQueueSize = %d, want 256", MaxQueueSize)
	}
}

func TestSeedsFromEnv_Empty(t *testing.T) {
	os.Unsetenv("GOSSIP_SEEDS")

	seeds := SeedsFromEnv()
	if seeds != nil {
		t.Errorf("expected nil seeds, got %v", seeds)
	}
}

func TestSeedsFromEnv_SingleSeed(t *testing.T) {
	os.Setenv("GOSSIP_SEEDS", "10.0.0.1:4221")
	defer os.Unsetenv("GOSSIP_SEEDS")

	seeds := SeedsFromEnv()
	if len(seeds) != 1 {
		t.Fatalf("expected 1 seed, got %d", len(seeds))
	}
	if seeds[0] != "10.0.0.1:4221" {
		t.Errorf("seed = %q, want %q", seeds[0], "10.0.0.1:4221")
	}
}

func TestSeedsFromEnv_MultipleSeeds(t *testing.T) {
	os.Setenv("GOSSIP_SEEDS", "10.0.0.1:4221,10.0.0.2:4221,10.0.0.3:4221")
	defer os.Unsetenv("GOSSIP_SEEDS")

	seeds := SeedsFromEnv()
	if len(seeds) != 3 {
		t.Fatalf("expected 3 seeds, got %d", len(seeds))
	}

	expected := []string{"10.0.0.1:4221", "10.0.0.2:4221", "10.0.0.3:4221"}
	for i, seed := range seeds {
		if seed != expected[i] {
			t.Errorf("seed[%d] = %q, want %q", i, seed, expected[i])
		}
	}
}

func TestSeedsFromEnv_TrailingComma(t *testing.T) {
	os.Setenv("GOSSIP_SEEDS", "10.0.0.1:4221,")
	defer os.Unsetenv("GOSSIP_SEEDS")

	seeds := SeedsFromEnv()
	if len(seeds) != 2 {
		t.Fatalf("expected 2 seeds, got %d", len(seeds))
	}
	if seeds[0] != "10.0.0.1:4221" {
		t.Errorf("seed[0] = %q, want %q", seeds[0], "10.0.0.1:4221")
	}
	if seeds[1] != "" {
		t.Errorf("seed[1] = %q, want empty", seeds[1])
	}
}
