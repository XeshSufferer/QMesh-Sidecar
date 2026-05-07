package internal

import (
	"bytes"
	"strconv"
	"testing"
)

func setupTrieWithPaths(numPaths int, connsPerPath int) (*Trie, [][]byte, [][]byte, [][]byte) {
	paths := make([][]byte, numPaths)
	connIDs := make([][]byte, numPaths*connsPerPath)
	connIPs := make([][]byte, numPaths*connsPerPath)

	buf := bytes.NewBuffer(make([]byte, 0, 64))
	trie := NewTrie()

	connIdx := 0
	for i := 0; i < numPaths; i++ {
		buf.Reset()
		buf.WriteString("/api/v")
		buf.WriteString(strconv.Itoa(i / 100))
		buf.WriteString("/resource")
		buf.WriteString(strconv.Itoa(i % 100))
		buf.WriteString("/sub")
		buf.WriteString(strconv.Itoa(i % 10))
		paths[i] = make([]byte, buf.Len())
		copy(paths[i], buf.Bytes())

		for j := 0; j < connsPerPath; j++ {
			buf.Reset()
			buf.WriteString("conn-")
			buf.WriteString(strconv.Itoa(i))
			buf.WriteByte('-')
			buf.WriteString(strconv.Itoa(j))
			connIDs[connIdx] = make([]byte, buf.Len())
			copy(connIDs[connIdx], buf.Bytes())

			buf.Reset()
			buf.WriteString("10.0.")
			buf.WriteString(strconv.Itoa(i / 256))
			buf.WriteByte('.')
			buf.WriteString(strconv.Itoa(i % 256))
			connIPs[connIdx] = make([]byte, buf.Len())
			copy(connIPs[connIdx], buf.Bytes())

			conn := &Connection{
				Id:     ZeroAllocBytesToString(connIDs[connIdx]),
				ConnIP: ZeroAllocBytesToString(connIPs[connIdx]),
			}
			trie.AddConnection([]string{ZeroAllocBytesToString(paths[i])}, conn)
			connIdx++
		}
	}

	return trie, paths, connIDs, connIPs
}

func BenchmarkTrie_GetConnections_ShallowHit(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(100, 3)
	path := paths[5%len(paths)]

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetConnections(ZeroAllocBytesToString(path))
	}
}

func BenchmarkTrie_GetConnections_DeepHit(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(1000, 5)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetConnections(ZeroAllocBytesToString(paths[i%len(paths)]))
	}
}

func BenchmarkTrie_GetConnections_Miss(b *testing.B) {
	trie, _, _, _ := setupTrieWithPaths(100, 3)
	missPath := []byte("/nonexistent/path/here")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetConnections(ZeroAllocBytesToString(missPath))
	}
}

func BenchmarkTrie_GetConnections_ManyConns(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(10, 50)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetConnections(ZeroAllocBytesToString(paths[0]))
	}
}

func BenchmarkTrie_GetConnections_SinglePath(b *testing.B) {
	trie := NewTrie()
	healthPath := []byte("/health")
	trie.AddConnection([]string{ZeroAllocBytesToString(healthPath)}, &Connection{Id: "conn-1", ConnIP: "10.0.0.1"})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetConnections(ZeroAllocBytesToString(healthPath))
	}
}

func BenchmarkTrie_EnsureNode_NewPath(b *testing.B) {
	trie := NewTrie()
	buf := make([]byte, 0, 64)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		buf = append(buf, "/api/v"...)
		buf = strconv.AppendInt(buf, int64(i/1000), 10)
		buf = append(buf, "/new/resource"...)
		buf = strconv.AppendInt(buf, int64(i%100), 10)
		buf = append(buf, "/sub"...)
		buf = strconv.AppendInt(buf, int64(i%10), 10)
		trie.EnsureNode(ZeroAllocBytesToString(buf))
	}
}

func BenchmarkTrie_EnsureNode_ExistingPath(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(500, 1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.EnsureNode(ZeroAllocBytesToString(paths[i%len(paths)]))
	}
}

