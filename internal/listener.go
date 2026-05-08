package internal

import (
	"QMesh-Sidecar/internal/gossip"
	"QMesh-Sidecar/internal/protos/pb/gen"
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/valyala/fasthttp"
)

type Listener struct {
	listener    *quic.Listener
	Connections []Connection
	connMutex   sync.RWMutex
	nodesIPs    []string
	nodesMutex  sync.RWMutex
	buffers     *sync.Pool
	trie        *Trie
	gossip      *gossip.Gossip
}

type Connection struct {
	Id     string
	ConnIP string
	Conn   *quic.Conn
}

func NewListener(g *gossip.Gossip) *Listener {
	pool := sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 2048))
		},
	}

	return &Listener{
		Connections: make([]Connection, 0, 24),
		buffers:     &pool,
		trie:        NewTrie(),
		nodesIPs:    make([]string, 0, 24),
		nodesMutex:  sync.RWMutex{},
		gossip:      g,
	}
}

func (l *Listener) StartListen() {
	listener, err := quic.ListenAddr("0.0.0.0:4224", GetStubTlsConf(), nil)
	if err != nil {
		panic(err)
	}
	l.listener = listener

	for {
		conn, err := listener.Accept(context.Background())
		if err != nil {
			continue
		}
		go l.serveConnection(conn)
	}
}

func (l *Listener) serveConnection(conn *quic.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	helloStream, err := conn.AcceptStream(ctx)
	if err != nil {
		conn.CloseWithError(1, "timeout")
		return
	}

	bufPtr := l.buffers.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer l.buffers.Put(bufPtr)

	limitReader := io.LimitReader(helloStream, 1024*10)
	if _, err = io.Copy(bufPtr, limitReader); err != nil && err != io.EOF {
		return
	}

	helloMsg := gen.HelloMessage{}
	if err := helloMsg.UnmarshalVT(bufPtr.Bytes()); err != nil {
		return
	}

	connIP := conn.RemoteAddr().String()
	connection := Connection{Id: UUIDv4(), Conn: conn, ConnIP: connIP}

	l.connMutex.Lock()
	l.Connections = append(l.Connections, connection)
	l.connMutex.Unlock()

	l.trie.AddConnection(helloMsg.Endpoints, &connection)

	if l.gossip != nil {
		l.gossip.OnNodeJoin(connIP)
	}

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			if l.gossip != nil {
				l.gossip.OnNodeLeave(connIP)
			}
			break
		}
		go l.serveStream(stream)
	}
}

func (l *Listener) serveStream(stream *quic.Stream) {
	defer stream.Close()

	bufPtr := l.buffers.Get().(*bytes.Buffer)
	bufPtr.Reset()
	defer l.buffers.Put(bufPtr)

	limitReader := io.LimitReader(stream, 1024*1024)
	if _, err := io.Copy(bufPtr, limitReader); err != nil && err != io.EOF {
		return
	}

	req := gen.TunnelRequest{}
	if err := req.UnmarshalVT(bufPtr.Bytes()); err != nil {
		return
	}

	encodedReq, err := DecodeRequestFast(&req)
	if err != nil {
		return
	}
	defer ReleaseRequest(encodedReq)

	resp := fasthttp.AcquireResponse()
	defer ReleaseResponse(resp)

	if err := SendReq(encodedReq, resp); err != nil {
		return
	}

	response, err := EncodeResponseFast(resp)
	defer ReleaseEncodedResponse(response)
	if err != nil {
		return
	}

	bufPtr.Reset()
	data, err := response.MarshalVT()
	if err != nil {
		return
	}

	stream.Write(data)
}

func (l *Listener) OnNodeJoin(ip string) {
	l.nodesMutex.Lock()
	defer l.nodesMutex.Unlock()
	l.nodesIPs = append(l.nodesIPs, ip)
}

func (l *Listener) OnNodeLeave(ip string) {
	l.nodesMutex.Lock()
	defer l.nodesMutex.Unlock()
	for i, nodeIP := range l.nodesIPs {
		if nodeIP == ip {
			l.nodesIPs = append(l.nodesIPs[:i], l.nodesIPs[i+1:]...)
			break
		}
	}
}

func (l *Listener) GetNodesIPs() []string {
	l.nodesMutex.RLock()
	defer l.nodesMutex.RUnlock()
	return l.nodesIPs
}
