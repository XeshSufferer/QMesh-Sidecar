package tests

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"QMesh-Sidecar/internal/protos/pb/gen"

	"github.com/quic-go/quic-go"
)

const (
	sidecarHost     = "127.0.0.1"
	sidecarQUICPort = 4224
	backendPort     = 8080
)

var projectRoot string

func init() {
	dir, _ := os.Getwd()
	projectRoot = findProjectRoot(dir)
}

func findProjectRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find project root")
		}
		dir = parent
	}
}

type testResult struct {
	mu      sync.Mutex
	passed  int
	failed  int
	errors  []string
}

func (r *testResult) ok(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.passed++
	fmt.Printf("  ✓ %s\n", name)
}

func (r *testResult) fail(name, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed++
	r.errors = append(r.errors, fmt.Sprintf("%s: %s", name, reason))
	fmt.Printf("  ✗ %s — %s\n", name, reason)
}

func (r *testResult) check(condition bool, name, reason string) {
	if condition {
		r.ok(name)
	} else {
		r.fail(name, reason)
	}
}

type backendHandler struct{}

func (h *backendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	w.Header().Set("Content-Type", "text/plain")

	switch r.Method {
	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("GET %s", r.URL.Path)))
	case http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(fmt.Sprintf("POST %s body=%s", r.URL.Path, string(body))))
	case http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPut:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("PUT %s body=%s", r.URL.Path, string(body))))
	case http.MethodHead:
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("%s %s", r.Method, r.URL.Path)))
	}
}

func startBackend() (*http.Server, error) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", backendPort),
		Handler: &backendHandler{},
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "backend error: %v\n", err)
		}
	}()

	return srv, nil
}

func buildSidecar(t *testing.T) string {
	buildDir := filepath.Join(projectRoot, "build")
	os.MkdirAll(buildDir, 0755)
	binPath := filepath.Join(buildDir, "sidecar")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %s\n%s", err, string(out))
	}
	return binPath
}

func startSidecar(binPath string) (*exec.Cmd, error) {
	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), "GOSSIP_SEEDS=")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, cmd.Start()
}

func waitForSidecar(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:%d", sidecarHost, sidecarQUICPort), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

type tunnelClient struct {
	conn *quic.Conn
}

func newTunnelClient(conn *quic.Conn) *tunnelClient {
	return &tunnelClient{conn: conn}
}

func (c *tunnelClient) sendHello(endpoints []string) error {
	stream, err := c.conn.OpenStream()
	if err != nil {
		return err
	}
	defer stream.Close()

	msg := gen.HelloMessage{Endpoints: endpoints}
	data, err := msg.MarshalVT()
	if err != nil {
		return err
	}

	_, err = stream.Write(data)
	return err
}

func (c *tunnelClient) sendRequest(method gen.HttpMethod, path string, body []byte, rawHeaders [][]byte, timeout time.Duration) (*gen.TunnelResponse, error) {
	stream, err := c.conn.OpenStream()
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	req := gen.TunnelRequest{
		Method:     method,
		Path:       []byte(path),
		Body:       body,
		RawHeaders: rawHeaders,
	}

	data, err := req.MarshalVT()
	if err != nil {
		return nil, err
	}

	if _, err := stream.Write(data); err != nil {
		return nil, err
	}

	if err := stream.Close(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stream.SetReadDeadline(time.Now().Add(timeout))

	respData, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	if len(respData) == 0 {
		_ = ctx
		return nil, fmt.Errorf("no response data for %s", path)
	}

	var resp gen.TunnelResponse
	if err := resp.UnmarshalVT(respData); err != nil {
		return nil, err
	}

	return &resp, nil
}

func connectQUIC() (*tunnelClient, error) {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"qmp"},
	}

	quicConf := &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 5 * time.Second,
	}

	conn, err := quic.DialAddr(
		context.Background(),
		fmt.Sprintf("%s:%d", sidecarHost, sidecarQUICPort),
		tlsConf,
		quicConf,
	)
	if err != nil {
		return nil, err
	}

	return newTunnelClient(conn), nil
}