func BenchmarkTrie_AddConnection_SinglePath(b *testing.B) {
	trie := NewTrie()
	path := []byte("/api/v1/users")
	trie.EnsureNode(ZeroAllocBytesToString(path))

	buf := make([]byte, 0, 32)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		buf = append(buf, "conn-"...)
		buf = strconv.AppendInt(buf, int64(i), 10)
		connID := make([]byte, len(buf))
		copy(connID, buf)

		conn := &Connection{
			Id:     ZeroAllocBytesToString(connID),
			ConnIP: "10.0.0.1",
		}
		trie.AddConnection([]string{ZeroAllocBytesToString(path)}, conn)
	}
}

func BenchmarkTrie_AddConnection_MultiPath(b *testing.B) {
	trie := NewTrie()
	p1 := []byte("/api/v1/users")
	p2 := []byte("/api/v1/orders")
	p3 := []byte("/api/v2/items")
	trie.EnsureNode(ZeroAllocBytesToString(p1))
	trie.EnsureNode(ZeroAllocBytesToString(p2))
	trie.EnsureNode(ZeroAllocBytesToString(p3))
	paths := []string{
		ZeroAllocBytesToString(p1),
		ZeroAllocBytesToString(p2),
		ZeroAllocBytesToString(p3),
	}

	const BenchDataSize = 1000

	buf := make([]byte, 0, 32)

	benchData := make([]*Connection, 0, BenchDataSize+1)

	for i := range BenchDataSize + 1 {
		buf = buf[:0]
		buf = append(buf, "conn-"...)
		buf = strconv.AppendInt(buf, int64(i), 10)
		connID := make([]byte, len(buf))
		copy(connID, buf)

		conn := &Connection{
			Id:     ZeroAllocBytesToString(connID),
			ConnIP: "10.0.0.1",
		}
		benchData = append(benchData, conn)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 1; i < b.N; i++ {

		trie.AddConnection(paths, benchData[BenchDataSize%i])
	}
}

func BenchmarkTrie_RemoveConnectionByID(b *testing.B) {
	trie, paths, connIDs, _ := setupTrieWithPaths(10, 100)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.RemoveConnectionByID(
			ZeroAllocBytesToString(paths[0]),
			ZeroAllocBytesToString(connIDs[i%len(connIDs)]),
		)
	}
}

func BenchmarkTrie_RemovePath(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(500, 3)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.RemovePath(ZeroAllocBytesToString(paths[i%len(paths)]))
	}
}

func BenchmarkTrie_GetNode(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(1000, 3)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetNode(ZeroAllocBytesToString(paths[i%len(paths)]))
	}
}

func BenchmarkTrie_ConcurrentReads(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(200, 5)
	numPaths := len(paths)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			trie.GetConnections(ZeroAllocBytesToString(paths[idx%numPaths]))
			idx++
		}
	})
}

func BenchmarkTrie_ConcurrentReadWrite(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(100, 3)
	numPaths := len(paths)

	buf := bytes.NewBuffer(make([]byte, 0, 32))
	connIDPool := make([][]byte, 0, 1000)
	for i := 0; i < 1000; i++ {
		buf.Reset()
		buf.WriteString("conn-rw-")
		buf.WriteString(strconv.Itoa(i))
		id := make([]byte, buf.Len())
		copy(id, buf.Bytes())
		connIDPool = append(connIDPool, id)
	}
	numConnIDs := len(connIDPool)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		for pb.Next() {
			if idx%10 == 0 {
				conn := &Connection{
					Id:     ZeroAllocBytesToString(connIDPool[idx%numConnIDs]),
					ConnIP: "10.0.0.1",
				}
				trie.AddConnection([]string{ZeroAllocBytesToString(paths[idx%numPaths])}, conn)
			} else {
				trie.GetConnections(ZeroAllocBytesToString(paths[idx%numPaths]))
			}
			idx++
		}
	})
}

func BenchmarkTrie_Scale_1000Paths(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(1000, 3)
	numPaths := len(paths)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetConnections(ZeroAllocBytesToString(paths[i%numPaths]))
	}
}

func BenchmarkTrie_Scale_5000Paths(b *testing.B) {
	trie, paths, _, _ := setupTrieWithPaths(5000, 3)
	numPaths := len(paths)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.GetConnections(ZeroAllocBytesToString(paths[i%numPaths]))
	}
}
