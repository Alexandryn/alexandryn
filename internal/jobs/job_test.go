package jobs

import (
	"errors"
	"testing"
)

func TestState_IsTerminal(t *testing.T) {
	terminal := map[State]bool{
		StateQueued:     false,
		StateRunning:    false,
		StateRetrying:   false,
		StateCompleted:  true,
		StateDeadLetter: true,
	}
	for s, want := range terminal {
		if got := s.IsTerminal(); got != want {
			t.Errorf("%s.IsTerminal() = %v, want %v", s, got, want)
		}
	}
}

func TestState_CanTransitionTo_LegalPaths(t *testing.T) {
	legal := []struct{ from, to State }{
		{StateQueued, StateRunning},
		{StateRetrying, StateRunning},
		{StateRunning, StateCompleted},
		{StateRunning, StateRetrying},
		{StateRunning, StateDeadLetter},
	}
	for _, tc := range legal {
		if !tc.from.CanTransitionTo(tc.to) {
			t.Errorf("%s -> %s should be legal", tc.from, tc.to)
		}
	}
}

func TestState_CanTransitionTo_TerminalIsFinal(t *testing.T) {
	for _, from := range []State{StateCompleted, StateDeadLetter} {
		for _, to := range []State{StateQueued, StateRunning, StateRetrying, StateCompleted, StateDeadLetter} {
			if from.CanTransitionTo(to) {
				t.Errorf("%s -> %s must be rejected: %s is terminal", from, to, from)
			}
		}
	}
}

func TestState_CanTransitionTo_RejectsSkips(t *testing.T) {
	illegal := []struct{ from, to State }{
		{StateQueued, StateCompleted},
		{StateQueued, StateRetrying},
		{StateQueued, StateDeadLetter},
		{StateRetrying, StateCompleted},
		{StateRunning, StateQueued},
		{StateRunning, StateRunning},
	}
	for _, tc := range illegal {
		if tc.from.CanTransitionTo(tc.to) {
			t.Errorf("%s -> %s should be illegal", tc.from, tc.to)
		}
	}
}

func TestPermanent_WrapsAndUnwraps(t *testing.T) {
	base := errors.New("source returned 404")
	p := Permanent(base)

	if !IsPermanent(p) {
		t.Fatal("IsPermanent(Permanent(err)) = false")
	}
	if !errors.Is(p, base) {
		t.Fatal("Permanent(err) does not unwrap to err")
	}
	if p.Error() != base.Error() {
		t.Fatalf("Error() = %q, want %q", p.Error(), base.Error())
	}
}

func TestPermanent_Nil(t *testing.T) {
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) should be nil")
	}
	if IsPermanent(nil) {
		t.Fatal("IsPermanent(nil) should be false")
	}
	if IsPermanent(errors.New("plain transient error")) {
		t.Fatal("a plain error must not read as Permanent")
	}
}
