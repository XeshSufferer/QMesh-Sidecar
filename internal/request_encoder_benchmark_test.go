package internal

import (
	"testing"

	"QMesh-Sidecar/internal/protos/pb/gen"

	"github.com/valyala/fasthttp"
	"google.golang.org/protobuf/proto"
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
		req, _ := EncodeRequestFast(req)
		ReleaseEncodedRequest(req)
	}
}

func BenchmarkDecodeRequestFast(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_GET,
		Path:          []byte("/api/v1/users"),
		Body:          []byte(`{"name":"test","value":123}`),
		PackedHeaders: []uint32{1, 18},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := DecodeRequestFast(treq)
		ReleaseRequest(req)
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
		respp, _ := EncodeResponseFast(resp)
		ReleaseEncodedResponse(respp)
	}
}

func BenchmarkDecodeResponseFast(b *testing.B) {
	tresp := &gen.TunnelResponse{
		CodeResponse:  200,
		Body:          []byte(`{"success":true}`),
		PackedHeaders: []uint32{1, 13},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, _ := DecodeResponseFast(tresp)
		ReleaseResponse(resp)
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
		reqq, _ := EncodeRequestFast(req)
		ReleaseEncodedRequest(reqq)
	}
}

func BenchmarkDecodeRequestFast_WithManyHeaders(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_POST,
		Path:          []byte("/api/v1/users"),
		Body:          []byte(`{"name":"test","value":123,"nested":{"key":"value"}}`),
		PackedHeaders: []uint32{1, 18, 13, 10},
		RawHeaders:    [][]byte{[]byte("Authorization"), []byte("Bearer token123"), []byte("X-Request-ID"), []byte("abc-123"), []byte("X-Custom-Header"), []byte("custom-value")},
	}

	//b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := DecodeRequestFast(treq)
		ReleaseRequest(req)
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
		reqq, _ := EncodeRequestFast(req)
		ReleaseEncodedRequest(reqq)
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
		req, _ := DecodeRequestFast(treq)
		ReleaseRequest(req)
	}
}

func BenchmarkEncodeDecodeRequestRoundTrip(b *testing.B) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod("POST")
	req.URI().SetPath("/api/v1/orders")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	req.Header.Set("X-Request-ID", "req-abc-123-xyz")
	req.Header.Set("X-Trace-ID", "trace-789")
	req.SetBody([]byte(`{"order_id":12345,"items":[{"sku":"ABC-1","qty":2}],"total":59.99}`))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := EncodeRequestFast(req)
		decoded, _ := DecodeRequestFast(encoded)
		ReleaseEncodedRequest(encoded)
		ReleaseRequest(decoded)
	}
}

func BenchmarkEncodeDecodeResponseRoundTrip(b *testing.B) {
	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(200)
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Cache-Control", "no-cache")
	resp.Header.Set("X-Response-Time", "12ms")
	resp.SetBody([]byte(`{"status":"ok","data":{"id":1,"name":"test"}}`))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := EncodeResponseFast(resp)
		decoded, _ := DecodeResponseFast(encoded)
		ReleaseEncodedResponse(encoded)
		ReleaseResponse(decoded)
	}
}

func BenchmarkProtoMarshalTunnelRequest(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_POST,
		Path:          []byte("/api/v1/users"),
		Body:          []byte(`{"name":"bench","payload":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`),
		PackedHeaders: []uint32{1, 18},
		RawHeaders:    [][]byte{[]byte("Authorization"), []byte("Bearer token"), []byte("X-Request-ID"), []byte("bench-123")},
	}

	buf := make([]byte, 0, 1024)
	opts := proto.MarshalOptions{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		opts.MarshalAppend(buf, treq)
	}
}

func BenchmarkProtoUnmarshalTunnelRequest(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_POST,
		Path:          []byte("/api/v1/users"),
		Body:          []byte(`{"name":"bench","payload":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`),
		PackedHeaders: []uint32{1, 18},
		RawHeaders:    [][]byte{[]byte("Authorization"), []byte("Bearer token"), []byte("X-Request-ID"), []byte("bench-123")},
	}

	opts := proto.MarshalOptions{}
	data, _ := opts.Marshal(treq)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var msg gen.TunnelRequest
		proto.Unmarshal(data, &msg)
	}
}

func BenchmarkProtoMarshalTunnelResponse(b *testing.B) {
	tresp := &gen.TunnelResponse{
		CodeResponse:  200,
		Body:          []byte(`{"status":"ok","data":{"id":1,"name":"benchmark payload"}}`),
		PackedHeaders: []uint32{1, 13},
		RawHeaders:    [][]byte{[]byte("X-Response-Time"), []byte("15ms")},
	}

	buf := make([]byte, 0, 1024)
	opts := proto.MarshalOptions{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		opts.MarshalAppend(buf, tresp)
	}
}

func BenchmarkProtoUnmarshalTunnelResponse(b *testing.B) {
	tresp := &gen.TunnelResponse{
		CodeResponse:  200,
		Body:          []byte(`{"status":"ok","data":{"id":1,"name":"benchmark payload"}}`),
		PackedHeaders: []uint32{1, 13},
		RawHeaders:    [][]byte{[]byte("X-Response-Time"), []byte("15ms")},
	}

	opts := proto.MarshalOptions{}
	data, _ := opts.Marshal(tresp)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var msg gen.TunnelResponse
		proto.Unmarshal(data, &msg)
	}
}

