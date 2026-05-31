package protocomm

type MethodHandler func(ctx *ServerContext, request []byte) (response []byte, status Status)

type Service interface {
	RegisterWith(s *Server)
}
