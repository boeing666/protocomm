package protocomm

type ServerContext struct {
	PeerAddress string
	MethodID    uint32
	CallID      uint32

	Trace        bool
	MethodName   string
	RequestText  string
	ResponseText string
}
