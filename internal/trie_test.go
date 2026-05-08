package internal

import (
	"sync"
	"testing"
)

func TestNewTrie(t *testing.T) {
	trie := NewTrie()
	if trie == nil {
		t.Fatal("NewTrie() returned nil")
	}
	if trie.root == nil {
		t.Fatal("Trie root is nil")
	}
}

func TestTrie_GetNode_RootPath(t *testing.T) {
	trie := NewTrie()

	node, exists := trie.GetNode("/")
	if !exists {
		t.Error("expected root node to exist")
	}
	if node != trie.root {
		t.Error("expected to get root node")
	}
}

func TestTrie_GetNode_EmptyPath(t *testing.T) {
	trie := NewTrie()

	node, exists := trie.GetNode("")
	if !exists {
		t.Error("expected empty path to return root")
	}
	if node != trie.root {
		t.Error("expected to get root node for empty path")
	}
}

func TestTrie_GetNode_SingleSegment(t *testing.T) {
	trie := NewTrie()
	trie.EnsureNode("/api")

	node, exists := trie.GetNode("/api")
	if !exists {
		t.Error("expected node to exist")
	}
	if node == nil {
		t.Error("expected non-nil node")
	}
}

func TestTrie_GetNode_MultiSegment(t *testing.T) {
	trie := NewTrie()
	trie.EnsureNode("/api/v1/users")

	node, exists := trie.GetNode("/api/v1/users")
	if !exists {
		t.Error("expected node to exist")
	}
	if node == nil {
		t.Error("expected non-nil node")
	}
}

func TestTrie_GetNode_NotFound(t *testing.T) {
	trie := NewTrie()
	trie.EnsureNode("/api/v1")

	node, exists := trie.GetNode("/api/v2")
	if exists {
		t.Error("expected node to not exist")
	}
	if node != nil {
		t.Error("expected nil node for non-existent path")
	}
}

func TestTrie_GetNode_TrailingSlash(t *testing.T) {
	trie := NewTrie()
	trie.EnsureNode("/api/v1")

	node1, _ := trie.GetNode("/api/v1")
	node2, _ := trie.GetNode("/api/v1/")

	if node1 != node2 {
		t.Error("expected same node for path with and without trailing slash")
	}
}

func TestTrie_EnsureNode_SingleSegment(t *testing.T) {
	trie := NewTrie()

	node := trie.EnsureNode("/api")
	if node == nil {
		t.Error("expected non-nil node")
	}

	if len(node.child) != 0 {
		t.Error("expected leaf node with no children")
	}
}

func TestTrie_EnsureNode_MultiSegment(t *testing.T) {
	trie := NewTrie()

	node := trie.EnsureNode("/api/v1/users")
	if node == nil {
		t.Error("expected non-nil node")
	}

	retrieved, exists := trie.GetNode("/api/v1/users")
	if !exists {
		t.Error("expected node to exist after ensure")
	}
	if retrieved != node {
		t.Error("expected to retrieve same node")
	}
}

func TestTrie_EnsureNode_ExistingNode(t *testing.T) {
	trie := NewTrie()

	node1 := trie.EnsureNode("/api/v1")
	node2 := trie.EnsureNode("/api/v1")

	if node1 != node2 {
		t.Error("expected same node when ensuring existing path")
	}
}

func TestTrie_EnsureNode_EmptyPath(t *testing.T) {
	trie := NewTrie()

	node := trie.EnsureNode("")
	if node != trie.root {
		t.Error("expected root node for empty path")
	}
}

func TestTrie_EnsureNode_RootOnly(t *testing.T) {
	trie := NewTrie()

	node := trie.EnsureNode("/")
	if node != trie.root {
		t.Error("expected root node for / path")
	}
}

func TestTrie_AddConnection(t *testing.T) {
	trie := NewTrie()
	conn := &Connection{Id: "conn-1", ConnIP: "10.0.0.1"}

	trie.AddConnection([]string{"/api/v1"}, conn)

	conns := trie.GetConnections("/api/v1")
	if len(conns) != 1 {
		t.Errorf("expected 1 connection, got %d", len(conns))
	}
	if conns[0].Id != "conn-1" {
		t.Errorf("expected connection id 'conn-1', got %q", conns[0].Id)
	}
}

func TestTrie_AddConnection_MultiplePaths(t *testing.T) {
	trie := NewTrie()
	conn := &Connection{Id: "conn-1", ConnIP: "10.0.0.1"}

	trie.AddConnection([]string{"/api/v1", "/api/v2"}, conn)

	conns1 := trie.GetConnections("/api/v1")
	if len(conns1) != 1 {
		t.Errorf("expected 1 connection for /api/v1, got %d", len(conns1))
	}

	conns2 := trie.GetConnections("/api/v2")
	if len(conns2) != 1 {
		t.Errorf("expected 1 connection for /api/v2, got %d", len(conns2))
	}
}

