#!/usr/bin/env bash
# Prototype orchestrator. Answers, empirically, on Linux:
#  1. Can a Node parent spawn the Go binary and reach it over HTTP? (FR-1/FR-2)
#  2. Does a graceful SIGTERM shutdown complete cleanly? (FR-9)
#  3. Baseline: does the Go child survive a SIGKILL of its parent? (the FR-10 problem)
#  4. Pdeathsig: does the fix candidate prevent that? (the FR-10 fix)
set -uo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

section() { echo; echo "=== $1 ==="; }

wait_for_port_line() {
  # $1 = file with parent stdout so far
  for _ in $(seq 1 50); do
    if grep -q '^PORT=' "$1" 2>/dev/null; then return 0; fi
    sleep 0.1
  done
  return 1
}

test_basic_and_shutdown() {
  local label="$1" bin="$2"
  section "$label: spawn, health check, graceful shutdown"
  local out; out=$(mktemp)
  node "$DIR/node-parent/parent.js" "$bin" > "$out" 2>&1 &
  local parent_pid=$!
  wait_for_port_line "$out"
  local child_pid port
  child_pid=$(grep -m1 '^CHILD_PID=' "$out" | cut -d= -f2)
  port=$(grep -m1 '^PORT=' "$out" | cut -d= -f2)
  echo "parent_pid=$parent_pid child_pid=$child_pid port=$port"

  local health
  health=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$port/health")
  echo "health check: HTTP $health"

  local t0 t1
  t0=$(date +%s.%N)
  kill -TERM "$parent_pid"
  for _ in $(seq 1 50); do
    if ! kill -0 "$child_pid" 2>/dev/null; then break; fi
    sleep 0.1
  done
  t1=$(date +%s.%N)
  if kill -0 "$child_pid" 2>/dev/null; then
    echo "RESULT: child still alive after graceful SIGTERM — FAIL"
    kill -9 "$child_pid" 2>/dev/null
  else
    echo "RESULT: child exited cleanly in $(awk "BEGIN{printf \"%.2f\", $t1 - $t0}")s after graceful parent SIGTERM — PASS"
  fi
  cat "$out"
  rm -f "$out"
}

test_orphan() {
  local label="$1" bin="$2" expect="$3"
  section "$label: SIGKILL the parent, check the child"
  local out; out=$(mktemp)
  node "$DIR/node-parent/parent.js" "$bin" > "$out" 2>&1 &
  local parent_pid=$!
  wait_for_port_line "$out"
  local child_pid port
  child_pid=$(grep -m1 '^CHILD_PID=' "$out" | cut -d= -f2)
  port=$(grep -m1 '^PORT=' "$out" | cut -d= -f2)
  echo "parent_pid=$parent_pid child_pid=$child_pid port=$port"

  echo "SIGKILLing parent (simulates Electron main crashing, no cleanup code runs)"
  kill -9 "$parent_pid"
  sleep 1.5

  if kill -0 "$child_pid" 2>/dev/null; then
    local health
    health=$(curl -s -o /dev/null -w '%{http_code}' --max-time 1 "http://127.0.0.1:$port/health" || echo "unreachable")
    echo "RESULT: child ($child_pid) still ALIVE 1.5s after parent SIGKILL, still serving HTTP ($health) — orphan"
    if [ "$expect" = "should_die" ]; then
      echo "VERDICT: FAIL — expected the pdeathsig mechanism to have killed it"
    else
      echo "VERDICT: EXPECTED — baseline has no orphan-prevention mechanism"
    fi
    kill -9 "$child_pid" 2>/dev/null
  else
    echo "RESULT: child ($child_pid) is gone 1.5s after parent SIGKILL"
    if [ "$expect" = "should_die" ]; then
      echo "VERDICT: PASS — pdeathsig killed the orphan"
    else
      echo "VERDICT: UNEXPECTED for baseline — investigate"
    fi
  fi
  rm -f "$out"
}

BASELINE="$DIR/go-child-baseline/child-baseline"
PDEATHSIG="$DIR/go-child-pdeathsig/child-pdeathsig"

test_basic_and_shutdown "BASELINE" "$BASELINE"
test_basic_and_shutdown "PDEATHSIG" "$PDEATHSIG"
test_orphan "BASELINE" "$BASELINE" "should_survive"
test_orphan "PDEATHSIG" "$PDEATHSIG" "should_die"

echo
echo "=== done ==="
