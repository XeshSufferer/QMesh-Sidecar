package internal

import (
	"strings"
	"sync"
)

type node struct {
	child       map[string]*node
	connections []*Connection
}

type Trie struct {
	root *node
	mu   sync.RWMutex
}

func NewTrie() *Trie {
	return &Trie{
		root: &node{
			child:       make(map[string]*node),
			connections: make([]*Connection, 0),
		},
	}
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func (t *Trie) GetNode(path string) (*node, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	prefixes := splitPath(path)
	current := t.root

	for _, v := range prefixes {
		next, ok := current.child[v]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func (t *Trie) EnsureNode(path string) *node {
	t.mu.Lock()
	defer t.mu.Unlock()

	prefixes := splitPath(path)
	current := t.root

	for _, v := range prefixes {
		if _, ok := current.child[v]; !ok {
			current.child[v] = &node{
				child:       make(map[string]*node),
				connections: make([]*Connection, 0),
			}
		}
		current = current.child[v]
	}
	return current
}

func (t *Trie) AddConnection(paths []string, conn *Connection) {
	for _, path := range paths {
		n := t.EnsureNode(path)
		t.mu.Lock()
		n.connections = append(n.connections, conn)
		t.mu.Unlock()
	}
}

func (t *Trie) GetConnections(path string) []*Connection {
	n, exists := t.GetNode(path)
	if !exists {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	res := make([]*Connection, len(n.connections))
	copy(res, n.connections)
	return res
}

func (t *Trie) RemoveConnectionByID(path string, connID string) {
	n, exists := t.GetNode(path)
	if !exists {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	newConns := make([]*Connection, 0, len(n.connections))
	for _, c := range n.connections {
		if c.Id != connID {
			newConns = append(newConns, c)
		}
	}
	n.connections = newConns
}

func (t *Trie) RemovePath(path string) {
	prefixes := splitPath(path)
	if len(prefixes) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	current := t.root
	for i := 0; i < len(prefixes)-1; i++ {
		next, ok := current.child[prefixes[i]]
		if !ok {
			return
		}
		current = next
	}

	delete(current.child, prefixes[len(prefixes)-1])
}
