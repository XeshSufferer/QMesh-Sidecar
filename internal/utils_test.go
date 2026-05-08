package internal

import (
	"bytes"
	"regexp"
	"testing"

	"QMesh-Sidecar/internal/protos/pb/gen"
)

func TestZeroAllocBytesToString(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"empty", []byte{}, ""},
		{"short", []byte("hello"), "hello"},
		{"path", []byte("/api/v1/users"), "/api/v1/users"},
		{"special chars", []byte("hello\x00world"), "hello\x00world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ZeroAllocBytesToString(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestZeroAllocStringToBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{"empty", "", []byte{}},
		{"short", "hello", []byte("hello")},
		{"path", "/api/v1/users", []byte("/api/v1/users")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ZeroAllocStringToBytes(tt.input)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestZeroAllocRoundTrip(t *testing.T) {
	original := []byte("/api/v1/users/123")
	s := ZeroAllocBytesToString(original)
	back := ZeroAllocStringToBytes(s)

	if !bytes.Equal(original, back) {
		t.Errorf("round trip failed: got %v, want %v", back, original)
	}
}

func TestUUIDv4(t *testing.T) {
	uuid := UUIDv4()

	if len(uuid) != 36 {
		t.Errorf("UUID length = %d, want 36", len(uuid))
	}

	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(uuid) {
		t.Errorf("UUID %q does not match v4 format", uuid)
	}
}

func TestUUIDv4Uniqueness(t *testing.T) {
	uuids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		uuid := UUIDv4()
		if uuids[uuid] {
			t.Errorf("duplicate UUID generated: %s", uuid)
		}
		uuids[uuid] = true
	}
}

func TestUUIDv4Version(t *testing.T) {
	for i := 0; i < 100; i++ {
		uuid := UUIDv4()
		if uuid[14] != '4' {
			t.Errorf("UUID version char = %c, want '4'", uuid[14])
		}
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		input    byte
		expected string
	}{
		{0x00, "00"},
		{0x0f, "0f"},
		{0xff, "ff"},
		{0xab, "ab"},
		{0x42, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			dst := make([]byte, 2)
			encode(dst, tt.input)
			if string(dst) != tt.expected {
				t.Errorf("encode(0x%02x) = %q, want %q", tt.input, dst, tt.expected)
			}
		})
	}
}

func TestMarshalToWriter(t *testing.T) {
	msg := &gen.TunnelRequest{
		Method: gen.HttpMethod_HTTP_METHOD_GET,
		Path:   []byte("/test"),
		Body:   []byte("body"),
	}

	var buf bytes.Buffer
	n, err := MarshalToWriter(&buf, msg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n == 0 {
		t.Error("expected non-zero bytes written")
	}

	if buf.Len() == 0 {
		t.Error("expected buffer to contain data")
	}

	var decoded gen.TunnelRequest
	if err := decoded.UnmarshalVT(buf.Bytes()); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Method != msg.Method {
		t.Errorf("method mismatch: got %v, want %v", decoded.Method, msg.Method)
	}

	if string(decoded.Path) != string(msg.Path) {
		t.Errorf("path mismatch: got %q, want %q", decoded.Path, msg.Path)
	}
}

func TestMarshalToWriter_EmptyMessage(t *testing.T) {
	msg := &gen.TunnelRequest{}

	var buf bytes.Buffer
	n, err := MarshalToWriter(&buf, msg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty protobuf message may serialize to 0 bytes
	if n < 0 {
		t.Error("expected non-negative bytes written")
	}
}

func TestBufferPool(t *testing.T) {
	buf1 := bufferPool.Get().(*buff)
	buf1.buff = append(buf1.buff, []byte("test")...)

	bufferPool.Put(buf1)

	buf2 := bufferPool.Get().(*buff)
	if cap(buf2.buff) < 1024 {
		t.Errorf("buffer capacity = %d, want >= 1024", cap(buf2.buff))
	}
}

func TestGetHeaderByID(t *testing.T) {
	tests := []struct {
		id      uint32
		wantK   []byte
		wantV   []byte
		wantOk  bool
	}{
		{0, nil, nil, false},
		{1, []byte("Content-Type"), []byte("application/json"), true},
		{2, []byte("Content-Type"), []byte("application/x-www-form-urlencoded"), true},
		{8, []byte("Connection"), []byte("keep-alive"), true},
		{9, []byte("Connection"), []byte("close"), true},
		{1000, nil, nil, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			k, v, ok := getHeaderByID(tt.id)
			if ok != tt.wantOk {
				t.Errorf("getHeaderByID(%d) ok = %v, want %v", tt.id, ok, tt.wantOk)
			}
			if tt.wantOk {
				if !bytes.Equal(k, tt.wantK) {
					t.Errorf("key = %v, want %v", k, tt.wantK)
				}
				if !bytes.Equal(v, tt.wantV) {
					t.Errorf("value = %v, want %v", v, tt.wantV)
				}
			}
		})
	}
}

func TestFindHeaderID(t *testing.T) {
	tests := []struct {
		key    []byte
		val    []byte
		wantID uint32
		wantOk bool
	}{
		{[]byte("Content-Type"), []byte("application/json"), 1, true},
		{[]byte("Connection"), []byte("keep-alive"), 8, true},
		{[]byte("X-Custom"), []byte("value"), 0, false},
		{[]byte("Content-Type"), []byte("unknown/type"), 0, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			id, ok := findHeaderID(tt.key, tt.val)
			if ok != tt.wantOk {
				t.Errorf("findHeaderID(%q, %q) ok = %v, want %v", tt.key, tt.val, ok, tt.wantOk)
			}
			if id != tt.wantID {
				t.Errorf("findHeaderID(%q, %q) id = %d, want %d", tt.key, tt.val, id, tt.wantID)
			}
		})
	}
}
