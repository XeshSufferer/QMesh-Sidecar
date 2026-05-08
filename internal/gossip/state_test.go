package gossip

import (
	"sync"
	"testing"
	"time"

	"QMesh-Sidecar/internal/protos/pb/gen/gossip"
)

func TestNewGossipState(t *testing.T) {
	state := NewGossipState()
	if state == nil {
		t.Fatal("NewGossipState() returned nil")
	}
	if state.nodes == nil {
		t.Fatal("state.nodes is nil")
	}
	if state.version != 1 {
		t.Errorf("initial version = %d, want 1", state.version)
	}
}

func TestGossipState_Update_NewNode(t *testing.T) {
	state := NewGossipState()

	state.Update("10.0.0.1", gossip.Status_NEW)

	node, exists := state.Get("10.0.0.1")
	if !exists {
		t.Fatal("expected node to exist")
	}
	if node.Status != gossip.Status_NEW {
		t.Errorf("status = %v, want %v", node.Status, gossip.Status_NEW)
	}
	if node.Version != 1 {
		t.Errorf("version = %d, want 1", node.Version)
	}
}

func TestGossipState_Update_ExistingNode(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	state.Update("10.0.0.1", gossip.Status_DEAD)

	node, exists := state.Get("10.0.0.1")
	if !exists {
		t.Fatal("expected node to exist")
	}
	if node.Status != gossip.Status_DEAD {
		t.Errorf("status = %v, want %v", node.Status, gossip.Status_DEAD)
	}
}

func TestGossipState_Update_SameStatus(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	oldVersion := state.GetVersion()
	state.Update("10.0.0.1", gossip.Status_NEW)

	if state.GetVersion() != oldVersion {
		t.Error("version should not change when status is same")
	}
}

func TestGossipState_Get_NotFound(t *testing.T) {
	state := NewGossipState()

	_, exists := state.Get("nonexistent")
	if exists {
		t.Error("expected node to not exist")
	}
}

func TestGossipState_GetAll(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)
	state.Update("10.0.0.2", gossip.Status_NEW)

	all := state.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(all))
	}

	if _, ok := all["10.0.0.1"]; !ok {
		t.Error("expected 10.0.0.1 in nodes")
	}
	if _, ok := all["10.0.0.2"]; !ok {
		t.Error("expected 10.0.0.2 in nodes")
	}
}

func TestGossipState_GetAll_Empty(t *testing.T) {
	state := NewGossipState()

	all := state.GetAll()
	if len(all) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(all))
	}
}

func TestGossipState_GetVersion(t *testing.T) {
	state := NewGossipState()

	initialVersion := state.GetVersion()
	if initialVersion != 1 {
		t.Errorf("initial version = %d, want 1", initialVersion)
	}

	state.Update("10.0.0.1", gossip.Status_NEW)
	if state.GetVersion() <= initialVersion {
		t.Error("version should increment after update")
	}
}

func TestGossipState_GetHash(t *testing.T) {
	state := NewGossipState()

	hash1 := state.GetHash()
	if hash1 == 0 {
		t.Log("hash of empty state is 0 (expected)")
	}

	state.Update("10.0.0.1", gossip.Status_NEW)
	hash2 := state.GetHash()

	if hash2 == hash1 {
		t.Error("hash should change after adding node")
	}
}

func TestGossipState_GetHash_Consistency(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)
	state.Update("10.0.0.2", gossip.Status_DEAD)

	hash1 := state.GetHash()
	hash2 := state.GetHash()

	if hash1 != hash2 {
		t.Error("hash should be consistent for same state")
	}
}

func TestGossipState_GetHash_DifferentStates(t *testing.T) {
	state1 := NewGossipState()
	state1.Update("10.0.0.1", gossip.Status_NEW)

	state2 := NewGossipState()
	state2.Update("10.0.0.2", gossip.Status_NEW)

	if state1.GetHash() == state2.GetHash() {
		t.Error("different states should have different hashes (likely)")
	}
}

func TestGossipState_GetHashBytes(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	hashBytes := state.GetHashBytes()
	defer ReleaseHashBytes(hashBytes)

	if len(hashBytes) != 8 {
		t.Errorf("hash bytes length = %d, want 8", len(hashBytes))
	}
}

