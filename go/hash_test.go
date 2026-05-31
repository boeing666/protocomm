package protocomm_test

import (
	"testing"

	"protocomm"
)

func TestFnv1a32KnownValues(t *testing.T) {
	// Standard FNV-1a 32-bit test vectors.
	tests := []struct {
		input    string
		expected uint32
	}{
		{"", 0x811c9dc5},
		{"a", 0xe40c292c},
		{"foo", 0xa9f37ed7},
		{"foobar", 0xbf9cf968},
	}
	for _, tt := range tests {
		got := protocomm.Fnv1a32(tt.input)
		if got != tt.expected {
			t.Errorf("Fnv1a32(%q) = 0x%08x, want 0x%08x", tt.input, got, tt.expected)
		}
	}
}

func TestMethodIDFormat(t *testing.T) {
	// MethodID must produce the same hash as Fnv1a32 of the full method path.
	tests := []struct {
		pkg, svc, method, fullName string
	}{
		{"", "Greeter", "SayHello", "/Greeter/SayHello"},
		{"", "Greeter", "SayHello1", "/Greeter/SayHello1"},
		{"math", "Calculator", "Add", "/math.Calculator/Add"},
		{"math", "Calculator", "Subtract", "/math.Calculator/Subtract"},
		{"math", "Calculator", "Multiply", "/math.Calculator/Multiply"},
		{"math", "Calculator", "Divide", "/math.Calculator/Divide"},
		{"messaging", "Chat", "SendMessage", "/messaging.Chat/SendMessage"},
		{"messaging", "Chat", "GetHistory", "/messaging.Chat/GetHistory"},
		{"messaging", "Chat", "Ping", "/messaging.Chat/Ping"},
	}
	for _, tt := range tests {
		id := protocomm.MethodID(tt.pkg, tt.svc, tt.method)
		expected := protocomm.Fnv1a32(tt.fullName)
		if id != expected {
			t.Errorf("MethodID(%q,%q,%q) = 0x%08x, Fnv1a32(%q) = 0x%08x",
				tt.pkg, tt.svc, tt.method, id, tt.fullName, expected)
		}
	}
}

func TestFnv1a32NoCollisions(t *testing.T) {
	// All method names used in the project must hash to unique values.
	methods := []string{
		"/Greeter/SayHello",
		"/Greeter/SayHello1",
		"/Greeter/SayHello2",
		"/math.Calculator/Add",
		"/math.Calculator/Subtract",
		"/math.Calculator/Multiply",
		"/math.Calculator/Divide",
		"/messaging.Chat/SendMessage",
		"/messaging.Chat/GetHistory",
		"/messaging.Chat/Ping",
	}
	seen := make(map[uint32]string)
	for _, m := range methods {
		h := protocomm.Fnv1a32(m)
		if prev, ok := seen[h]; ok {
			t.Errorf("collision: %q and %q both → 0x%08x", m, prev, h)
		}
		seen[h] = m
	}
}
