package protocomm

import "fmt"

type StatusCode int

const (
	OK                 StatusCode = 0
	Cancelled          StatusCode = 1
	Unknown            StatusCode = 2
	InvalidArgument    StatusCode = 3
	DeadlineExceeded   StatusCode = 4
	NotFound           StatusCode = 5
	AlreadyExists      StatusCode = 6
	PermissionDenied   StatusCode = 7
	ResourceExhausted  StatusCode = 8
	FailedPrecondition StatusCode = 9
	Aborted            StatusCode = 10
	OutOfRange         StatusCode = 11
	Unimplemented      StatusCode = 12
	Internal           StatusCode = 13
	Unavailable        StatusCode = 14
	DataLoss           StatusCode = 15
	Unauthenticated    StatusCode = 16
)

var statusCodeNames = map[StatusCode]string{
	OK: "OK", Cancelled: "CANCELLED", Unknown: "UNKNOWN",
	InvalidArgument: "INVALID_ARGUMENT", DeadlineExceeded: "DEADLINE_EXCEEDED",
	NotFound: "NOT_FOUND", AlreadyExists: "ALREADY_EXISTS",
	PermissionDenied: "PERMISSION_DENIED", ResourceExhausted: "RESOURCE_EXHAUSTED",
	FailedPrecondition: "FAILED_PRECONDITION", Aborted: "ABORTED",
	OutOfRange: "OUT_OF_RANGE", Unimplemented: "UNIMPLEMENTED",
	Internal: "INTERNAL", Unavailable: "UNAVAILABLE",
	DataLoss: "DATA_LOSS", Unauthenticated: "UNAUTHENTICATED",
}

func (c StatusCode) String() string {
	if name, ok := statusCodeNames[c]; ok {
		return name
	}
	return fmt.Sprintf("StatusCode(%d)", int(c))
}

type Status struct {
	Code    StatusCode
	Message string
}

func (s Status) IsOK() bool { return s.Code == OK }

func (s Status) String() string {
	if s.IsOK() {
		return "OK"
	}
	return fmt.Sprintf("%s: %s", s.Code, s.Message)
}

func StatusOK() Status { return Status{} }
