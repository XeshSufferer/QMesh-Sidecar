package gossip

import (
	"strconv"
	"testing"
	"unsafe"

	pbg "QMesh-Sidecar/internal/protos/pb/gen/gossip"
)

func zeroAllocBytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func appendIP(buf []byte, a, b byte) []byte {
	buf = buf[:0]
	buf = append(buf, "10.0."...)
	buf = strconv.AppendInt(buf, int64(a), 10)
	buf = append(buf, '.')
	buf = strconv.AppendInt(buf, int64(b), 10)
	return buf
}

func setupGossipState(numNodes int) (*GossipState, [][]byte) {
	state := NewGossipState()
	ips := make([][]byte, numNodes)
	buf := make([]byte, 0, 24)

	for i := 0; i < numNodes; i++ {
		ips[i] = appendIP(buf, byte(i/256), byte(i%256))
		bufCopy := make([]byte, len(ips[i]))
		copy(bufCopy, ips[i])
		ips[i] = bufCopy
		state.Update(zeroAllocBytesToString(ips[i]), pbg.Status_NEW)
	}

	return state, ips
}

func BenchmarkGossipState_Update_NewNode(b *testing.B) {
	state := NewGossipState()
	buf := make([]byte, 0, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := appendIP(buf, byte(i/256), byte(i%256))
		ipCopy := make([]byte, len(ip))
		copy(ipCopy, ip)
		state.Update(zeroAllocBytesToString(ipCopy), pbg.Status_NEW)
	}
}

func BenchmarkGossipState_Update_ExistingNode(b *testing.B) {
	state, ips := setupGossipState(1000)
	buf := make([]byte, 0, 24)
	numIPs := len(ips)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := appendIP(buf, byte(i/1000), byte(i%1000))
		state.Update(zeroAllocBytesToString(ip), pbg.Status_SUSPECT)
	}
	_ = ips
	_ = numIPs
}

func BenchmarkGossipState_Get_Hit(b *testing.B) {
	state, ips := setupGossipState(1000)
	buf := make([]byte, 0, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := appendIP(buf, byte(i/1000), byte(i%1000))
		state.Get(zeroAllocBytesToString(ip))
	}
	_ = ips
}

func BenchmarkGossipState_Get_Miss(b *testing.B) {
	state, _ := setupGossipState(100)
	buf := make([]byte, 0, 24)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		buf = append(buf, "192.168.1."...)
		buf = strconv.AppendInt(buf, int64(i%256), 10)
		state.Get(zeroAllocBytesToString(buf))
	}
}

func BenchmarkGossipState_GetAll_Small(b *testing.B) {
	state, _ := setupGossipState(50)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetAll()
	}
}

func BenchmarkGossipState_GetAll_Medium(b *testing.B) {
	state, _ := setupGossipState(500)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetAll()
	}
}

func BenchmarkGossipState_GetAll_Large(b *testing.B) {
	state, _ := setupGossipState(2000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetAll()
	}
}

func BenchmarkGossipState_GetHash_Small(b *testing.B) {
	state, _ := setupGossipState(50)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetHash()
	}
}

func BenchmarkGossipState_GetHash_Medium(b *testing.B) {
	state, _ := setupGossipState(500)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetHash()
	}
}

func BenchmarkGossipState_GetHash_Large(b *testing.B) {
	state, _ := setupGossipState(2000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetHash()
	}
}

func BenchmarkGossipState_GetHash_XLarge(b *testing.B) {
	state, _ := setupGossipState(5000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetHash()
	}
}

func BenchmarkGossipState_Merge_Small(b *testing.B) {
	local, _ := setupGossipState(100)
	remote := make(map[string]*NodeState, 50)
	buf := make([]byte, 0, 24)
	for i := 100; i < 150; i++ {
		ip := appendIP(buf, byte(i/256), byte(i%256))
		ipCopy := make([]byte, len(ip))
		copy(ipCopy, ip)
		remote[zeroAllocBytesToString(ipCopy)] = &NodeState{Status: pbg.Status_NEW, Version: 1}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		local.Merge(remote)
	}
}