func TestGossipState_Merge_NewNodes(t *testing.T) {
	state := NewGossipState()

	other := map[string]*NodeState{
		"10.0.0.1": {Status: gossip.Status_NEW, Version: 1},
		"10.0.0.2": {Status: gossip.Status_NEW, Version: 1},
	}

	state.Merge(other)

	if _, exists := state.Get("10.0.0.1"); !exists {
		t.Error("expected 10.0.0.1 after merge")
	}
	if _, exists := state.Get("10.0.0.2"); !exists {
		t.Error("expected 10.0.0.2 after merge")
	}
}

func TestGossipState_Merge_VersionConflict(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	other := map[string]*NodeState{
		"10.0.0.1": {Status: gossip.Status_DEAD, Version: 5},
	}

	state.Merge(other)

	node, _ := state.Get("10.0.0.1")
	if node.Status != gossip.Status_DEAD {
		t.Errorf("status = %v, want %v (higher version should win)", node.Status, gossip.Status_DEAD)
	}
}

func TestGossipState_Merge_LowerVersionIgnored(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)
	state.Update("10.0.0.1", gossip.Status_DEAD)

	other := map[string]*NodeState{
		"10.0.0.1": {Status: gossip.Status_NEW, Version: 1},
	}

	state.Merge(other)

	node, _ := state.Get("10.0.0.1")
	if node.Status != gossip.Status_DEAD {
		t.Errorf("status = %v, want %v (lower version should be ignored)", node.Status, gossip.Status_DEAD)
	}
}

func TestGossipState_Merge_Empty(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	oldVersion := state.GetVersion()
	state.Merge(map[string]*NodeState{})

	if state.GetVersion() != oldVersion {
		t.Error("version should not change after merging empty map")
	}
}

func TestGossipState_GetDiff(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)
	state.Update("10.0.0.2", gossip.Status_DEAD)

	diff := state.GetDiff()
	defer ReleaseDiff(diff)

	if len(diff) != 2 {
		t.Errorf("expected 2 diff entries, got %d", len(diff))
	}

	found := make(map[string]bool)
	for _, d := range diff {
		found[d.NodeIp] = true
	}

	if !found["10.0.0.1"] || !found["10.0.0.2"] {
		t.Error("expected both IPs in diff")
	}
}

func TestGossipState_GetDiff_Empty(t *testing.T) {
	state := NewGossipState()

	diff := state.GetDiff()
	defer ReleaseDiff(diff)

	if len(diff) != 0 {
		t.Errorf("expected 0 diff entries, got %d", len(diff))
	}
}

func TestReleaseHashBytes(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	hashBytes := state.GetHashBytes()
	ReleaseHashBytes(hashBytes)

	hashBytes2 := state.GetHashBytes()
	defer ReleaseHashBytes(hashBytes2)

	if len(hashBytes2) != 8 {
		t.Errorf("hash bytes length = %d, want 8", len(hashBytes2))
	}
}

func TestReleaseDiff(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	diff := state.GetDiff()
	ReleaseDiff(diff)

	for _, d := range diff {
		if d.NodeIp != "" || d.Status != 0 {
			t.Error("diff entries should be cleared after release")
		}
	}
}

func TestGossipState_ConcurrentUpdates(t *testing.T) {
	state := NewGossipState()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip := "10.0.0." + string(rune('0'+id%10))
			state.Update(ip, gossip.Status_NEW)
		}(i)
	}

	wg.Wait()

	all := state.GetAll()
	if len(all) == 0 {
		t.Error("expected some nodes after concurrent updates")
	}
}

func TestGossipState_ConcurrentMerge(t *testing.T) {
	state := NewGossipState()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			other := map[string]*NodeState{
				"10.0.1." + string(rune('0'+id%10)): {Status: gossip.Status_NEW, Version: 1},
			}
			state.Merge(other)
		}(i)
	}

	wg.Wait()

	all := state.GetAll()
	if len(all) == 0 {
		t.Error("expected some nodes after concurrent merges")
	}
}

func TestNodeState_UpdateTimestamp(t *testing.T) {
	state := NewGossipState()
	state.Update("10.0.0.1", gossip.Status_NEW)

	node1, _ := state.Get("10.0.0.1")

	time.Sleep(1 * time.Millisecond)
	state.Update("10.0.0.1", gossip.Status_DEAD)

	node2, _ := state.Get("10.0.0.1")

	if !node2.LastSeen.After(node1.LastSeen) {
		t.Error("LastSeen should be updated on status change")
	}
}
