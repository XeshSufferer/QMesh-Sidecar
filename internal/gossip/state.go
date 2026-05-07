package gossip

import (
	"sync"
	"time"
	"unsafe"

	"QMesh-Sidecar/internal/protos/pb/gen/gossip"
	"github.com/cespare/xxhash/v2"
)

type hashBuf struct {
	buf [8]byte
}

var (
	nodeUpdatesPool = sync.Pool{
		New: func() any {
			return &gossip.NodeUpdates{}
		},
	}
	hashBytesPool = sync.Pool{
		New: func() any {
			return &hashBuf{}
		},
	}
)

type NodeState struct {
	Status   gossip.Status
	Version  uint32
	LastSeen time.Time
}

type GossipState struct {
	mu      sync.RWMutex
	nodes   map[string]*NodeState
	version uint32
	diffBuf []*gossip.NodeUpdates
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
	node, ok := s.nodes[ip]
	s.mu.RUnlock()

	if ok {
		return *node, true
	}
	return NodeState{}, false
}

func (s *GossipState) GetAll() map[string]*NodeState {
	s.mu.RLock()

	result := make(map[string]*NodeState, len(s.nodes))
	for k, v := range s.nodes {
		result[k] = v
	}
	s.mu.RUnlock()

	return result
}

func (s *GossipState) GetVersion() uint32 {
	s.mu.RLock()
	v := s.version
	s.mu.RUnlock()
	return v
}

func (s *GossipState) GetHash() uint64 {
	s.mu.RLock()

	var result uint64
	for ip, node := range s.nodes {
		nodeHash := xxhash.Sum64String(ip) ^ (uint64(node.Status) << 48) ^ (uint64(node.Version) << 32)
		result ^= nodeHash
	}

	s.mu.RUnlock()

	// Final mix for avalanche effect (no allocation)
	result ^= result >> 33
	result *= 0xff51afd7ed558ccd
	result ^= result >> 33
	result *= 0xc4ceb9fe1a85ec53
	result ^= result >> 33
	return result
}

func (s *GossipState) GetHashBytes() []byte {
	s.mu.RLock()

	var result uint64
	for ip, node := range s.nodes {
		nodeHash := xxhash.Sum64String(ip) ^ (uint64(node.Status) << 48) ^ (uint64(node.Version) << 32)
		result ^= nodeHash
	}
	s.mu.RUnlock()

	// Final mix
	result ^= result >> 33
	result *= 0xff51afd7ed558ccd
	result ^= result >> 33
	result *= 0xc4ceb9fe1a85ec53
	result ^= result >> 33

	hb := hashBytesPool.Get().(*hashBuf)
	hb.buf[0] = byte(result)
	hb.buf[1] = byte(result >> 8)
	hb.buf[2] = byte(result >> 16)
	hb.buf[3] = byte(result >> 24)
	hb.buf[4] = byte(result >> 32)
	hb.buf[5] = byte(result >> 40)
	hb.buf[6] = byte(result >> 48)
	hb.buf[7] = byte(result >> 56)
	return hb.buf[:]
}

func ReleaseHashBytes(b []byte) {
	if cap(b) == 8 {
		hb := (*hashBuf)(unsafe.Pointer(&b[0]))
		hashBytesPool.Put(hb)
	}
}

func (s *GossipState) Merge(other map[string]*NodeState) {
	s.mu.Lock()

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

	s.mu.Unlock()
}

func (s *GossipState) GetDiff() []*gossip.NodeUpdates {
	s.mu.RLock()

	if cap(s.diffBuf) < len(s.nodes) {
		s.diffBuf = make([]*gossip.NodeUpdates, 0, len(s.nodes))
	}
	s.diffBuf = s.diffBuf[:0]

	for ip, node := range s.nodes {
		nu := nodeUpdatesPool.Get().(*gossip.NodeUpdates)
		nu.NodeIp = ip
		nu.Status = node.Status
		s.diffBuf = append(s.diffBuf, nu)
	}

	s.mu.RUnlock()
	return s.diffBuf
}

func ReleaseDiff(diff []*gossip.NodeUpdates) {
	for i := range diff {
		diff[i].NodeIp = ""
		diff[i].Status = 0
		nodeUpdatesPool.Put(diff[i])
	}
}
