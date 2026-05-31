package protocomm

type ServerContext struct {
	PeerAddress string
	MethodID    uint32
	CallID      uint32
}
