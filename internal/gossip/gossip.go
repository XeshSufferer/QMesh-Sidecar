package gossip

import (
	"log"
	"net"
	"sync"
	"time"

	"QMesh-Sidecar/internal/protos/pb/gen/gossip"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type MemberListener interface {
	OnNodeJoin(ip string)
	OnNodeLeave(ip string)
}

type Gossip struct {
	seeds          []string
	memberListener MemberListener
	state          *GossipState
	udpConn        *net.UDPConn
	worker         *Worker
	mu             sync.RWMutex
	running        bool
	stopCh         chan struct{}
}

func NewGossip(seeds []string) *Gossip {
	return &Gossip{
		seeds:  seeds,
		state:  NewGossipState(),
		stopCh: make(chan struct{}),
	}
}

func (g *Gossip) SetMemberListener(listener MemberListener) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.memberListener = listener
}

func (g *Gossip) Start() error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return nil
	}
	g.running = true
	g.mu.Unlock()

	addr, err := net.ResolveUDPAddr("udp", GossipPort)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	g.udpConn = conn

	go g.listen()
	
	g.worker = NewWorker(g)
	g.worker.Start()

	log.Printf("Gossip started on %s", GossipPort)
	return nil
}

func (g *Gossip) Stop() {
	g.mu.Lock()
	if !g.running {
		g.mu.Unlock()
		return
	}
	g.running = false
	close(g.stopCh)
	g.mu.Unlock()

	if g.worker != nil {
		g.worker.Stop()
	}
	if g.udpConn != nil {
		g.udpConn.Close()
	}
	log.Println("Gossip stopped")
}

func (g *Gossip) OnNodeJoin(ip string) {
	g.state.Update(ip, gossip.Status_NEW)
	g.notifyListener(ip, gossip.Status_NEW)
}

func (g *Gossip) OnNodeLeave(ip string) {
	g.state.Update(ip, gossip.Status_DEAD)
	g.notifyListener(ip, gossip.Status_DEAD)
}

func (g *Gossip) notifyListener(ip string, status gossip.Status) {
	g.mu.RLock()
	listener := g.memberListener
	g.mu.RUnlock()

	if listener != nil {
		if status == gossip.Status_DEAD {
			listener.OnNodeLeave(ip)
		} else {
			listener.OnNodeJoin(ip)
		}
	}
}

func (g *Gossip) listen() {
	buffer := make([]byte, 65535)
	for {
		select {
		case <-g.stopCh:
			return
		default:
		}

		g.udpConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := g.udpConn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		go g.handleMessage(buffer[:n], addr)
	}
}

func (g *Gossip) handleMessage(data []byte, addr *net.UDPAddr) {
	var msg gossip.GossipMsg
	if err := proto.Unmarshal(data, &msg); err != nil {
		return
	}

	otherState := make(map[string]*NodeState)
	for _, diff := range msg.GetDiff() {
		otherState[diff.GetNodeIp()] = &NodeState{
			Status:   diff.GetStatus(),
			Version:  msg.GetVersion(),
			LastSeen: time.Now(),
		}
	}

	oldHash := g.state.GetHash()
	if string(oldHash) != string(msg.GetListHash()) {
		g.state.Merge(otherState)
		g.notifyChanges(otherState)
	}

	g.BroadcastTo(addr)
}

func (g *Gossip) notifyChanges(newNodes map[string]*NodeState) {
	g.mu.RLock()
	listener := g.memberListener
	g.mu.RUnlock()

	if listener == nil {
		return
	}

	for ip, node := range newNodes {
		if state, exists := g.state.Get(ip); exists {
			if state.Status == gossip.Status_NEW && node.Status == gossip.Status_NEW {
				listener.OnNodeJoin(ip)
			}
		}
	}
}

func (g *Gossip) Broadcast() {
	msg := g.buildMessage()

	g.mu.RLock()
	seeds := make([]string, len(g.seeds))
	copy(seeds, g.seeds)
	g.mu.RUnlock()

	for _, seed := range seeds {
		g.sendToSeed(seed, msg)
	}
}

func (g *Gossip) BroadcastTo(addr *net.UDPAddr) {
	msg := g.buildMessage()
	data, err := proto.Marshal(msg)
	if err != nil {
		return
	}
	g.udpConn.WriteToUDP(data, addr)
}

func (g *Gossip) buildMessage() *gossip.GossipMsg {
	return &gossip.GossipMsg{
		Timestamp: timestamppb.Now(),
		ListHash:  g.state.GetHash(),
		Ttl:        3,
		Version:    g.state.GetVersion(),
		Diff:       g.buildDiff(),
	}
}

func (g *Gossip) buildDiff() []*gossip.NodeUpdates {
	all := g.state.GetAll()
	diff := make([]*gossip.NodeUpdates, 0, len(all))

	for ip, node := range all {
		diff = append(diff, &gossip.NodeUpdates{
			NodeIp: ip,
			Status: node.Status,
		})
	}

	return diff
}

func (g *Gossip) sendToSeed(seed string, msg *gossip.GossipMsg) {
	addr, err := net.ResolveUDPAddr("udp", seed)
	if err != nil {
		return
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return
	}

	g.udpConn.WriteToUDP(data, addr)
}

func (g *Gossip) GetSeeds() []string {
	return g.seeds
}

func (g *Gossip) GetState() *GossipState {
	return g.state
}
