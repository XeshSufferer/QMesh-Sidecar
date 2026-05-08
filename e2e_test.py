#!/usr/bin/env python3
"""
E2E tests for QMesh-Sidecar.

Tests the full QUIC tunnel flow:
1. Starts threaded HTTP backend on :8080
2. Builds and starts sidecar binary (QUIC :4224)
3. Connects via QUIC client (aioquic) with self-signed TLS
4. Sends HelloMessage to register endpoints
5. Sends TunnelRequest via QUIC streams, verifies TunnelResponse
6. Measures proxy overhead vs direct HTTP (aiohttp, 500 iter, concurrency 1/10/50)

Requirements:
    python3 -m venv /tmp/qmesh-e2e
    /tmp/qmesh-e2e/bin/pip install aioquic aiohttp protobuf

Run:
    python3 e2e_test.py
"""

import asyncio
import os
import socket
import ssl
import statistics
import subprocess
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from threading import Thread

import aiohttp

from aioquic.asyncio import connect
from aioquic.asyncio.protocol import QuicConnectionProtocol
from aioquic.quic.configuration import QuicConfiguration
from aioquic.quic.events import QuicEvent, StreamDataReceived

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------

SIDECAR_HOST = "127.0.0.1"
SIDECAR_QUIC_PORT = 4224
BACKEND_PORT = 8080
PROJECT_ROOT = Path(__file__).parent
SIDECAR_BIN = PROJECT_ROOT / "build" / "sidecar"

# ---------------------------------------------------------------------------
# Minimal protobuf encoder / decoder
# ---------------------------------------------------------------------------

WIRE_VARINT = 0
WIRE_BYTES = 2

HTTP_METHOD_GET = 1
HTTP_METHOD_POST = 2
HTTP_METHOD_PUT = 3
HTTP_METHOD_DELETE = 5
HTTP_METHOD_HEAD = 6


def _varint(value: int) -> bytes:
    buf = bytearray()
    while value > 0x7F:
        buf.append((value & 0x7F) | 0x80)
        value >>= 7
    buf.append(value & 0x7F)
    return bytes(buf)


def _tag(field: int, wire: int) -> bytes:
    return _varint((field << 3) | wire)


def _bytes_field(field: int, value: bytes) -> bytes:
    return _tag(field, WIRE_BYTES) + _varint(len(value)) + value


def _varint_field(field: int, value: int) -> bytes:
    return _tag(field, WIRE_VARINT) + _varint(value)


def _string_field(field: int, value: str) -> bytes:
    return _bytes_field(field, value.encode("utf-8"))


def encode_tunnel_request(
    method: int,
    path: str,
    body: bytes = b"",
    raw_headers: list[tuple[bytes, bytes]] | None = None,
) -> bytes:
    buf = b""
    buf += _varint_field(1, method)
    buf += _bytes_field(2, path.encode("utf-8"))
    if body:
        buf += _bytes_field(3, body)
    if raw_headers:
        for k, v in raw_headers:
            buf += _bytes_field(5, k)
            buf += _bytes_field(5, v)
    return buf


def encode_hello_message(endpoints: list[str], key: bytes = b"") -> bytes:
    buf = b""
    for ep in endpoints:
        buf += _string_field(1, ep)
    if key:
        buf += _bytes_field(2, key)
    return buf


