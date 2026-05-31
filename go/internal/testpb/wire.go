package testpb

import (
	"encoding/binary"
	"errors"
	"math"
)

const (
	WireVarint  = 0
	WireFixed64 = 1
	WireBytes   = 2
	WireFixed32 = 5
)

type Encoder struct {
	buf []byte
}

func (e *Encoder) Bytes() []byte { return e.buf }

func (e *Encoder) appendVarint(v uint64) {
	for v >= 0x80 {
		e.buf = append(e.buf, byte(v)|0x80)
		v >>= 7
	}
	e.buf = append(e.buf, byte(v))
}

func (e *Encoder) appendTag(fieldNum int, wireType int) {
	e.appendVarint(uint64(fieldNum<<3) | uint64(wireType))
}

func (e *Encoder) EncodeString(fieldNum int, s string) {
	if s == "" {
		return
	}
	e.appendTag(fieldNum, WireBytes)
	e.appendVarint(uint64(len(s)))
	e.buf = append(e.buf, s...)
}

func (e *Encoder) EncodeBytes(fieldNum int, data []byte) {
	if len(data) == 0 {
		return
	}
	e.appendTag(fieldNum, WireBytes)
	e.appendVarint(uint64(len(data)))
	e.buf = append(e.buf, data...)
}

func (e *Encoder) EncodeDouble(fieldNum int, v float64) {
	if v == 0 {
		return
	}
	e.appendTag(fieldNum, WireFixed64)
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(v))
	e.buf = append(e.buf, tmp[:]...)
}

func (e *Encoder) EncodeUint64(fieldNum int, v uint64) {
	if v == 0 {
		return
	}
	e.appendTag(fieldNum, WireVarint)
	e.appendVarint(v)
}

func (e *Encoder) EncodeUint32(fieldNum int, v uint32) {
	if v == 0 {
		return
	}
	e.appendTag(fieldNum, WireVarint)
	e.appendVarint(uint64(v))
}

func (e *Encoder) EncodeBool(fieldNum int, v bool) {
	if !v {
		return
	}
	e.appendTag(fieldNum, WireVarint)
	e.appendVarint(1)
}

type Decoder struct {
	buf []byte
	pos int
}

func NewDecoder(data []byte) *Decoder {
	return &Decoder{buf: data}
}

func (d *Decoder) Done() bool { return d.pos >= len(d.buf) }

func (d *Decoder) consumeVarint() (uint64, error) {
	var v uint64
	for i := 0; ; i++ {
		if d.pos >= len(d.buf) {
			return 0, errors.New("unexpected EOF in varint")
		}
		if i >= 10 {
			return 0, errors.New("varint too long")
		}
		b := d.buf[d.pos]
		d.pos++
		v |= uint64(b&0x7F) << (7 * uint(i))
		if b < 0x80 {
			return v, nil
		}
	}
}

func (d *Decoder) ReadTag() (fieldNum int, wireType int, err error) {
	tag, err := d.consumeVarint()
	if err != nil {
		return 0, 0, err
	}
	return int(tag >> 3), int(tag & 0x7), nil
}

func (d *Decoder) ReadVarint() (uint64, error) {
	return d.consumeVarint()
}

func (d *Decoder) ReadFixed64() (uint64, error) {
	if d.pos+8 > len(d.buf) {
		return 0, errors.New("unexpected EOF in fixed64")
	}
	v := binary.LittleEndian.Uint64(d.buf[d.pos:])
	d.pos += 8
	return v, nil
}

func (d *Decoder) ReadDouble() (float64, error) {
	v, err := d.ReadFixed64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(v), nil
}

func (d *Decoder) ReadBytes() ([]byte, error) {
	length, err := d.consumeVarint()
	if err != nil {
		return nil, err
	}
	end := d.pos + int(length)
	if end > len(d.buf) {
		return nil, errors.New("unexpected EOF in bytes")
	}
	data := make([]byte, length)
	copy(data, d.buf[d.pos:end])
	d.pos = end
	return data, nil
}

func (d *Decoder) ReadString() (string, error) {
	b, err := d.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (d *Decoder) Skip(wireType int) error {
	switch wireType {
	case WireVarint:
		_, err := d.consumeVarint()
		return err
	case WireFixed64:
		if d.pos+8 > len(d.buf) {
			return errors.New("EOF")
		}
		d.pos += 8
		return nil
	case WireFixed32:
		if d.pos+4 > len(d.buf) {
			return errors.New("EOF")
		}
		d.pos += 4
		return nil
	case WireBytes:
		length, err := d.consumeVarint()
		if err != nil {
			return err
		}
		if d.pos+int(length) > len(d.buf) {
			return errors.New("EOF")
		}
		d.pos += int(length)
		return nil
	default:
		return errors.New("unknown wire type")
	}
}