func TestTrie_AddConnection_MultipleConnections(t *testing.T) {
	trie := NewTrie()
	conn1 := &Connection{Id: "conn-1", ConnIP: "10.0.0.1"}
	conn2 := &Connection{Id: "conn-2", ConnIP: "10.0.0.2"}

	trie.AddConnection([]string{"/api"}, conn1)
	trie.AddConnection([]string{"/api"}, conn2)

	conns := trie.GetConnections("/api")
	if len(conns) != 2 {
		t.Errorf("expected 2 connections, got %d", len(conns))
	}
}

func TestTrie_GetConnections_NotFound(t *testing.T) {
	trie := NewTrie()

	conns := trie.GetConnections("/nonexistent")
	if conns != nil {
		t.Errorf("expected nil connections for non-existent path, got %d", len(conns))
	}
}

func TestTrie_GetConnections_NoConnections(t *testing.T) {
	trie := NewTrie()
	trie.EnsureNode("/api")

	conns := trie.GetConnections("/api")
	if conns == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
}

func TestTrie_RemoveConnectionByID(t *testing.T) {
	trie := NewTrie()
	conn1 := &Connection{Id: "conn-1", ConnIP: "10.0.0.1"}
	conn2 := &Connection{Id: "conn-2", ConnIP: "10.0.0.2"}

	trie.AddConnection([]string{"/api"}, conn1)
	trie.AddConnection([]string{"/api"}, conn2)

	trie.RemoveConnectionByID("/api", "conn-1")

	conns := trie.GetConnections("/api")
	if len(conns) != 1 {
		t.Errorf("expected 1 connection after removal, got %d", len(conns))
	}
	if conns[0].Id != "conn-2" {
		t.Errorf("expected connection 'conn-2', got %q", conns[0].Id)
	}
}

func TestTrie_RemoveConnectionByID_NotFound(t *testing.T) {
	trie := NewTrie()
	conn := &Connection{Id: "conn-1", ConnIP: "10.0.0.1"}

	trie.AddConnection([]string{"/api"}, conn)

	trie.RemoveConnectionByID("/api", "nonexistent")

	conns := trie.GetConnections("/api")
	if len(conns) != 1 {
		t.Errorf("expected 1 connection, got %d", len(conns))
	}
}

func TestTrie_RemoveConnectionByID_PathNotFound(t *testing.T) {
	trie := NewTrie()

	trie.RemoveConnectionByID("/nonexistent", "conn-1")
}

func TestTrie_RemovePath(t *testing.T) {
	trie := NewTrie()
	trie.EnsureNode("/api/v1/users")

	trie.RemovePath("/api/v1/users")

	_, exists := trie.GetNode("/api/v1/users")
	if exists {
		t.Error("expected path to be removed")
	}
}

func TestTrie_RemovePath_NotFound(t *testing.T) {
	trie := NewTrie()

	trie.RemovePath("/nonexistent")
}

func TestTrie_RemovePath_Empty(t *testing.T) {
	trie := NewTrie()

	trie.RemovePath("")
	trie.RemovePath("/")
}

func TestTrie_RemovePath_ParentStillExists(t *testing.T) {
	trie := NewTrie()
	trie.EnsureNode("/api/v1/users")
	trie.EnsureNode("/api/v1/orders")

	trie.RemovePath("/api/v1/users")

	_, exists := trie.GetNode("/api/v1/orders")
	if !exists {
		t.Error("expected /api/v1/orders to still exist")
	}
}

func TestTrie_ConcurrentAccess(t *testing.T) {
	trie := NewTrie()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(3)

		go func(id int) {
			defer wg.Done()
			path := "/api/v" + string(rune('0'+id%10))
			trie.EnsureNode(path)
		}(i)

		go func(id int) {
			defer wg.Done()
			path := "/api/v" + string(rune('0'+id%10))
			trie.GetConnections(path)
		}(i)

		go func(id int) {
			defer wg.Done()
			conn := &Connection{Id: "conn-" + string(rune('0'+id%10)), ConnIP: "10.0.0.1"}
			path := "/api/v" + string(rune('0'+id%10))
			trie.AddConnection([]string{path}, conn)
		}(i)
	}

	wg.Wait()
}

func TestTrie_GetConnections_ReturnsCopy(t *testing.T) {
	trie := NewTrie()
	conn := &Connection{Id: "conn-1", ConnIP: "10.0.0.1"}

	trie.AddConnection([]string{"/api"}, conn)

	conns1 := trie.GetConnections("/api")
	conns1[0] = &Connection{Id: "modified", ConnIP: "10.0.0.2"}

	conns2 := trie.GetConnections("/api")
	if conns2[0].Id != "conn-1" {
		t.Error("expected original connection, modification should not affect trie")
	}
}
