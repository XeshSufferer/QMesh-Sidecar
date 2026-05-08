package internal

import (
	"bytes"
	"testing"

	"QMesh-Sidecar/internal/protos/pb/gen"

	"github.com/valyala/fasthttp"
)

func TestEncodeRequestFast_Basic(t *testing.T) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethodBytes([]byte("GET"))
	req.URI().SetPath("/api/v1/users")

	encoded, err := EncodeRequestFast(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseEncodedRequest(encoded)

	if encoded.Method != gen.HttpMethod_HTTP_METHOD_GET {
		t.Errorf("method = %v, want %v", encoded.Method, gen.HttpMethod_HTTP_METHOD_GET)
	}

	if !bytes.Equal(encoded.Path, []byte("/api/v1/users")) {
		t.Errorf("path = %q, want %q", encoded.Path, "/api/v1/users")
	}
}

func TestEncodeRequestFast_AllMethods(t *testing.T) {
	methods := []struct {
		name   string
		method []byte
		want   gen.HttpMethod
	}{
		{"GET", []byte("GET"), gen.HttpMethod_HTTP_METHOD_GET},
		{"POST", []byte("POST"), gen.HttpMethod_HTTP_METHOD_POST},
		{"PUT", []byte("PUT"), gen.HttpMethod_HTTP_METHOD_PUT},
		{"PATCH", []byte("PATCH"), gen.HttpMethod_HTTP_METHOD_PATCH},
		{"DELETE", []byte("DELETE"), gen.HttpMethod_HTTP_METHOD_DELETE},
		{"HEAD", []byte("HEAD"), gen.HttpMethod_HTTP_METHOD_HEAD},
		{"OPTIONS", []byte("OPTIONS"), gen.HttpMethod_HTTP_METHOD_OPTIONS},
	}

	for _, tt := range methods {
		t.Run(tt.name, func(t *testing.T) {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)

			req.Header.SetMethodBytes(tt.method)
			req.URI().SetPath("/test")

			encoded, err := EncodeRequestFast(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer ReleaseEncodedRequest(encoded)

			if encoded.Method != tt.want {
				t.Errorf("method = %v, want %v", encoded.Method, tt.want)
			}
		})
	}
}

func TestEncodeRequestFast_WithBody(t *testing.T) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod("POST")
	req.URI().SetPath("/api/users")
	req.SetBody([]byte(`{"name":"test"}`))

	encoded, err := EncodeRequestFast(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseEncodedRequest(encoded)

	if !bytes.Equal(encoded.Body, []byte(`{"name":"test"}`)) {
		t.Errorf("body = %q, want %q", encoded.Body, `{"name":"test"}`)
	}
}

func TestEncodeRequestFast_PackedHeaders(t *testing.T) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod("GET")
	req.URI().SetPath("/test")
	req.Header.Set("Content-Type", "application/json")

	encoded, err := EncodeRequestFast(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseEncodedRequest(encoded)

	if len(encoded.PackedHeaders) == 0 {
		t.Error("expected packed headers to contain Content-Type")
	}
}

func TestEncodeRequestFast_RawHeaders(t *testing.T) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.Header.SetMethod("GET")
	req.URI().SetPath("/test")
	req.Header.Set("X-Custom-Header", "custom-value")

	encoded, err := EncodeRequestFast(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseEncodedRequest(encoded)

	if len(encoded.RawHeaders) < 2 {
		t.Error("expected raw headers to contain X-Custom-Header")
	}
}