func BenchmarkGossipState_Merge_Medium(b *testing.B) {
	local, _ := setupGossipState(500)
	remote := make(map[string]*NodeState, 200)
	buf := make([]byte, 0, 24)
	for i := 0; i < 200; i++ {
		ip := appendIP(buf, byte(i/256), byte(i%256))
		ipCopy := make([]byte, len(ip))
		copy(ipCopy, ip)
		remote[zeroAllocBytesToString(ipCopy)] = &NodeState{Status: pbg.Status_SUSPECT, Version: 10}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		local.Merge(remote)
	}
}

func BenchmarkGossipState_Merge_Large(b *testing.B) {
	local, _ := setupGossipState(1000)
	remote := make(map[string]*NodeState, 500)
	buf := make([]byte, 0, 24)
	for i := 0; i < 500; i++ {
		ip := appendIP(buf, byte(i/256), byte(i%256))
		ipCopy := make([]byte, len(ip))
		copy(ipCopy, ip)
		remote[zeroAllocBytesToString(ipCopy)] = &NodeState{Status: pbg.Status_DEAD, Version: uint32(i + 100)}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		local.Merge(remote)
	}
}

func BenchmarkGossipState_GetVersion(b *testing.B) {
	state, _ := setupGossipState(100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.GetVersion()
	}
}

func BenchmarkGossip_BuildDiff_Small(b *testing.B) {
	g := &Gossip{state: setupGossipStateWithState(50)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		diff := g.state.GetDiff()
		ReleaseDiff(diff)
	}
}

func BenchmarkGossip_BuildDiff_Medium(b *testing.B) {
	g := &Gossip{state: setupGossipStateWithState(500)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		diff := g.state.GetDiff()
		ReleaseDiff(diff)
	}
}

func BenchmarkGossip_BuildDiff_Large(b *testing.B) {
	g := &Gossip{state: setupGossipStateWithState(2000)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		diff := g.state.GetDiff()
		ReleaseDiff(diff)
	}
}

func BenchmarkGossip_BuildMessage_Small(b *testing.B) {
	g := &Gossip{state: setupGossipStateWithState(50)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, diff, hashBuf := g.buildMessage()
		ReleaseDiff(diff)
		ReleaseHashBytes(hashBuf)
		g.releaseBuild()
	}
}

func BenchmarkGossip_BuildMessage_Medium(b *testing.B) {
	g := &Gossip{state: setupGossipStateWithState(200)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, diff, hashBuf := g.buildMessage()
		ReleaseDiff(diff)
		ReleaseHashBytes(hashBuf)
		g.releaseBuild()
	}
}

func BenchmarkGossipState_ConcurrentUpdates(b *testing.B) {
	state, _ := setupGossipState(100)
	buf := make([]byte, 0, 24)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			ip := appendIP(buf, byte(i/256), byte(i%256))
			ipCopy := make([]byte, len(ip))
			copy(ipCopy, ip)
			state.Update(zeroAllocBytesToString(ipCopy), pbg.Status_NEW)
			i++
		}
	})
}

func BenchmarkGossipState_ConcurrentReads_GetHash(b *testing.B) {
	state, _ := setupGossipState(500)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			state.GetHash()
		}
	})
}

func BenchmarkGossipState_ConcurrentReadWrite(b *testing.B) {
	state, _ := setupGossipState(100)
	buf := make([]byte, 0, 24)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			if i%4 == 0 {
				ip := appendIP(buf, byte(i/256), byte(i%256))
				state.Update(zeroAllocBytesToString(ip), pbg.Status_SUSPECT)
			} else {
				ip := appendIP(buf, byte((i-1)/256), byte((i-1)%256))
				state.Get(zeroAllocBytesToString(ip))
			}
			i++
		}
	})
}

func BenchmarkGossipState_Merge_Concurrent(b *testing.B) {
	state, _ := setupGossipState(200)
	buf := make([]byte, 0, 32)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			remote := make(map[string]*NodeState, 10)
			for j := 0; j < 10; j++ {
				buf = buf[:0]
				buf = append(buf, "10.1."...)
				buf = strconv.AppendInt(buf, int64((i+j)/256), 10)
				buf = append(buf, '.')
				buf = strconv.AppendInt(buf, int64((i+j)%256), 10)
				ipCopy := make([]byte, len(buf))
				copy(ipCopy, buf)
				remote[zeroAllocBytesToString(ipCopy)] = &NodeState{Status: pbg.Status_NEW, Version: 1}
			}
			state.Merge(remote)
			i += 10
		}
	})
}

func setupGossipStateWithState(n int) *GossipState {
	s, _ := setupGossipState(n)
	return s
}
