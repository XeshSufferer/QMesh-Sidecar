package internal

import (
	"QMesh-Sidecar/internal/protos/pb/gen"
	"fmt"
	"strings"

	"github.com/valyala/fasthttp"
)

type header struct {
	Name    string
	Content string
}

var tableSTR2UINT map[string]uint32 = map[string]uint32{
	"Content-Type: application/json": 1,
}

var tableUINT2HEADER map[uint32]header = map[uint32]header{
	1: {"Content-Type", "application/json"},
}

// --- REQUEST ---

func DecodeRequestFast(treq *gen.TunnelRequest) (*fasthttp.Request, error) {
	req := fasthttp.AcquireRequest()

	req.Header.SetMethod(treq.Method.String())
	req.SetRequestURI(fmt.Sprintf("http://localhost:8080/%s", strings.TrimPrefix(string(treq.Path), "/")))
	req.SetBody(treq.Body)

	for _, v := range treq.PackedHeaders {
		if h, ok := tableUINT2HEADER[v]; ok {
			req.Header.Set(h.Name, h.Content)
		}
	}

	for i := 0; i < len(treq.RawHeaders); i += 2 {
		if i+1 < len(treq.RawHeaders) {
			req.Header.Set(treq.RawHeaders[i], treq.RawHeaders[i+1])
		}
	}

	return req, nil
}

func EncodeRequestFast(req *fasthttp.Request) (*gen.TunnelRequest, error) {
	treq := &gen.TunnelRequest{
		Method: gen.HttpMethod(gen.HttpMethod_value[string(req.Header.Method())]),
		Path:   req.URI().Path(),
		Body:   req.Body(),
	}

	req.Header.VisitAll(func(key, value []byte) {
		k, v := string(key), string(value)
		fullHeader := fmt.Sprintf("%s: %s", k, v)

		if id, ok := tableSTR2UINT[fullHeader]; ok {
			treq.PackedHeaders = append(treq.PackedHeaders, id)
		} else {
			treq.RawHeaders = append(treq.RawHeaders, k, v)
		}
	})

	return treq, nil
}

// --- RESPONSE ---

func EncodeResponseFast(resp *fasthttp.Response) (*gen.TunnelResponse, error) {
	tresp := &gen.TunnelResponse{
		CodeResponse: uint32(resp.StatusCode()),
		Body:         resp.Body(),
	}

	resp.Header.VisitAll(func(key, value []byte) {
		k, v := string(key), string(value)
		fullHeader := fmt.Sprintf("%s: %s", k, v)

		if id, ok := tableSTR2UINT[fullHeader]; ok {
			tresp.PackedHeaders = append(tresp.PackedHeaders, id)
		} else {
			tresp.RawHeaders = append(tresp.RawHeaders, k, v)
		}
	})

	return tresp, nil
}

func DecodeResponseFast(tresp *gen.TunnelResponse) (*fasthttp.Response, error) {
	resp := fasthttp.AcquireResponse()

	resp.SetStatusCode(int(tresp.CodeResponse))
	resp.SetBody(tresp.Body)

	for _, v := range tresp.PackedHeaders {
		if h, ok := tableUINT2HEADER[v]; ok {
			resp.Header.Set(h.Name, h.Content)
		}
	}

	for i := 0; i < len(tresp.RawHeaders); i += 2 {
		if i+1 < len(tresp.RawHeaders) {
			resp.Header.Set(tresp.RawHeaders[i], tresp.RawHeaders[i+1])
		}
	}

	return resp, nil
}

// --- UTILS ---

func ReleaseRequest(req *fasthttp.Request) {
	fasthttp.ReleaseRequest(req)
}

func ReleaseResponse(resp *fasthttp.Response) {
	fasthttp.ReleaseResponse(resp)
}