def decode_tunnel_response(data: bytes) -> dict:
    result: dict = {
        "code_response": 0,
        "body": b"",
        "packed_headers": [],
        "raw_headers": [],
    }
    pos = 0
    raw_header_buf: list[bytes] = []
    while pos < len(data):
        tag_byte = data[pos]
        field_num = tag_byte >> 3
        wire_type = tag_byte & 0x07
        pos += 1

        if wire_type == WIRE_VARINT:
            value = 0
            shift = 0
            while True:
                b = data[pos]
                pos += 1
                value |= (b & 0x7F) << shift
                if not (b & 0x80):
                    break
                shift += 7
            if field_num == 1:
                result["code_response"] = value
            elif field_num == 4:
                result["packed_headers"].append(value)

        elif wire_type == WIRE_BYTES:
            length = 0
            shift = 0
            while True:
                b = data[pos]
                pos += 1
                length |= (b & 0x7F) << shift
                if not (b & 0x80):
                    break
                shift += 7
            value = data[pos : pos + length]
            pos += length
            if field_num == 2:
                result["body"] = value
            elif field_num == 5:
                raw_header_buf.append(value)

    i = 0
    while i + 1 < len(raw_header_buf):
        result["raw_headers"].append((raw_header_buf[i], raw_header_buf[i + 1]))
        i += 2

    return result


# ---------------------------------------------------------------------------
# QUIC tunnel client protocol
# ---------------------------------------------------------------------------

class TunnelClientProtocol(QuicConnectionProtocol):

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._pending: dict[int, bytearray] = {}
        self._stream_done: dict[int, asyncio.Event] = {}

    def quic_event_received(self, event: QuicEvent) -> None:
        if isinstance(event, StreamDataReceived):
            sid = event.stream_id
            if sid not in self._pending:
                self._pending[sid] = bytearray()
            self._pending[sid].extend(event.data)
            if event.end_stream:
                if sid in self._stream_done:
                    self._stream_done[sid].set()

    async def _create_tracked_stream(self) -> int:
        stream_id = self._quic.get_next_available_stream_id(is_unidirectional=False)
        self._pending[stream_id] = bytearray()
        self._stream_done[stream_id] = asyncio.Event()
        return stream_id

    async def _send_on_stream(self, stream_id: int, data: bytes) -> None:
        self._quic.send_stream_data(stream_id, data, end_stream=True)
        self.transmit()

    async def _read_stream(self, stream_id: int, timeout: float = 10.0) -> bytes:
        try:
            await asyncio.wait_for(
                self._stream_done[stream_id].wait(),
                timeout=timeout,
            )
            return bytes(self._pending.pop(stream_id, b""))
        except asyncio.TimeoutError:
            self._pending.pop(stream_id, None)
            raise TimeoutError(f"Stream timed out after {timeout}s")

    async def send_hello(self, endpoints: list[str]) -> None:
        sid = await self._create_tracked_stream()
        msg = encode_hello_message(endpoints)
        await self._send_on_stream(sid, msg)
        await asyncio.sleep(0.2)

    async def send_tunnel_request(
        self,
        method: int,
        path: str,
        body: bytes = b"",
        raw_headers: list[tuple[bytes, bytes]] | None = None,
        timeout: float = 10.0,
    ) -> dict:
        sid = await self._create_tracked_stream()
        msg = encode_tunnel_request(method, path, body, raw_headers=raw_headers)
        await self._send_on_stream(sid, msg)
        data = await self._read_stream(sid, timeout=timeout)
        if not data:
            raise TimeoutError(f"No response data for {path}")
        return decode_tunnel_response(data)


# ---------------------------------------------------------------------------
# Threaded HTTP backend
# ---------------------------------------------------------------------------

