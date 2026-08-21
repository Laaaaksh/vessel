package ui

import (
	"testing"
	"time"
)

// The outer bounds this package wraps an action in are the UI-side halves of a
// contract whose client-side halves are backend's identically sized per-call
// budgets. backend's TestLongOperationBudgets_matchInternalUIOuterBounds pins
// its half against the same numbers; both constants are unexported, so the
// pairing needs an assertion on each side or a drift here would silently make
// the UI context the earlier deadline again.
const (
	wantConfirmBound = 60 * time.Second
	wantGlobalBound  = 120 * time.Second
	wantExecBound    = 30 * time.Second
)

func TestOuterBounds_matchBackendPerCallBudgets(t *testing.T) {
	if confirmTimeout != wantConfirmBound {
		t.Fatalf("confirmed-removal bound = %v, want %v (backend confirmTimeout)", confirmTimeout, wantConfirmBound)
	}
	if globalTimeout != wantGlobalBound {
		t.Fatalf("whole-store/transfer bound = %v, want %v (backend globalTimeout)", globalTimeout, wantGlobalBound)
	}
	if execTimeout != wantExecBound {
		t.Fatalf("one-shot exec bound = %v, want %v (backend execTimeout)", execTimeout, wantExecBound)
	}
}
