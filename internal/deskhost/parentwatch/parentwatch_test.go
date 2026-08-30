package parentwatch_test

import (
	"os"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/deskhost/parentwatch"
)

func TestWatch_RejectsInvalidPID(t *testing.T) {
	if err := parentwatch.Watch(0); err == nil {
		t.Fatal("Watch(0) error = nil, want error for zero PID")
	}
	if err := parentwatch.Watch(-1); err == nil {
		t.Fatal("Watch(-1) error = nil, want error for negative PID")
	}
}

func TestWatch_CurrentParent(t *testing.T) {
	// Watching the current real parent should succeed
	ppid := os.Getppid()
	if ppid <= 0 {
		t.Skip("no valid ppid")
	}
	if err := parentwatch.Watch(ppid); err != nil {
		t.Fatalf("Watch(%d) failed: %v", ppid, err)
	}
}
