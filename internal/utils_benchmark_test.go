package internal

import (
	"bytes"
	"strings"
	"testing"

	"QMesh-Sidecar/internal/protos/pb/gen"
)

func BenchmarkZeroAllocBytesToString_Short(b *testing.B) {
	data := []byte("hello")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ZeroAllocBytesToString(data)
	}
}

func BenchmarkZeroAllocBytesToString_Medium(b *testing.B) {
	data := []byte("/api/v1/users/12345/orders")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ZeroAllocBytesToString(data)
	}
}

func BenchmarkZeroAllocBytesToString_Long(b *testing.B) {
	data := []byte("Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ZeroAllocBytesToString(data)
	}
}

func BenchmarkZeroAllocBytesToString_Large(b *testing.B) {
	data := make([]byte, 65536)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ZeroAllocBytesToString(data)
	}
}

func BenchmarkZeroAllocStringToBytes_Short(b *testing.B) {
	s := "hello"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ZeroAllocStringToBytes(s)
	}
}

func BenchmarkZeroAllocStringToBytes_Medium(b *testing.B) {
	s := "/api/v1/users/12345/orders"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ZeroAllocStringToBytes(s)
	}
}

func BenchmarkZeroAllocStringToBytes_Long(b *testing.B) {
	s := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ZeroAllocStringToBytes(s)
	}
}

func BenchmarkStringConversion_RoundTrip(b *testing.B) {
	original := []byte("/api/v1/resource/data")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := ZeroAllocBytesToString(original)
		_ = ZeroAllocStringToBytes(s)
	}
}

func BenchmarkStdConversion_BytesToString(b *testing.B) {
	data := []byte("/api/v1/users/12345/orders")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = string(data)
	}
}

func BenchmarkStdConversion_StringToBytes(b *testing.B) {
	s := "/api/v1/users/12345/orders"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = []byte(s)
	}
}

func BenchmarkMarshalToWriter_Small(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_GET,
		Path:          []byte("/health"),
		PackedHeaders: []uint32{1},
	}

	var buf bytes.Buffer

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		MarshalToWriter(&buf, treq)
	}
}

func BenchmarkMarshalToWriter_Medium(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_POST,
		Path:          []byte("/api/v1/users"),
		Body:          []byte(`{"name":"test","email":"test@example.com"}`),
		PackedHeaders: []uint32{1, 18},
		RawHeaders:    [][]byte{[]byte("X-Request-ID"), []byte("abc-123")},
	}

	var buf bytes.Buffer

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		MarshalToWriter(&buf, treq)
	}
}

func BenchmarkMarshalToWriter_Large(b *testing.B) {
	tresp := &gen.TunnelResponse{
		CodeResponse:  200,
		Body:          make([]byte, 32768),
		PackedHeaders: []uint32{1, 13, 16},
		RawHeaders:    [][]byte{[]byte("X-Response-Time"), []byte("42ms"), []byte("X-Trace-ID"), []byte("trace-xyz-789")},
	}

	var buf bytes.Buffer

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		MarshalToWriter(&buf, tresp)
	}
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bufferPool.Get().(*buff)
		buf.buff = buf.buff[:0]
		bufferPool.Put(buf)
	}
}

func BenchmarkSyncPool_Request_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := requests.Get().(*gen.TunnelRequest)
			req.Reset()
			requests.Put(req)
		}
	})
}

func BenchmarkSyncPool_Response_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp := responses.Get().(*gen.TunnelResponse)
			resp.Reset()
			responses.Put(resp)
		}
	})
}

func BenchmarkSyncPool_Builder_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			builder := builders.Get().(*strings.Builder)
			builder.Reset()
			builders.Put(builder)
		}
	})
}
