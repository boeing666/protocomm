package protocomm

import (
	"encoding/binary"
	"io"
)

const FrameHeaderSize = 16

type FrameType uint8

const (
	FrameRequest  FrameType = 1
	FrameResponse FrameType = 2
	FramePush     FrameType = 3
)

type FrameHeader struct {
	Type        FrameType
	StatusCode  uint16
	CallID      uint32
	MethodID    uint32
	PayloadSize uint32
}

func (h *FrameHeader) Serialize() [FrameHeaderSize]byte {
	var buf [FrameHeaderSize]byte
	buf[0] = byte(h.Type)
	buf[1] = 0
	binary.BigEndian.PutUint16(buf[2:4], h.StatusCode)
	binary.BigEndian.PutUint32(buf[4:8], h.CallID)
	binary.BigEndian.PutUint32(buf[8:12], h.MethodID)
	binary.BigEndian.PutUint32(buf[12:16], h.PayloadSize)
	return buf
}

func DeserializeFrameHeader(buf [FrameHeaderSize]byte) FrameHeader {
	return FrameHeader{
		Type:        FrameType(buf[0]),
		StatusCode:  binary.BigEndian.Uint16(buf[2:4]),
		CallID:      binary.BigEndian.Uint32(buf[4:8]),
		MethodID:    binary.BigEndian.Uint32(buf[8:12]),
		PayloadSize: binary.BigEndian.Uint32(buf[12:16]),
	}
}

func WriteFrame(w io.Writer, hdr FrameHeader, payload []byte) error {
	buf := hdr.Serialize()
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

func ReadFrame(r io.Reader) (FrameHeader, []byte, error) {
	var buf [FrameHeaderSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return FrameHeader{}, nil, err
	}
	hdr := DeserializeFrameHeader(buf)
	var payload []byte
	if hdr.PayloadSize > 0 {
		payload = make([]byte, hdr.PayloadSize)
		if _, err := io.ReadFull(r, payload); err != nil {
			return FrameHeader{}, nil, err
		}
	}
	return hdr, payload, nil
}