class BackendHandler(BaseHTTPRequestHandler):

    def log_message(self, format, *args):
        pass

    def do_GET(self):
        self._respond(200, f"GET {self.path}")

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length) if length else b""
        self._respond(201, f"POST {self.path} body={body.decode()}")

    def do_DELETE(self):
        self._respond(204, f"DELETE {self.path}")

    def do_PUT(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length) if length else b""
        self._respond(200, f"PUT {self.path} body={body.decode()}")

    def do_HEAD(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()

    def _respond(self, code: int, body: str):
        self.send_response(code)
        self.send_header("Content-Type", "text/plain")
        self.end_headers()
        self.wfile.write(body.encode())


class ThreadedHTTPServer(HTTPServer):
    """Handle each request in a separate thread."""
    allow_reuse_address = True

    def process_request(self, request, client_address):
        t = Thread(target=self._process, args=(request, client_address), daemon=True)
        t.start()

    def _process(self, request, client_address):
        try:
            self.finish_request(request, client_address)
        except Exception:
            self.handle_error(request, client_address)
        finally:
            self.shutdown_request(request)


def start_backend() -> ThreadedHTTPServer:
    server = ThreadedHTTPServer(("127.0.0.1", BACKEND_PORT), BackendHandler)
    t = Thread(target=server.serve_forever, daemon=True)
    t.start()
    return server


# ---------------------------------------------------------------------------
# Sidecar process management
# ---------------------------------------------------------------------------

def build_sidecar() -> bool:
    build_dir = PROJECT_ROOT / "build"
    build_dir.mkdir(exist_ok=True)
    result = subprocess.run(
        ["go", "build", "-o", str(SIDECAR_BIN), "./cmd/"],
        cwd=PROJECT_ROOT,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        print(f"  Build failed:\n{result.stderr}")
        return False
    return True


def start_sidecar() -> subprocess.Popen:
    env = os.environ.copy()
    env["GOSSIP_SEEDS"] = ""
    proc = subprocess.Popen(
        [str(SIDECAR_BIN)],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return proc


def wait_for_sidecar(timeout: float = 8.0) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
            sock.settimeout(0.5)
            sock.connect((SIDECAR_HOST, SIDECAR_QUIC_PORT))
            sock.close()
            return True
        except (ConnectionRefusedError, OSError):
            time.sleep(0.3)
    return False


# ---------------------------------------------------------------------------
# Test runner
# ---------------------------------------------------------------------------

class TestResult:
    def __init__(self):
        self.passed = 0
        self.failed = 0
        self.errors: list[str] = []

    def ok(self, name: str):
        self.passed += 1
        print(f"  \u2713 {name}")

    def fail(self, name: str, reason: str = ""):
        self.failed += 1
        self.errors.append(f"{name}: {reason}")
        print(f"  \u2717 {name} \u2014 {reason}")


results = TestResult()


def check(condition: bool, name: str, reason: str = ""):
    if condition:
        results.ok(name)
    else:
        results.fail(name, reason or f"expected truthy, got {condition}")


# ---------------------------------------------------------------------------
# Test cases
# ---------------------------------------------------------------------------

async def test_get_request(client: TunnelClientProtocol):
    resp = await client.send_tunnel_request(HTTP_METHOD_GET, "/hello")
    check(resp["code_response"] == 200, "GET /hello -> status 200",
          f"got {resp['code_response']}")
    check(b"GET /hello" in resp["body"], "GET /hello -> correct body",
          f"body={resp['body']}")


async def test_post_request(client: TunnelClientProtocol):
    resp = await client.send_tunnel_request(
        HTTP_METHOD_POST, "/api/users",
        body=b'{"name":"e2e-test"}',
    )
    check(resp["code_response"] == 201, "POST /api/users -> status 201",
          f"got {resp['code_response']}")
    check(b'{"name":"e2e-test"}' in resp["body"], "POST -> body echoed",
          f"body={resp['body']}")


async def test_delete_request(client: TunnelClientProtocol):
    resp = await client.send_tunnel_request(HTTP_METHOD_DELETE, "/api/items/42")
    check(resp["code_response"] == 204, "DELETE -> status 204",
          f"got {resp['code_response']}")


async def test_put_request(client: TunnelClientProtocol):
    resp = await client.send_tunnel_request(
        HTTP_METHOD_PUT, "/api/items/1",
        body=b'{"price":99}',
    )
    check(resp["code_response"] == 200, "PUT -> status 200",
          f"got {resp['code_response']}")


async def test_head_request(client: TunnelClientProtocol):
    resp = await client.send_tunnel_request(HTTP_METHOD_HEAD, "/health")
    check(resp["code_response"] == 200, "HEAD /health -> status 200",
          f"got {resp['code_response']}")


async def test_custom_headers(client: TunnelClientProtocol):
    resp = await client.send_tunnel_request(
        HTTP_METHOD_GET, "/headers-test",
        raw_headers=[(b"X-Test-Header", b"e2e-value"), (b"X-Request-Id", b"12345")],
    )
    check(resp["code_response"] == 200, "GET with custom headers -> status 200",
          f"got {resp['code_response']}")


async def test_large_body(client: TunnelClientProtocol):
    payload = b"x" * 10000
    resp = await client.send_tunnel_request(
        HTTP_METHOD_POST, "/api/upload",
        body=payload,
    )
    check(resp["code_response"] == 201, "POST 10KB body -> status 201",
          f"got {resp['code_response']}")
    check(len(resp["body"]) > 0, "POST 10KB -> non-empty response",
          f"body length={len(resp['body'])}")


async def test_many_sequential_requests(client: TunnelClientProtocol):
    errors = 0
    for i in range(20):
        try:
            resp = await client.send_tunnel_request(
                HTTP_METHOD_GET, f"/api/seq/{i}"
            )
            if resp["code_response"] != 200:
                errors += 1
        except Exception:
            errors += 1
    check(errors == 0, "20 sequential requests -> all succeeded",
          f"{errors} errors")


async def test_nested_path(client: TunnelClientProtocol):
    resp = await client.send_tunnel_request(
        HTTP_METHOD_GET, "/api/v2/users/123/posts/456"
    )
    check(resp["code_response"] == 200, "GET nested path -> status 200",
          f"got {resp['code_response']}")
    check(b"/api/v2/users/123/posts/456" in resp["body"],
          "GET nested path -> correct path echoed",
          f"body={resp['body']}")


# ---------------------------------------------------------------------------
# Proxy overhead benchmark
# ---------------------------------------------------------------------------

ITERATIONS = 500
WARMUP = 50
CONCURRENCIES = [1, 10, 50]

BENCH_SCENARIOS = [
    ("GET /hello",      HTTP_METHOD_GET,  "/hello",      b""),
    ("POST /api/users", HTTP_METHOD_POST, "/api/users",  b'{"name":"bench"}'),
    ("POST 10KB body",  HTTP_METHOD_POST, "/api/upload", b"x" * 10000),
]


def _percentile(sorted_lats: list[float], p: float) -> float:
    if not sorted_lats:
        return 0
    idx = min(int(len(sorted_lats) * p / 100), len(sorted_lats) - 1)
    return sorted_lats[idx]


def _stats(latencies: list[float]) -> dict:
    if not latencies:
        return {"p50": 0, "p99": 0, "min": 0, "max": 0, "avg": 0, "count": 0}
    s = sorted(latencies)
    return {
        "p50": _percentile(s, 50),
        "p99": _percentile(s, 99),
        "min": s[0],
        "max": s[-1],
        "avg": statistics.mean(latencies),
        "count": len(latencies),
    }


async def _bench_direct(session: aiohttp.ClientSession,
                         method: str, path: str, body: bytes,
                         iterations: int, concurrency: int) -> list[float]:
    """Benchmark direct HTTP with aiohttp, concurrent."""
    url = f"http://127.0.0.1:{BACKEND_PORT}{path}"
    lats: list[float] = []
    sem = asyncio.Semaphore(concurrency)

    async def _one():
        async with sem:
            start = time.perf_counter()
            if body:
                resp = await session.request(method, url, data=body)
            else:
                resp = await session.request(method, url)
            await resp.read()
            lats.append((time.perf_counter() - start) * 1000)

    tasks = [asyncio.create_task(_one()) for _ in range(iterations)]
    await asyncio.gather(*tasks, return_exceptions=True)
    return lats


async def _bench_tunnel(client: TunnelClientProtocol,
                         method: int, path: str, body: bytes,
                         iterations: int, concurrency: int) -> list[float]:
    """Benchmark QUIC tunnel, concurrent."""
    lats: list[float] = []
    sem = asyncio.Semaphore(concurrency)

    async def _one():
        async with sem:
            start = time.perf_counter()
            await client.send_tunnel_request(method, path, body, timeout=10.0)
            lats.append((time.perf_counter() - start) * 1000)

    tasks = [asyncio.create_task(_one()) for _ in range(iterations)]
    await asyncio.gather(*tasks, return_exceptions=True)
    return lats


async def benchmark_overhead(client: TunnelClientProtocol) -> dict:
    """
    Measure proxy overhead: direct HTTP (aiohttp) vs QUIC tunnel.

    For each scenario and concurrency:
    1. Warmup (50 requests)
    2. Measure (500 requests)
    3. Report avg, p50, p99, throughput, overhead
    """
    results_data: dict[str, list[tuple[int, dict, dict]]] = {}

    for label, method, path, body in BENCH_SCENARIOS:
        print(f"\n  Scenario: {label}")
        print(f"  {'Concurrency':>13} {'Direct avg':>10} {'Direct p99':>10} "
              f"{'Tunnel avg':>10} {'Tunnel p99':>10} "
              f"{'Overhead':>10} {'Direct rps':>10} {'Tunnel rps':>10}")
        print("  " + "-" * 96)

        concurrency_results = []

        async with aiohttp.ClientSession() as session:
            for conc in CONCURRENCIES:
                http_method = "GET" if method == HTTP_METHOD_GET else "POST"

                # Warmup — direct
                d_warmup = await _bench_direct(session, http_method, path, body, WARMUP, conc)
                # Warmup — tunnel
                t_warmup = await _bench_tunnel(client, method, path, body, WARMUP, conc)

                # Measure
                d_lats = await _bench_direct(session, http_method, path, body, ITERATIONS, conc)
                t_lats = await _bench_tunnel(client, method, path, body, ITERATIONS, conc)

                d = _stats(d_lats)
                t = _stats(t_lats)

                overhead = 0.0
                if d["avg"] > 0:
                    overhead = ((t["avg"] - d["avg"]) / d["avg"]) * 100

                d_rps = d["count"] / (sum(d_lats) / 1000) if sum(d_lats) > 0 else 0
                t_rps = t["count"] / (sum(t_lats) / 1000) if sum(t_lats) > 0 else 0

                print(f"  {conc:>13} {d['avg']:>8.2f}ms {d['p99']:>8.2f}ms "
                      f"{t['avg']:>8.2f}ms {t['p99']:>8.2f}ms "
                      f"{overhead:>+9.1f}% {d_rps:>8.0f}rps {t_rps:>8.0f}rps")

                concurrency_results.append((conc, d, t))

        results_data[label] = concurrency_results

    return results_data


def print_overhead_summary(results: dict) -> None:
    """Print consolidated overhead summary."""
    print("\n" + "=" * 60)
    print("OVERHEAD SUMMARY")
    print("=" * 60)

    all_overheads_pct = []
    all_overheads_ms = []

    for scenario, concurrency_results in results.items():
        print(f"\n  {scenario}:")
        print(f"  {'Concurrency':>13} {'Overhead':>10} {'Abs ms':>10} "
              f"{'Direct p99':>11} {'Tunnel p99':>11}")
        print("  " + "-" * 60)

        for conc, d, t in concurrency_results:
            overhead_pct = 0.0
            abs_ms = 0.0
            if d["avg"] > 0:
                overhead_pct = ((t["avg"] - d["avg"]) / d["avg"]) * 100
                abs_ms = t["avg"] - d["avg"]
                all_overheads_pct.append(overhead_pct)
                all_overheads_ms.append(abs_ms)

            print(f"  {conc:>13} {overhead_pct:>+9.1f}% {abs_ms:>+9.2f}ms "
                  f"{d['p99']:>9.2f}ms {t['p99']:>9.2f}ms")

    if all_overheads_pct:
        print(f"\n  Overall:")
        print(f"    Mean overhead:   +{statistics.mean(all_overheads_pct):.1f}%")
        print(f"    Median overhead: +{statistics.median(all_overheads_pct):.1f}%")
        print(f"    Mean absolute:   +{statistics.mean(all_overheads_ms):.2f}ms")
        print(f"    Median absolute: +{statistics.median(all_overheads_ms):.2f}ms")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

async def run_tests():
    print("=" * 60)
    print("QMesh-Sidecar E2E Tests")
    print("=" * 60)

    sidecar_stderr = b""
    sidecar = None
    backend = None
    overhead_results: dict = {}

    try:
        # 1. Build sidecar
        print("\n[1/7] Building sidecar...")
        if not build_sidecar():
            print("FATAL: could not build sidecar")
            sys.exit(1)
        results.ok("sidecar built")

        # 2. Start mock backend
        print("\n[2/7] Starting threaded HTTP backend on :8080...")
        backend = start_backend()
        results.ok(f"backend listening on :{BACKEND_PORT}")

        # 3. Start sidecar
        print("\n[3/7] Starting sidecar...")
        sidecar = start_sidecar()
        if not wait_for_sidecar():
            results.fail("sidecar start", "QUIC port not ready after timeout")
            sys.exit(1)
        results.ok("sidecar QUIC port :4224 ready")

        await asyncio.sleep(1.0)

        # 4. Connect via QUIC
        print("\n[4/7] Connecting via QUIC (ALPN: qmp, TLS insecure)...")

        config = QuicConfiguration(is_client=True, alpn_protocols=["qmp"])
        config.verify_mode = ssl.CERT_NONE

        async with connect(
            SIDECAR_HOST, SIDECAR_QUIC_PORT,
            configuration=config,
            create_protocol=TunnelClientProtocol,
        ) as client:
            await client.send_hello([
                "/hello", "/api", "/headers-test", "/health", "/ping",
            ])
            results.ok("HelloMessage sent with endpoints")

            await asyncio.sleep(0.5)

            # 5. Run test cases
            print("\n[5/7] Running QUIC tunnel tests...")

            try:
                resp = await client.send_tunnel_request(HTTP_METHOD_GET, "/ping", timeout=5.0)
                results.ok(f"ping: status={resp['code_response']}")
            except Exception as e:
                results.fail("ping", str(e))

            await test_get_request(client)
            await test_post_request(client)
            await test_delete_request(client)
            await test_put_request(client)
            await test_head_request(client)
            await test_custom_headers(client)
            await test_large_body(client)
            await test_many_sequential_requests(client)
            await test_nested_path(client)

            # 6. Overhead benchmark
            print("\n[6/7] Measuring proxy overhead "
                  f"({ITERATIONS} iter x {len(BENCH_SCENARIOS)} scenarios "
                  f"x {len(CONCURRENCIES)} concurrencies)...")
            overhead_results = await benchmark_overhead(client)

    except Exception as e:
        results.fail("QUIC connection", str(e))

    finally:
        # 7. Cleanup
        print("\n[7/7] Cleaning up...")
        if sidecar:
            sidecar.terminate()
            try:
                sidecar_stderr = sidecar.stderr.read() or b""
                sidecar.wait(timeout=3)
            except subprocess.TimeoutExpired:
                sidecar.kill()
        if backend:
            backend.shutdown()
        results.ok("processes terminated")

    # Summary
    print("\n" + "=" * 60)
    total = results.passed + results.failed
    print(f"Results: {results.passed}/{total} passed", end="")
    if results.failed:
        print(f", {results.failed} FAILED")
        print("\nFailures:")
        for err in results.errors:
            print(f"  - {err}")
    else:
        print(", 0 failed")
    print("=" * 60)

    # Overhead summary
    if overhead_results:
        print_overhead_summary(overhead_results)

    if results.failed and sidecar_stderr:
        print(f"\nSidecar stderr:\n{sidecar_stderr.decode(errors='replace')[-1500:]}")
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(run_tests())
