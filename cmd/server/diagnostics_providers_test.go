package main

import (
	"context"
	"errors"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/jobs"
)

func TestQueueDepthProvider_MapsStatesToStrings(t *testing.T) {
	count := func(context.Context) (map[jobs.State]int, error) {
		return map[jobs.State]int{jobs.StateQueued: 3, jobs.StateRunning: 1}, nil
	}

	got, err := queueDepthProvider(count)(context.Background())
	if err != nil {
		t.Fatalf("queueDepthProvider: %v", err)
	}
	if got["queued"] != 3 || got["running"] != 1 {
		t.Fatalf("got %v, want queued=3 running=1", got)
	}
}

func TestQueueDepthProvider_PropagatesError(t *testing.T) {
	boom := errors.New("db down")
	_, err := queueDepthProvider(func(context.Context) (map[jobs.State]int, error) {
		return nil, boom
	})(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the count error propagated", err)
	}
}