func BenchmarkFullRequestPipeline(b *testing.B) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.SetMethod("POST")
	req.URI().SetPath("/api/v1/users")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.SetBody([]byte(`{"name":"test","value":123}`))

	marshalBuf := make([]byte, 0, 2048)
	var decoded gen.TunnelRequest

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := EncodeRequestFast(req)

		marshalBuf = marshalBuf[:0]
		data, _ := encoded.MarshalVT()
		if cap(marshalBuf) < len(data) {
			marshalBuf = make([]byte, len(data))
		}
		marshalBuf = marshalBuf[:len(data)]
		copy(marshalBuf, data)

		decoded.Reset()
		decoded.UnmarshalVT(marshalBuf)

		fastReq, _ := DecodeRequestFast(&decoded)
		fasthttp.ReleaseRequest(fastReq)

		ReleaseEncodedRequest(encoded)
	}
}

func BenchmarkFullResponsePipeline(b *testing.B) {
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)
	resp.SetStatusCode(200)
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Cache-Control", "no-cache")
	resp.SetBody([]byte(`{"success":true,"id":42}`))

	marshalBuf := make([]byte, 0, 2048)
	var decoded gen.TunnelResponse

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := EncodeResponseFast(resp)

		marshalBuf = marshalBuf[:0]
		data, _ := encoded.MarshalVT()
		if cap(marshalBuf) < len(data) {
			marshalBuf = make([]byte, len(data))
		}
		marshalBuf = marshalBuf[:len(data)]
		copy(marshalBuf, data)

		decoded.CodeResponse = 0
		decoded.Body = nil
		decoded.PackedHeaders = decoded.PackedHeaders[:0]
		decoded.RawHeaders = decoded.RawHeaders[:0]
		decoded.UnmarshalVT(marshalBuf)

		fastResp, _ := DecodeResponseFast(&decoded)
		fasthttp.ReleaseResponse(fastResp)

		ReleaseEncodedResponse(encoded)
	}
}

func BenchmarkEncodeRequestFast_LargeBody(b *testing.B) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod("POST")
	req.URI().SetPath("/api/v1/upload")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.SetBody(make([]byte, 65536))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := EncodeRequestFast(req)
		ReleaseEncodedRequest(encoded)
	}
}

func BenchmarkDecodeRequestFast_LargeBody(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_POST,
		Path:          []byte("/api/v1/upload"),
		Body:          make([]byte, 65536),
		PackedHeaders: []uint32{6},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := DecodeRequestFast(treq)
		ReleaseRequest(req)
	}
}

func BenchmarkEncodeResponseFast_LargeBody(b *testing.B) {
	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(200)
	resp.Header.Set("Content-Type", "application/json")
	resp.SetBody(make([]byte, 131072))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := EncodeResponseFast(resp)
		ReleaseEncodedResponse(encoded)
	}
}

func BenchmarkEncodeRequestFast_NoHeaders(b *testing.B) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod("GET")
	req.URI().SetPath("/health")
	req.SetBody(nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, _ := EncodeRequestFast(req)
		ReleaseEncodedRequest(encoded)
	}
}

func BenchmarkBuildHeader(b *testing.B) {
	key := []byte("X-Custom-Header")
	value := []byte("some-custom-value-here")
	builder := getBuilder()
	defer putBuilder(builder)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.Reset()
		buildHeader(key, value, builder)
	}
}

func BenchmarkEncodeRequestFast_Parallel(b *testing.B) {
	req := fasthttp.AcquireRequest()
	req.Header.SetMethod("POST")
	req.URI().SetPath("/api/v1/users")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("X-Request-ID", "abc-123")
	req.SetBody([]byte(`{"name":"test","value":123}`))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			encoded, _ := EncodeRequestFast(req)
			ReleaseEncodedRequest(encoded)
		}
	})
}

func BenchmarkDecodeRequestFast_Parallel(b *testing.B) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_POST,
		Path:          []byte("/api/v1/users"),
		Body:          []byte(`{"name":"test","value":123}`),
		PackedHeaders: []uint32{1, 18},
		RawHeaders:    [][]byte{[]byte("Authorization"), []byte("Bearer token123"), []byte("X-Request-ID"), []byte("abc-123")},
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := DecodeRequestFast(treq)
			ReleaseRequest(req)
		}
	})
}

func BenchmarkFullRequestPipeline_Parallel(b *testing.B) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.SetMethod("POST")
	req.URI().SetPath("/api/v1/users")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer token123")
	req.SetBody([]byte(`{"name":"test","value":123}`))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		marshalBuf := make([]byte, 0, 2048)
		var decoded gen.TunnelRequest

		for pb.Next() {
			encoded, _ := EncodeRequestFast(req)

		marshalBuf = marshalBuf[:0]
		data, _ := encoded.MarshalVT()
		if cap(marshalBuf) < len(data) {
			marshalBuf = make([]byte, len(data))
		}
		marshalBuf = marshalBuf[:len(data)]
		copy(marshalBuf, data)

		decoded.Method = 0
		decoded.Path = nil
		decoded.Body = nil
		decoded.PackedHeaders = decoded.PackedHeaders[:0]
		decoded.RawHeaders = decoded.RawHeaders[:0]
		decoded.UnmarshalVT(marshalBuf)

			fastReq, _ := DecodeRequestFast(&decoded)
			fasthttp.ReleaseRequest(fastReq)

			ReleaseEncodedRequest(encoded)
		}
	})
}
