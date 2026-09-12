package supervisor_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres/supervisor"
)

// A real OS interaction (a real net.Listen probe), not faked —
// there's no faithful pure-logic substitute for "ask the OS for a free
// port." Called twice; only validity is asserted, not that the two calls
// differ — the OS is free to reuse a just-closed port, and asserting
// difference would make this test flaky.
func TestSelectPort_ReturnsAValidPort(t *testing.T) {
	for i := 0; i < 2; i++ {
		port, err := supervisor.SelectPort()
		if err != nil {
			t.Fatalf("SelectPort() [call %d]: %v", i, err)
		}
		if port <= 0 || port > 65535 {
			t.Fatalf("SelectPort() [call %d] = %d, want a valid TCP port", i, port)
		}
	}
}
