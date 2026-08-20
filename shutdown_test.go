// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// shutdown_test.go — prüft, WANN der Rechner ausgeschaltet wird.
//
// Kein Test darf dabei wirklich abschalten: Der Wächter ruft shutdown.exe über
// eine austauschbare Funktion, die hier nur mitschreibt, was aufgerufen worden
// wäre.
package main

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// testGuard baut einen Wächter, dessen Umgebung der Test selbst bestimmt.
type testGuard struct {
	guard    *shutdownGuard
	commands [][]string
	notes    []string
	states   []ShutdownState
	mu       sync.Mutex
}

func newTestGuard(busy, watching bool, found map[string]int, listErr error) *testGuard {
	harness := &testGuard{}
	guard := newShutdownGuard(
		func() bool { return busy },
		func() bool { return watching },
		func(text string) {
			harness.mu.Lock()
			defer harness.mu.Unlock()
			harness.notes = append(harness.notes, text)
		},
		func(state ShutdownState) {
			harness.mu.Lock()
			defer harness.mu.Unlock()
			harness.states = append(harness.states, state)
		},
	)
	guard.others = func() (map[string]int, error) { return found, listErr }
	guard.command = func(args ...string) error {
		harness.mu.Lock()
		defer harness.mu.Unlock()
		harness.commands = append(harness.commands, args)
		return nil
	}
	guard.settle = 0
	harness.guard = guard
	return harness
}

func (h *testGuard) commandCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.commands)
}

func (h *testGuard) allNotes() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.notes, " | ")
}

func TestNothingHappensWithoutTheWish(t *testing.T) {
	harness := newTestGuard(false, false, nil, nil)
	harness.guard.fireNow()
	if harness.commandCount() != 0 {
		t.Errorf("nobody asked for a shutdown, yet %v was run", harness.commands)
	}
}

func TestShutdownRunsOnlyWhenEverythingIsDone(t *testing.T) {
	harness := newTestGuard(false, false, nil, nil)
	harness.guard.Arm(true)
	harness.guard.fireNow()

	if harness.commandCount() != 1 {
		t.Fatalf("expected exactly one call, got %v", harness.commands)
	}
	got := strings.Join(harness.commands[0], " ")
	if got != "/s /t 60" {
		t.Errorf("expected Windows to be asked for a 60 second shutdown, got %q", got)
	}
	if !harness.guard.Status().Counting {
		t.Error("the window must know that the countdown is running")
	}
}

// Der Kernfall des Fehlers bis v1.0.1: Es läuft noch etwas.
func TestNoShutdownWhileWorkIsLeft(t *testing.T) {
	harness := newTestGuard(true, false, nil, nil)
	harness.guard.Arm(true)
	harness.guard.fireNow()

	if harness.commandCount() != 0 {
		t.Errorf("something is still running, yet %v was run", harness.commands)
	}
	// Der Wunsch bleibt stehen: Am Ende des Stapels soll er ja greifen.
	if !harness.guard.Status().Armed {
		t.Error("the wish must survive a run that is still going")
	}
}

func TestNoShutdownWhileAFolderIsWatched(t *testing.T) {
	harness := newTestGuard(false, true, nil, nil)
	harness.guard.Arm(true)
	harness.guard.fireNow()

	if harness.commandCount() != 0 {
		t.Errorf("a folder is being watched, yet %v was run", harness.commands)
	}
	if harness.guard.Status().Armed {
		t.Error("the wish must be dropped, not left standing")
	}
	if !strings.Contains(harness.allNotes(), "watched") {
		t.Errorf("the log must say why the box was cleared: %q", harness.allNotes())
	}
}

// Das Sicherheitsnetz: Der Nutzer hat NVENCForge von Hand gestartet, davon
// weiß dieses Fenster nichts — nur die Prozessliste des Systems.
func TestNoShutdownWhileSomethingElseConverts(t *testing.T) {
	harness := newTestGuard(false, false, map[string]int{"ffmpeg.exe": 2}, nil)
	harness.guard.Arm(true)
	harness.guard.fireNow()

	if harness.commandCount() != 0 {
		t.Errorf("a foreign converter is running, yet %v was run", harness.commands)
	}
	if !strings.Contains(harness.allNotes(), "ffmpeg.exe") {
		t.Errorf("the log must name what is still running: %q", harness.allNotes())
	}
}

// Lieber angelassen als blind ausgeschaltet.
func TestNoShutdownWhenTheProcessListCannotBeRead(t *testing.T) {
	harness := newTestGuard(false, false, nil, errors.New("no access"))
	harness.guard.Arm(true)
	harness.guard.fireNow()

	if harness.commandCount() != 0 {
		t.Errorf("the check failed, yet %v was run", harness.commands)
	}
	if harness.guard.Status().Armed {
		t.Error("an unchecked machine must not stay armed")
	}
}

func TestCancelStopsTheCountdownAndTheWish(t *testing.T) {
	harness := newTestGuard(false, false, nil, nil)
	harness.guard.Arm(true)
	harness.guard.fireNow()

	state, err := harness.guard.CancelCountdown()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Counting || state.Armed {
		t.Errorf("after cancelling, nothing may be pending: %+v", state)
	}
	if harness.commandCount() != 2 || strings.Join(harness.commands[1], " ") != "/a" {
		t.Errorf("expected Windows' own cancel, got %v", harness.commands)
	}

	// Und es darf nicht gleich wieder losgehen.
	harness.guard.fireNow()
	if harness.commandCount() != 2 {
		t.Errorf("a cancelled shutdown must stay cancelled, got %v", harness.commands)
	}
}

func TestCancelWithoutACountdownAsksWindowsForNothing(t *testing.T) {
	harness := newTestGuard(false, false, nil, nil)
	harness.guard.Arm(true)

	if _, err := harness.guard.CancelCountdown(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if harness.commandCount() != 0 {
		t.Errorf("nothing was scheduled, so nothing may be cancelled: %v", harness.commands)
	}
	if harness.guard.Status().Armed {
		t.Error("clicking cancel must clear the wish as well")
	}
}

// Während Windows herunterzählt, ändert das Kästchen nichts mehr — sonst
// stünde die Anzeige auf "aus", während der Countdown weiterläuft.
func TestTheBoxCannotUndoARunningCountdown(t *testing.T) {
	harness := newTestGuard(false, false, nil, nil)
	harness.guard.Arm(true)
	harness.guard.fireNow()

	state := harness.guard.Arm(false)
	if !state.Counting {
		t.Error("the countdown must still be reported as running")
	}
	if harness.commandCount() != 1 {
		t.Errorf("unticking the box must not talk to Windows: %v", harness.commands)
	}
}

// MaybeFire ist der Weg, den der Verteiler nimmt: nach kurzer Karenz und in
// einer eigenen Goroutine, damit das Nachrücken nicht wartet.
func TestMaybeFireGoesThroughAfterTheSettleTime(t *testing.T) {
	harness := newTestGuard(false, false, nil, nil)
	done := make(chan struct{})
	harness.guard.command = func(args ...string) error {
		harness.mu.Lock()
		harness.commands = append(harness.commands, args)
		harness.mu.Unlock()
		close(done)
		return nil
	}
	harness.guard.settle = time.Millisecond
	harness.guard.Arm(true)
	harness.guard.MaybeFire()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the shutdown was never scheduled")
	}
}
