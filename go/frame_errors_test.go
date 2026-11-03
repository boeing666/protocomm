package protocomm_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/boeing666/protocomm/go"
)

// failingWriter accepts the first okWrites calls and fails every call after that.
type failingWriter struct {
	okWrites int
	calls    int
}

var errWrite = errors.New("write failed")

func (w *failingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls > w.okWrites {
		return 0, errWrite
	}
	return len(p), nil
}

func TestWriteFrameHeaderError(t *testing.T) {
	w := &failingWriter{okWrites: 0}
	err := protocomm.WriteFrame(w, protocomm.FrameHeader{Type: protocomm.FrameRequest, PayloadSize: 3}, []byte("abc"))
	if !errors.Is(err, errWrite) {
		t.Fatalf("got %v, want %v", err, errWrite)
	}
	if w.calls != 1 {
		t.Errorf("payload written after failed header: %d calls", w.calls)
	}
}

func TestWriteFramePayloadError(t *testing.T) {
	w := &failingWriter{okWrites: 1}
	err := protocomm.WriteFrame(w, protocomm.FrameHeader{Type: protocomm.FrameRequest, PayloadSize: 3}, []byte("abc"))
	if !errors.Is(err, errWrite) {
		t.Fatalf("got %v, want %v", err, errWrite)
	}
}

func TestReadFrameEmptyInput(t *testing.T) {
	_, _, err := protocomm.ReadFrame(bytes.NewReader(nil))
	if err != io.EOF {
		t.Fatalf("got %v, want io.EOF", err)
	}
}

func TestReadFrameTruncatedHeader(t *testing.T) {
	_, _, err := protocomm.ReadFrame(bytes.NewReader(make([]byte, protocomm.FrameHeaderSize-1)))
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("got %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestReadFrameTruncatedPayload(t *testing.T) {
	var buf bytes.Buffer
	hdr := protocomm.FrameHeader{Type: protocomm.FrameResponse, CallID: 7, PayloadSize: 10}
	ser := hdr.Serialize()
	buf.Write(ser[:])
	buf.WriteString("short")

	_, payload, err := protocomm.ReadFrame(&buf)
	if err != io.ErrUnexpectedEOF {
		t.Fatalf("got %v, want io.ErrUnexpectedEOF", err)
	}
	if payload != nil {
		t.Errorf("partial payload returned: %q", payload)
	}
}
