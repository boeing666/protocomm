package prototrace

import (
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

const MaxBody = 1024

var marshal = prototext.MarshalOptions{Multiline: false}

func Render(m proto.Message) string {
	if m == nil || !m.ProtoReflect().IsValid() {
		return ""
	}
	out, err := marshal.Marshal(m)
	if err != nil {
		return ""
	}
	if len(out) > MaxBody {
		return string(out[:MaxBody]) + "…"
	}
	return string(out)
}
