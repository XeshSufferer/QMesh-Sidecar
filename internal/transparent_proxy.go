package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"syscall"

	"github.com/valyala/fasthttp"
)

const (
	SO_ORIGINAL_DST = 80
)

type TransparentProxy struct {
	listener net.Listener
	trie     *Trie
	client   *Client
}

func NewTransparentProxy(trie *Trie) *TransparentProxy {
	return &TransparentProxy{
		trie:   trie,
		client: NewClient(trie),
	}
}

func (tp *TransparentProxy) Start(port string) error {
	addr := "0.0.0.0:" + port

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	tp.listener = listener
	log.Printf("Transparent proxy listening on %s", addr)

	go tp.acceptLoop()
	return nil
}

func (tp *TransparentProxy) acceptLoop() {
	for {
		conn, err := tp.listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			return
		}
		go tp.handleConnection(conn)
	}
}

func (tp *TransparentProxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	originalDst, err := getOriginalDst(conn)
	if err != nil {
		log.Printf("Failed to get original destination: %v", err)
		return
	}

	log.Printf("Original destination: %s", originalDst)

	reader := bufio.NewReader(conn)

	req, resp := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	if err := req.Read(reader); err != nil {
		log.Printf("Failed to parse request: %v", err)
		return
	}

	host, _, _ := net.SplitHostPort(originalDst)
	if host != "" {
		req.Header.SetHost(host)
	}
	req.SetRequestURI("http://" + originalDst + string(req.URI().Path()))
	if len(req.URI().QueryString()) > 0 {
		builder := getBuilder()
		defer putBuilder(builder)

		builder.WriteString(req.URI().String())
		builder.WriteRune('?')
		builder.Write(req.URI().QueryString())

		req.SetRequestURI(builder.String())
	}

	response, err := tp.client.ServeRequest(req)
	if err != nil {
		log.Printf("QMesh request to %s failed: %v", originalDst, err)
		resp.SetStatusCode(fasthttp.StatusBadGateway)
		resp.SetBodyString("Bad Gateway: unable to route request through QMesh")
		w := bufio.NewWriter(conn)
		resp.Write(w)
		w.Flush()
		return
	}
	defer fasthttp.ReleaseResponse(response)

	if _, err := io.Copy(conn, response.BodyStream()); err != nil {
		log.Printf("Write response error: %v", err)
	}
}

func getOriginalDst(conn net.Conn) (string, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return "", fmt.Errorf("not a TCP connection")
	}

	file, err := tcpConn.File()
	if err != nil {
		return "", err
	}
	defer file.Close()

	addr, err := syscall.GetsockoptIPv6Mreq(int(file.Fd()), syscall.IPPROTO_IP, SO_ORIGINAL_DST)
	if err != nil {
		return "", fmt.Errorf("getsockopt SO_ORIGINAL_DST: %w", err)
	}

	ip := net.IPv4(addr.Multiaddr[4], addr.Multiaddr[5], addr.Multiaddr[6], addr.Multiaddr[7])
	port := uint16(addr.Multiaddr[2])<<8 | uint16(addr.Multiaddr[3])

	return fmt.Sprintf("%s:%d", ip.String(), port), nil
}

func (tp *TransparentProxy) Stop() {
	if tp.listener != nil {
		tp.listener.Close()
	}
}
