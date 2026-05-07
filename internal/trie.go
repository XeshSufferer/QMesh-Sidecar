package internal

import (
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

func (t *Trie) GetNode(path string) (*node, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	startTrim := 0
	endTrim := len(path)

	if endTrim > 0 && path[0] == '/' {
		startTrim++
	}
	if endTrim > startTrim && path[endTrim-1] == '/' {
		endTrim--
	}

	if endTrim <= startTrim {
		return t.root, true
	}

	pathBytes := ZeroAllocStringToBytes(path)
	current := t.root
	start := startTrim

	for i := startTrim; i <= endTrim; i++ {
		if i == endTrim || pathBytes[i] == '/' {
			if i > start {
				seg := pathBytes[start:i]
				next, ok := current.child[string(seg)]
				if !ok {
					return nil, false
				}
				current = next
			}
			start = i + 1
		}
	}

	return current, true
}

func (t *Trie) EnsureNode(pathStr string) *node {
	t.mu.Lock()
	defer t.mu.Unlock()

	startTrim := 0
	endTrim := len(pathStr)

	if endTrim > 0 && pathStr[0] == '/' {
		startTrim++
	}
	if endTrim > startTrim && pathStr[endTrim-1] == '/' {
		endTrim--
	}

	pathBytes := ZeroAllocStringToBytes(pathStr)
	current := t.root
	if endTrim <= startTrim {
		return current
	}

	start := startTrim
	for i := startTrim; i <= endTrim; i++ {
		if i == endTrim || pathBytes[i] == '/' {
			if i > start {
				seg := pathBytes[start:i]
				key := string(seg)
				next, ok := current.child[key]
				if !ok {
					next = &node{
						child:       make(map[string]*node),
						connections: make([]*Connection, 0),
					}
					current.child[key] = next
				}
				current = next
			}
			start = i + 1
		}
	}
	return current
}

func (t *Trie) dangerEnsureNode(path string) *node {
	startTrim := 0
	endTrim := len(path)

	if endTrim > 0 && path[0] == '/' {
		startTrim++
	}
	if endTrim > startTrim && path[endTrim-1] == '/' {
		endTrim--
	}

	pathBytes := ZeroAllocStringToBytes(path)
	current := t.root
	if endTrim <= startTrim {
		return current
	}

	start := startTrim
	for i := startTrim; i <= endTrim; i++ {
		if i == endTrim || pathBytes[i] == '/' {
			if i > start {
				seg := pathBytes[start:i]
				key := string(seg)
				next, ok := current.child[key]
				if !ok {
					next = &node{
						child:       make(map[string]*node),
						connections: make([]*Connection, 0),
					}
					current.child[key] = next
				}
				current = next
			}
			start = i + 1
		}
	}
	return current
}

func (t *Trie) AddConnection(paths []string, conn *Connection) {
	for _, path := range paths {
		t.mu.Lock()
		n := t.dangerEnsureNode(path)
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
	startTrim := 0
	endTrim := len(path)

	if endTrim > 0 && path[0] == '/' {
		startTrim++
	}
	if endTrim > startTrim && path[endTrim-1] == '/' {
		endTrim--
	}

	if endTrim <= startTrim {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	pathBytes := ZeroAllocStringToBytes(path)
	current := t.root
	var lastParent *node
	var lastSegment string

	start := startTrim
	for i := startTrim; i <= endTrim; i++ {
		if i == endTrim || pathBytes[i] == '/' {
			if i > start {
				seg := pathBytes[start:i]
				key := string(seg)
				next, ok := current.child[key]
				if !ok {
					return
				}
				lastParent = current
				lastSegment = key
				current = next
			}
			start = i + 1
		}
	}

	if lastParent != nil {
		delete(lastParent.child, lastSegment)
	}
}
