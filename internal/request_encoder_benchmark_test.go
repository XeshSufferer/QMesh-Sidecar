package internal

import (
	"testing"

	"QMesh-Sidecar/internal/protos/pb/gen"

	"github.com/valyala/fasthttp"
)

func BenchmarkEncodeRequestFast(b *testing.B) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod("GET")
	req.URI().SetPath("/api/v1/users")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBody([]byte(`{"name":"test","value":123}`))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeRequestFast(req)
	}
}

func BenchmarkDecodeRequestFast(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method: gen.HttpMethod_HTTP_METHOD_GET,
		Path:   []byte("/api/v1/users"),
		Body:   []byte(`{"name":"test","value":123}`),
		PackedHeaders: []uint32{1, 18},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeRequestFast(treq)
	}
}

func BenchmarkEncodeResponseFast(b *testing.B) {
	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(200)
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Cache-Control", "no-cache")
	resp.SetBody([]byte(`{"success":true}`))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeResponseFast(resp)
	}
}

func BenchmarkDecodeResponseFast(b *testing.B) {
	tresp := &gen.TunnelResponse{
		CodeResponse: 200,
		Body:         []byte(`{"success":true}`),
		PackedHeaders: []uint32{1, 13},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeResponseFast(tresp)
	}
}

func BenchmarkEncodeRequestFast_WithManyHeaders(b *testing.B) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod("POST")
	req.URI().SetPath("/api/v1/users")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Request-ID", "abc-123")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.SetBody([]byte(`{"name":"test","value":123,"nested":{"key":"value"}}`))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeRequestFast(req)
	}
}

func BenchmarkDecodeRequestFast_WithManyHeaders(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:       gen.HttpMethod_HTTP_METHOD_POST,
		Path:         []byte("/api/v1/users"),
		Body:         []byte(`{"name":"test","value":123,"nested":{"key":"value"}}`),
		PackedHeaders: []uint32{1, 18, 13, 10},
		RawHeaders:    []string{"Authorization", "Bearer token123", "X-Request-ID", "abc-123", "X-Custom-Header", "custom-value"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeRequestFast(treq)
	}
}

func BenchmarkEncodeRequestFast_PackedOnly(b *testing.B) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod("GET")
	req.URI().SetPath("/api/v1/users")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.SetBody([]byte(`{}`))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeRequestFast(req)
	}
}

func BenchmarkDecodeRequestFast_PackedOnly(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_GET,
		Path:          []byte("/api/v1/users"),
		Body:          []byte(`{}`),
		PackedHeaders: []uint32{1, 18, 13},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DecodeRequestFast(treq)
	}
}