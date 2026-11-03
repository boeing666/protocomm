package protocomm_test

import (
	"testing"

	"github.com/boeing666/protocomm/go"
)

func TestStatusCodeString(t *testing.T) {
	cases := map[protocomm.StatusCode]string{
		protocomm.OK:              "OK",
		protocomm.InvalidArgument: "INVALID_ARGUMENT",
		protocomm.Unavailable:     "UNAVAILABLE",
		protocomm.Unauthenticated: "UNAUTHENTICATED",
		protocomm.StatusCode(42):  "StatusCode(42)",
		protocomm.StatusCode(-1):  "StatusCode(-1)",
	}
	for code, want := range cases {
		if got := code.String(); got != want {
			t.Errorf("StatusCode(%d).String() = %q, want %q", int(code), got, want)
		}
	}
}

func TestStatusString(t *testing.T) {
	if got := protocomm.StatusOK().String(); got != "OK" {
		t.Errorf("StatusOK().String() = %q, want %q", got, "OK")
	}
	// The message of an OK status is ignored.
	if got := (protocomm.Status{Code: protocomm.OK, Message: "ignored"}).String(); got != "OK" {
		t.Errorf("OK with message: got %q", got)
	}
	st := protocomm.Status{Code: protocomm.NotFound, Message: "no such user"}
	if got := st.String(); got != "NOT_FOUND: no such user" {
		t.Errorf("error status: got %q", got)
	}
	if st.IsOK() {
		t.Error("NotFound status reported as OK")
	}
}
