package gossip

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"sync"
	"time"

	"QMesh-Sidecar/internal/protos/pb/gen/gossip"
)

type NodeState struct {
	Status    gossip.Status
	Version   uint32
	LastSeen  time.Time
}

type GossipState struct {
	mu       sync.RWMutex
	nodes    map[string]*NodeState
	version  uint32
}

func NewGossipState() *GossipState {
	return &GossipState{
		nodes:   make(map[string]*NodeState),
		version: 1,
	}
}

func (s *GossipState) Update(ip string, status gossip.Status) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node, exists := s.nodes[ip]; exists {
		if status != node.Status {
			node.Status = status
			node.Version++
			node.LastSeen = time.Now()
			s.version++
		}
	} else {
		s.nodes[ip] = &NodeState{
			Status:   status,
			Version:  1,
			LastSeen: time.Now(),
		}
		s.version++
	}
}

func (s *GossipState) Get(ip string) (NodeState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if node, ok := s.nodes[ip]; ok {
		return *node, true
	}
	return NodeState{}, false
}

func (s *GossipState) GetAll() map[string]*NodeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*NodeState, len(s.nodes))
	for k, v := range s.nodes {
		result[k] = v
	}
	return result
}

func (s *GossipState) GetVersion() uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

func (s *GossipState) GetHash() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.nodes))
	for k := range s.nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		node := s.nodes[k]
		h.Write([]byte(k))
		h.Write([]byte{byte(node.Status)})
		ver := make([]byte, 4)
		binary.BigEndian.PutUint32(ver, node.Version)
		h.Write(ver)
	}
	return h.Sum(nil)
}

func (s *GossipState) Merge(other map[string]*NodeState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for ip, otherNode := range other {
		if localNode, exists := s.nodes[ip]; exists {
			if otherNode.Version > localNode.Version {
				s.nodes[ip] = otherNode
				s.version++
			}
		} else {
			s.nodes[ip] = otherNode
			s.version++
		}
	}
}