func TestDecodeRequestFast_Basic(t *testing.T) {
	treq := &gen.TunnelRequest{
		Method: gen.HttpMethod_HTTP_METHOD_GET,
		Path:   []byte("/api/v1/users"),
	}

	req, err := DecodeRequestFast(treq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseRequest(req)

	if string(req.Header.Method()) != "GET" {
		t.Errorf("method = %q, want %q", req.Header.Method(), "GET")
	}

	if string(req.URI().Path()) != "/api/v1/users" {
		t.Errorf("path = %q, want %q", req.URI().Path(), "/api/v1/users")
	}
}

func TestDecodeRequestFast_WithBody(t *testing.T) {
	treq := &gen.TunnelRequest{
		Method: gen.HttpMethod_HTTP_METHOD_POST,
		Path:   []byte("/api/users"),
		Body:   []byte(`{"name":"test"}`),
	}

	req, err := DecodeRequestFast(treq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseRequest(req)

	if !bytes.Equal(req.Body(), []byte(`{"name":"test"}`)) {
		t.Errorf("body = %q, want %q", req.Body(), `{"name":"test"}`)
	}
}

func TestDecodeRequestFast_PackedHeaders(t *testing.T) {
	treq := &gen.TunnelRequest{
		Method:        gen.HttpMethod_HTTP_METHOD_GET,
		Path:          []byte("/test"),
		PackedHeaders: []uint32{1},
	}

	req, err := DecodeRequestFast(treq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseRequest(req)

	ct := req.Header.Peek("Content-Type")
	if string(ct) != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestDecodeRequestFast_RawHeaders(t *testing.T) {
	treq := &gen.TunnelRequest{
		Method:     gen.HttpMethod_HTTP_METHOD_GET,
		Path:       []byte("/test"),
		RawHeaders: [][]byte{[]byte("X-Custom"), []byte("value")},
	}

	req, err := DecodeRequestFast(treq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseRequest(req)

	if string(req.Header.Peek("X-Custom")) != "value" {
		t.Errorf("X-Custom = %q, want %q", req.Header.Peek("X-Custom"), "value")
	}
}

func TestEncodeResponseFast_Basic(t *testing.T) {
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	resp.SetStatusCode(200)
	resp.SetBody([]byte(`{"ok":true}`))

	encoded, err := EncodeResponseFast(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseEncodedResponse(encoded)

	if encoded.CodeResponse != 200 {
		t.Errorf("status code = %d, want 200", encoded.CodeResponse)
	}

	if !bytes.Equal(encoded.Body, []byte(`{"ok":true}`)) {
		t.Errorf("body = %q, want %q", encoded.Body, `{"ok":true}`)
	}
}

func TestEncodeResponseFast_WithHeaders(t *testing.T) {
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	resp.SetStatusCode(200)
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("X-Custom", "value")

	encoded, err := EncodeResponseFast(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseEncodedResponse(encoded)

	if len(encoded.PackedHeaders) == 0 && len(encoded.RawHeaders) == 0 {
		t.Error("expected some headers to be encoded")
	}
}

func TestDecodeResponseFast_Basic(t *testing.T) {
	tresp := &gen.TunnelResponse{
		CodeResponse: 200,
		Body:         []byte(`{"ok":true}`),
	}

	resp, err := DecodeResponseFast(tresp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseResponse(resp)

	if resp.StatusCode() != 200 {
		t.Errorf("status code = %d, want 200", resp.StatusCode())
	}

	if !bytes.Equal(resp.Body(), []byte(`{"ok":true}`)) {
		t.Errorf("body = %q, want %q", resp.Body(), `{"ok":true}`)
	}
}

func TestDecodeResponseFast_WithHeaders(t *testing.T) {
	tresp := &gen.TunnelResponse{
		CodeResponse:  200,
		Body:          []byte(`{"ok":true}`),
		PackedHeaders: []uint32{1},
		RawHeaders:    [][]byte{[]byte("X-Custom"), []byte("value")},
	}

	resp, err := DecodeResponseFast(tresp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer ReleaseResponse(resp)

	ct := resp.Header.Peek("Content-Type")
	if string(ct) != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	if string(resp.Header.Peek("X-Custom")) != "value" {
		t.Errorf("X-Custom = %q, want %q", resp.Header.Peek("X-Custom"), "value")
	}
}

func TestRequestRoundTrip(t *testing.T) {
	original := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(original)

	original.Header.SetMethodBytes([]byte("POST"))
	original.URI().SetPath("/api/v1/users")
	original.Header.Set("Content-Type", "application/json")
	original.Header.Set("X-Request-ID", "123")
	original.SetBody([]byte(`{"name":"test"}`))

	encoded, err := EncodeRequestFast(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	defer ReleaseEncodedRequest(encoded)

	decoded, err := DecodeRequestFast(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	defer ReleaseRequest(decoded)

	if string(decoded.Header.Method()) != "POST" {
		t.Errorf("method mismatch: got %q, want %q", decoded.Header.Method(), "POST")
	}

	if string(decoded.URI().Path()) != string(original.URI().Path()) {
		t.Errorf("path mismatch: got %q, want %q", decoded.URI().Path(), original.URI().Path())
	}

	if !bytes.Equal(decoded.Body(), original.Body()) {
		t.Errorf("body mismatch: got %q, want %q", decoded.Body(), original.Body())
	}
}

func TestResponseRoundTrip(t *testing.T) {
	original := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(original)

	original.SetStatusCode(201)
	original.Header.Set("Content-Type", "application/json")
	original.Header.Set("X-Trace-ID", "trace-123")
	original.SetBody([]byte(`{"id":1}`))

	encoded, err := EncodeResponseFast(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	defer ReleaseEncodedResponse(encoded)

	decoded, err := DecodeResponseFast(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	defer ReleaseResponse(decoded)

	if decoded.StatusCode() != original.StatusCode() {
		t.Errorf("status mismatch: got %d, want %d", decoded.StatusCode(), original.StatusCode())
	}

	if !bytes.Equal(decoded.Body(), original.Body()) {
		t.Errorf("body mismatch: got %q, want %q", decoded.Body(), original.Body())
	}
}

func TestPoolRequestReuse(t *testing.T) {
	req1 := getRequest()
	if req1 == nil {
		t.Fatal("expected non-nil request from pool")
	}
	putRequest(req1)

	req2 := getRequest()
	if req2 == nil {
		t.Fatal("expected non-nil request from pool after put")
	}

	if req2.Method != 0 || req2.Path != nil || req2.Body != nil {
		t.Error("expected reset request from pool")
	}
}

func TestPoolResponseReuse(t *testing.T) {
	resp1 := getResponse()
	if resp1 == nil {
		t.Fatal("expected non-nil response from pool")
	}
	putResponse(resp1)

	resp2 := getResponse()
	if resp2 == nil {
		t.Fatal("expected non-nil response from pool after put")
	}

	if resp2.CodeResponse != 0 || resp2.Body != nil {
		t.Error("expected reset response from pool")
	}
}

func TestBuildHeader(t *testing.T) {
	builder := getBuilder()
	defer putBuilder(builder)

	key := []byte("X-Custom")
	value := []byte("value")

	buildHeader(key, value, builder)

	expected := "X-Custom: value"
	if builder.String() != expected {
		t.Errorf("buildHeader = %q, want %q", builder.String(), expected)
	}
}
