package internal

import (
	"QMesh-Sidecar/internal/protos/pb/gen"
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
	"github.com/valyala/fasthttp"
	"google.golang.org/protobuf/proto"
)

type Listener struct {
	listener    *quic.Listener
	Connections []Connection
	connMutex   sync.RWMutex
	buffers     *sync.Pool
	trie        *Trie
}

type Connection struct {
	Id   string
	Conn *quic.Conn
}

func NewListener() *Listener {
	pool := sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, 2048))
		},
	}

	return &Listener{
		Connections: make([]Connection, 0, 24),
		buffers:     &pool,
		trie:        NewTrie(),
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

	var helloMsg gen.HelloMessage
	if err := proto.Unmarshal(bufPtr.Bytes(), &helloMsg); err != nil {
		return
	}

	connection := Connection{Id: uuid.New().String(), Conn: conn}

	l.connMutex.Lock()
	l.Connections = append(l.Connections, connection)
	l.connMutex.Unlock()

	l.trie.AddConnection(helloMsg.Endpoints, &connection)

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
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

	var req gen.TunnelRequest
	if err := proto.Unmarshal(bufPtr.Bytes(), &req); err != nil {
		return
	}

	encodedReq, err := DecodeRequestFast(&req)
	if err != nil {
		return
	}
	defer ReleaseRequest(encodedReq)

	resp := fasthttp.AcquireResponse()
	defer ReleaseResponse(resp)

	SendReq(encodedReq, resp)

	response, err := EncodeResponseFast(resp)
	if err != nil {
		return
	}

	bufPtr.Reset()
	options := proto.MarshalOptions{}
	data, err := options.MarshalAppend(bufPtr.Bytes(), response)
	if err != nil {
		return
	}

	stream.Write(data)
}
