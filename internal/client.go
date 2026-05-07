package internal

import (
	"QMesh-Sidecar/internal/protos/pb/gen"
	"bytes"
	"context"
	"io"
	"log"
	"math/rand/v2"
	"sync"

	"github.com/valyala/fasthttp"
)

type Client struct {
	trie *Trie
	buff *sync.Pool
}

func NewClient(trie *Trie) *Client {
	return &Client{
		trie: trie,
		buff: &sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, 0, 2048))
			},
		},
	}
}

func (c *Client) ServeRequest(req *fasthttp.Request) (*fasthttp.Response, error) {
	path := ZeroAllocBytesToString(req.URI().Path())
	connections := c.trie.GetConnections(path)

	if len(connections) == 0 {
		return nil, io.EOF
	}

	indices := rand.IntN(len(connections))
	maxRetries := 3
	if indices < maxRetries {
		maxRetries = indices
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		conn := connections[indices]
		resp, err := c.doRequest(conn, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		log.Printf("Retry %d/%d failed for path %s: %v", i+1, maxRetries, path, err)
	}

	return nil, lastErr
}

func (c *Client) doRequest(conn *Connection, req *fasthttp.Request) (*fasthttp.Response, error) {
	stream, err := (*conn.Conn).OpenStreamSync(context.Background())
	if err != nil {
		return nil, err
	}

	encodedReq, err := EncodeRequestFast(req)
	defer ReleaseEncodedRequest(encodedReq)
	if err != nil {
		stream.CancelRead(0)
		stream.Close()
		return nil, err
	}

	bufPtr := c.buff.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer c.buff.Put(bufPtr)

	data, err := encodedReq.MarshalVT()
	if err != nil {
		stream.CancelRead(0)
		stream.Close()
		return nil, err
	}

	if _, err = stream.Write(data); err != nil {
		stream.Close()
		return nil, err
	}

	stream.Close()

	bufPtr.Reset()
	limitReader := io.LimitReader(stream, 1024*1024)
	if _, err := io.Copy(bufPtr, limitReader); err != nil && err != io.EOF {
		return nil, err
	}

	resp := gen.TunnelResponse{}
	if err := resp.UnmarshalVT(bufPtr.Bytes()); err != nil {
		return nil, err
	}

	response, err := DecodeResponseFast(&resp)
	defer ReleaseResponse(response)

	if err != nil {
		ReleaseResponse(response)
		log.Printf("Error by decode response to fasthttp.Response: %v", err)
		return nil, err
	}

	return response, nil
}