func testGetRequest(t *testing.T, client *tunnelClient, results *testResult) {
	resp, err := client.sendRequest(gen.HttpMethod_HTTP_METHOD_GET, "/hello", nil, nil, 10*time.Second)
	if err != nil {
		results.fail("GET /hello", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 200, "GET /hello -> status 200", fmt.Sprintf("got %d", resp.GetCodeResponse()))
	results.check(bytes.Contains(resp.GetBody(), []byte("GET /hello")), "GET /hello -> correct body", fmt.Sprintf("body=%s", string(resp.GetBody())))
}

func testPostRequest(t *testing.T, client *tunnelClient, results *testResult) {
	resp, err := client.sendRequest(
		gen.HttpMethod_HTTP_METHOD_POST,
		"/api/users",
		[]byte(`{"name":"e2e-test"}`),
		nil,
		10*time.Second,
	)
	if err != nil {
		results.fail("POST /api/users", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 201, "POST /api/users -> status 201", fmt.Sprintf("got %d", resp.GetCodeResponse()))
	results.check(bytes.Contains(resp.GetBody(), []byte(`{"name":"e2e-test"}`)), "POST -> body echoed", fmt.Sprintf("body=%s", string(resp.GetBody())))
}

func testDeleteRequest(t *testing.T, client *tunnelClient, results *testResult) {
	resp, err := client.sendRequest(gen.HttpMethod_HTTP_METHOD_DELETE, "/api/items/42", nil, nil, 10*time.Second)
	if err != nil {
		results.fail("DELETE /api/items/42", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 204, "DELETE -> status 204", fmt.Sprintf("got %d", resp.GetCodeResponse()))
}

func testPutRequest(t *testing.T, client *tunnelClient, results *testResult) {
	resp, err := client.sendRequest(
		gen.HttpMethod_HTTP_METHOD_PUT,
		"/api/items/1",
		[]byte(`{"price":99}`),
		nil,
		10*time.Second,
	)
	if err != nil {
		results.fail("PUT /api/items/1", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 200, "PUT -> status 200", fmt.Sprintf("got %d", resp.GetCodeResponse()))
}

func testHeadRequest(t *testing.T, client *tunnelClient, results *testResult) {
	resp, err := client.sendRequest(gen.HttpMethod_HTTP_METHOD_HEAD, "/health", nil, nil, 10*time.Second)
	if err != nil {
		results.fail("HEAD /health", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 200, "HEAD /health -> status 200", fmt.Sprintf("got %d", resp.GetCodeResponse()))
}

func testCustomHeaders(t *testing.T, client *tunnelClient, results *testResult) {
	resp, err := client.sendRequest(
		gen.HttpMethod_HTTP_METHOD_GET,
		"/headers-test",
		nil,
		[][]byte{[]byte("X-Test-Header"), []byte("e2e-value"), []byte("X-Request-Id"), []byte("12345")},
		10*time.Second,
	)
	if err != nil {
		results.fail("GET with custom headers", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 200, "GET with custom headers -> status 200", fmt.Sprintf("got %d", resp.GetCodeResponse()))
}

func testLargeBody(t *testing.T, client *tunnelClient, results *testResult) {
	payload := bytes.Repeat([]byte("x"), 10000)
	resp, err := client.sendRequest(
		gen.HttpMethod_HTTP_METHOD_POST,
		"/api/upload",
		payload,
		nil,
		10*time.Second,
	)
	if err != nil {
		results.fail("POST 10KB body", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 201, "POST 10KB body -> status 201", fmt.Sprintf("got %d", resp.GetCodeResponse()))
	results.check(len(resp.GetBody()) > 0, "POST 10KB -> non-empty response", fmt.Sprintf("body length=%d", len(resp.GetBody())))
}

func testManySequential(t *testing.T, client *tunnelClient, results *testResult) {
	errors := 0
	for i := 0; i < 20; i++ {
		resp, err := client.sendRequest(
			gen.HttpMethod_HTTP_METHOD_GET,
			fmt.Sprintf("/api/seq/%d", i),
			nil, nil, 10*time.Second,
		)
		if err != nil || resp.GetCodeResponse() != 200 {
			errors++
		}
	}
	results.check(errors == 0, "20 sequential requests -> all succeeded", fmt.Sprintf("%d errors", errors))
}

func testNestedPath(t *testing.T, client *tunnelClient, results *testResult) {
	resp, err := client.sendRequest(
		gen.HttpMethod_HTTP_METHOD_GET,
		"/api/v2/users/123/posts/456",
		nil, nil, 10*time.Second,
	)
	if err != nil {
		results.fail("GET nested path", err.Error())
		return
	}
	results.check(resp.GetCodeResponse() == 200, "GET nested path -> status 200", fmt.Sprintf("got %d", resp.GetCodeResponse()))
	results.check(bytes.Contains(resp.GetBody(), []byte("/api/v2/users/123/posts/456")),
		"GET nested path -> correct path echoed",
		fmt.Sprintf("body=%s", string(resp.GetBody())))
}

type benchStats struct {
	p50   float64
	p99   float64
	min   float64
	max   float64
	avg   float64
	count int
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Min(float64(len(sorted))*p/100, float64(len(sorted)-1)))
	return sorted[idx]
}

func calcStats(latencies []float64) benchStats {
	if len(latencies) == 0 {
		return benchStats{}
	}
	sorted := make([]float64, len(latencies))
	copy(sorted, latencies)
	sort.Float64s(sorted)

	var sum float64
	for _, l := range latencies {
		sum += l
	}

	return benchStats{
		p50:   percentile(sorted, 50),
		p99:   percentile(sorted, 99),
		min:   sorted[0],
		max:   sorted[len(sorted)-1],
		avg:   sum / float64(len(latencies)),
		count: len(latencies),
	}
}

func benchDirectHTTP(method, path string, body []byte, iterations, concurrency int) []float64 {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", backendPort, path)
	var mu sync.Mutex
	lats := make([]float64, 0, iterations)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			var req *http.Request
			var err error
			if len(body) > 0 {
				req, err = http.NewRequest(method, url, bytes.NewReader(body))
			} else {
				req, err = http.NewRequest(method, url, nil)
			}
			if err != nil {
				return
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				return
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()

			elapsed := time.Since(start).Seconds() * 1000
			mu.Lock()
			lats = append(lats, elapsed)
			mu.Unlock()
		}()
	}

	wg.Wait()
	return lats
}

func benchTunnel(client *tunnelClient, method gen.HttpMethod, path string, body []byte, iterations, concurrency int) []float64 {
	var mu sync.Mutex
	lats := make([]float64, 0, iterations)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			_, err := client.sendRequest(method, path, body, nil, 10*time.Second)
			if err != nil {
				return
			}
			elapsed := time.Since(start).Seconds() * 1000
			mu.Lock()
			lats = append(lats, elapsed)
			mu.Unlock()
		}()
	}

	wg.Wait()
	return lats
}

type benchResult struct {
	concurrency int
	direct      benchStats
	tunnel      benchStats
}

func benchmarkOverhead(t *testing.T, client *tunnelClient) map[string][]benchResult {
	const (
		iterations = 500
		warmup     = 50
	)

	scenarios := []struct {
		label  string
		method gen.HttpMethod
		path   string
		body   []byte
		httpM  string
	}{
		{"GET /hello", gen.HttpMethod_HTTP_METHOD_GET, "/hello", nil, "GET"},
		{"POST /api/users", gen.HttpMethod_HTTP_METHOD_POST, "/api/users", []byte(`{"name":"bench"}`), "POST"},
		{"POST 10KB body", gen.HttpMethod_HTTP_METHOD_POST, "/api/upload", bytes.Repeat([]byte("x"), 10000), "POST"},
	}

	concurrencies := []int{1, 10, 50}
	resultsData := make(map[string][]benchResult)

	for _, sc := range scenarios {
		fmt.Printf("\n  Scenario: %s\n", sc.label)
		fmt.Printf("  %13s %10s %10s %10s %10s %10s %10s %10s\n",
			"Concurrency", "Direct avg", "Direct p99", "Tunnel avg", "Tunnel p99", "Overhead", "Direct rps", "Tunnel rps")
		fmt.Println("  " + strings.Repeat("-", 96))

		var concResults []benchResult

		for _, conc := range concurrencies {
			_ = benchDirectHTTP(sc.httpM, sc.path, sc.body, warmup, conc)
			_ = benchTunnel(client, sc.method, sc.path, sc.body, warmup, conc)

			dLats := benchDirectHTTP(sc.httpM, sc.path, sc.body, iterations, conc)
			tLats := benchTunnel(client, sc.method, sc.path, sc.body, iterations, conc)

			d := calcStats(dLats)
			t := calcStats(tLats)

			overhead := 0.0
			if d.avg > 0 {
				overhead = ((t.avg - d.avg) / d.avg) * 100
			}

			dSum := 0.0
			for _, l := range dLats {
				dSum += l
			}
			tSum := 0.0
			for _, l := range tLats {
				tSum += l
			}

			dRps := float64(d.count) / (dSum / 1000)
			tRps := float64(t.count) / (tSum / 1000)
			if dSum == 0 {
				dRps = 0
			}
			if tSum == 0 {
				tRps = 0
			}

			fmt.Printf("  %13d %8.2fms %8.2fms %8.2fms %8.2fms %+9.1f%% %8.0frps %8.0frps\n",
				conc, d.avg, d.p99, t.avg, t.p99, overhead, dRps, tRps)

			concResults = append(concResults, benchResult{
				concurrency: conc,
				direct:      d,
				tunnel:      t,
			})
		}

		resultsData[sc.label] = concResults
	}

	return resultsData
}

func printOverheadSummary(results map[string][]benchResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("OVERHEAD SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	var allOverheadsPct, allOverheadsMs []float64

	for scenario, concResults := range results {
		fmt.Printf("\n  %s:\n", scenario)
		fmt.Printf("  %13s %10s %10s %11s %11s\n",
			"Concurrency", "Overhead", "Abs ms", "Direct p99", "Tunnel p99")
		fmt.Println("  " + strings.Repeat("-", 60))

		for _, cr := range concResults {
			overheadPct := 0.0
			absMs := 0.0
			if cr.direct.avg > 0 {
				overheadPct = ((cr.tunnel.avg - cr.direct.avg) / cr.direct.avg) * 100
				absMs = cr.tunnel.avg - cr.direct.avg
				allOverheadsPct = append(allOverheadsPct, overheadPct)
				allOverheadsMs = append(allOverheadsMs, absMs)
			}

			fmt.Printf("  %13d %+9.1f%% %+9.2fms %9.2fms %9.2fms\n",
				cr.concurrency, overheadPct, absMs, cr.direct.p99, cr.tunnel.p99)
		}
	}

	if len(allOverheadsPct) > 0 {
		var sumPct, sumMs float64
		for _, v := range allOverheadsPct {
			sumPct += v
		}
		for _, v := range allOverheadsMs {
			sumMs += v
		}
		meanPct := sumPct / float64(len(allOverheadsPct))
		meanMs := sumMs / float64(len(allOverheadsMs))

		sortedPct := make([]float64, len(allOverheadsPct))
		copy(sortedPct, allOverheadsPct)
		sort.Float64s(sortedPct)
		medianPct := sortedPct[len(sortedPct)/2]

		sortedMs := make([]float64, len(allOverheadsMs))
		copy(sortedMs, allOverheadsMs)
		sort.Float64s(sortedMs)
		medianMs := sortedMs[len(sortedMs)/2]

		fmt.Println("\n  Overall:")
		fmt.Printf("    Mean overhead:   +%.1f%%\n", meanPct)
		fmt.Printf("    Median overhead: +%.1f%%\n", medianPct)
		fmt.Printf("    Mean absolute:   +%.2fms\n", meanMs)
		fmt.Printf("    Median absolute: +%.2fms\n", medianMs)
	}
}

func TestE2E(t *testing.T) {
	if os.Getenv("RUN_E2E") == "" {
		t.Skip("skip E2E: set RUN_E2E=1 to run")
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("QMesh-Sidecar E2E Tests")
	fmt.Println(strings.Repeat("=", 60))

	results := &testResult{}
	var sidecarCmd *exec.Cmd
	var backendSrv *http.Server
	var overheadResults map[string][]benchResult

	t.Cleanup(func() {
		fmt.Println("\n[7/7] Cleaning up...")
		if sidecarCmd != nil {
			sidecarCmd.Process.Kill()
			sidecarCmd.Wait()
		}
		if backendSrv != nil {
			backendSrv.Close()
		}
		results.ok("processes terminated")

		fmt.Println("\n" + strings.Repeat("=", 60))
		total := results.passed + results.failed
		fmt.Printf("Results: %d/%d passed", results.passed, total)
		if results.failed > 0 {
			fmt.Printf(", %d FAILED\n", results.failed)
			fmt.Println("\nFailures:")
			for _, err := range results.errors {
				fmt.Printf("  - %s\n", err)
			}
			t.Fail()
		} else {
			fmt.Println(", 0 failed")
		}
		fmt.Println(strings.Repeat("=", 60))

		if overheadResults != nil {
			printOverheadSummary(overheadResults)
		}
	})

	fmt.Println("\n[1/7] Building sidecar...")
	binPath := buildSidecar(t)
	results.ok("sidecar built")

	fmt.Println("\n[2/7] Starting threaded HTTP backend on :8080...")
	var err error
	backendSrv, err = startBackend()
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}
	results.ok(fmt.Sprintf("backend listening on :%d", backendPort))

	fmt.Println("\n[3/7] Starting sidecar...")
	sidecarCmd, err = startSidecar(binPath)
	if err != nil {
		t.Fatalf("failed to start sidecar: %v", err)
	}
	if !waitForSidecar(8 * time.Second) {
		results.fail("sidecar start", "QUIC port not ready after timeout")
		t.Fatal("sidecar QUIC port not ready")
	}
	results.ok("sidecar QUIC port :4224 ready")

	time.Sleep(1 * time.Second)

	fmt.Println("\n[4/7] Connecting via QUIC (ALPN: qmp, TLS insecure)...")
	client, err := connectQUIC()
	if err != nil {
		results.fail("QUIC connection", err.Error())
		t.Fatalf("failed to connect: %v", err)
	}

	err = client.sendHello([]string{"/hello", "/api", "/headers-test", "/health", "/ping"})
	if err != nil {
		results.fail("HelloMessage", err.Error())
	} else {
		results.ok("HelloMessage sent with endpoints")
	}

	time.Sleep(500 * time.Millisecond)

	fmt.Println("\n[5/7] Running QUIC tunnel tests...")

	resp, err := client.sendRequest(gen.HttpMethod_HTTP_METHOD_GET, "/ping", nil, nil, 5*time.Second)
	if err != nil {
		results.fail("ping", err.Error())
	} else {
		results.ok(fmt.Sprintf("ping: status=%d", resp.GetCodeResponse()))
	}

	testGetRequest(t, client, results)
	testPostRequest(t, client, results)
	testDeleteRequest(t, client, results)
	testPutRequest(t, client, results)
	testHeadRequest(t, client, results)
	testCustomHeaders(t, client, results)
	testLargeBody(t, client, results)
	testManySequential(t, client, results)
	testNestedPath(t, client, results)

	fmt.Println("\n[6/7] Measuring proxy overhead (500 iter x 3 scenarios x 3 concurrencies)...")
	overheadResults = benchmarkOverhead(t, client)
}

func TestBenchmarkOnly(t *testing.T) {
	if os.Getenv("RUN_E2E") == "" {
		t.Skip("skip E2E: set RUN_E2E=1 to run")
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("QMesh-Sidecar Proxy Overhead Benchmark")
	fmt.Println(strings.Repeat("=", 60))

	results := &testResult{}
	var sidecarCmd *exec.Cmd
	var backendSrv *http.Server
	var overheadResults map[string][]benchResult

	t.Cleanup(func() {
		if sidecarCmd != nil {
			sidecarCmd.Process.Kill()
			sidecarCmd.Wait()
		}
		if backendSrv != nil {
			backendSrv.Close()
		}

		if overheadResults != nil {
			printOverheadSummary(overheadResults)
		}
	})

	binPath := buildSidecar(t)

	var err error
	backendSrv, err = startBackend()
	if err != nil {
		t.Fatalf("failed to start backend: %v", err)
	}

	sidecarCmd, err = startSidecar(binPath)
	if err != nil {
		t.Fatalf("failed to start sidecar: %v", err)
	}
	if !waitForSidecar(8 * time.Second) {
		t.Fatal("sidecar QUIC port not ready")
	}

	time.Sleep(1 * time.Second)

	client, err := connectQUIC()
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	_ = client.sendHello([]string{"/hello", "/api", "/headers-test", "/health", "/ping"})
	time.Sleep(500 * time.Millisecond)

	overheadResults = benchmarkOverhead(t, client)
	_ = results
}


