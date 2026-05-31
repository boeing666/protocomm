package protocomm_test

import (
	"bytes"
	"testing"

	"protocomm"
)

func TestFrameSerializeDeserialize(t *testing.T) {
	hdr := protocomm.FrameHeader{
		Type:        protocomm.FrameRequest,
		StatusCode:  0,
		CallID:      42,
		MethodID:    0xDEADBEEF,
		PayloadSize: 100,
	}
	buf := hdr.Serialize()
	got := protocomm.DeserializeFrameHeader(buf)
	if got != hdr {
		t.Errorf("round-trip failed:\n got %+v\nwant %+v", got, hdr)
	}
}

func TestFrameByteLayout(t *testing.T) {
	hdr := protocomm.FrameHeader{
		Type:        protocomm.FrameRequest,
		StatusCode:  0,
		CallID:      1,
		MethodID:    0x12345678,
		PayloadSize: 256,
	}
	buf := hdr.Serialize()

	// Verify exact big-endian byte layout matching C++ FrameHeader::Serialize.
	checks := []struct {
		offset int
		expect byte
		desc   string
	}{
		{0, 1, "type=request"},
		{1, 0, "flags=0"},
		{2, 0, "status_code high"},
		{3, 0, "status_code low"},
		{4, 0, "call_id[0]"},
		{5, 0, "call_id[1]"},
		{6, 0, "call_id[2]"},
		{7, 1, "call_id[3]"},
		{8, 0x12, "method_id[0]"},
		{9, 0x34, "method_id[1]"},
		{10, 0x56, "method_id[2]"},
		{11, 0x78, "method_id[3]"},
		{12, 0, "payload_size[0]"},
		{13, 0, "payload_size[1]"},
		{14, 1, "payload_size[2]"},
		{15, 0, "payload_size[3]"},
	}
	for _, c := range checks {
		if buf[c.offset] != c.expect {
			t.Errorf("byte[%d] (%s): got 0x%02x, want 0x%02x",
				c.offset, c.desc, buf[c.offset], c.expect)
		}
	}
}

func TestFrameResponseStatus(t *testing.T) {
	hdr := protocomm.FrameHeader{
		Type:        protocomm.FrameResponse,
		StatusCode:  3, // INVALID_ARGUMENT
		CallID:      7,
		MethodID:    0xABCD,
		PayloadSize: 0,
	}
	buf := hdr.Serialize()
	got := protocomm.DeserializeFrameHeader(buf)
	if got.StatusCode != 3 {
		t.Errorf("status_code: got %d, want 3", got.StatusCode)
	}
	if got.Type != protocomm.FrameResponse {
		t.Errorf("type: got %d, want %d", got.Type, protocomm.FrameResponse)
	}
}

func TestWriteReadFrame(t *testing.T) {
	hdr := protocomm.FrameHeader{
		Type:        protocomm.FrameRequest,
		CallID:      5,
		MethodID:    12345,
		PayloadSize: 11,
	}
	payload := []byte("hello world")

	var buf bytes.Buffer
	if err := protocomm.WriteFrame(&buf, hdr, payload); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	gotHdr, gotPayload, err := protocomm.ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if gotHdr != hdr {
		t.Errorf("header:\n got %+v\nwant %+v", gotHdr, hdr)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Errorf("payload: got %q, want %q", gotPayload, payload)
	}
}

func TestWriteReadFrameEmpty(t *testing.T) {
	hdr := protocomm.FrameHeader{
		Type:     protocomm.FrameResponse,
		CallID:   1,
		MethodID: 42,
	}

	var buf bytes.Buffer
	if err := protocomm.WriteFrame(&buf, hdr, nil); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}

	gotHdr, gotPayload, err := protocomm.ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if gotHdr != hdr {
		t.Errorf("header:\n got %+v\nwant %+v", gotHdr, hdr)
	}
	if len(gotPayload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(gotPayload))
	}
}

func TestMultipleFrames(t *testing.T) {
	var buf bytes.Buffer

	frames := []struct {
		hdr     protocomm.FrameHeader
		payload []byte
	}{
		{protocomm.FrameHeader{Type: protocomm.FrameRequest, CallID: 1, MethodID: 100, PayloadSize: 3}, []byte("abc")},
		{protocomm.FrameHeader{Type: protocomm.FrameResponse, CallID: 1, MethodID: 100, PayloadSize: 3}, []byte("xyz")},
		{protocomm.FrameHeader{Type: protocomm.FramePush, CallID: 0, MethodID: 200, PayloadSize: 0}, nil},
	}

	for _, f := range frames {
		if err := protocomm.WriteFrame(&buf, f.hdr, f.payload); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}

	for i, f := range frames {
		gotHdr, gotPayload, err := protocomm.ReadFrame(&buf)
		if err != nil {
			t.Fatalf("ReadFrame[%d]: %v", i, err)
		}
		if gotHdr != f.hdr {
			t.Errorf("frame[%d] header:\n got %+v\nwant %+v", i, gotHdr, f.hdr)
		}
		if !bytes.Equal(gotPayload, f.payload) {
			t.Errorf("frame[%d] payload: got %q, want %q", i, gotPayload, f.payload)
		}
	}
}
