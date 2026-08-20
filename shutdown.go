// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// shutdown.go — "PC ausschalten, wenn alles fertig ist".
//
// Warum das hier steht und nicht mehr im Konverter:
//
// Der Konverter kennt nur den Stapel, den er selbst bekommen hat. Dieses
// Fenster gibt ihm aber JEDE DATEI EINZELN (siehe runargs.go, buildJobs) —
// mit dem Schalter -shutdown hätte jede fertige Datei den Rechner
// heruntergefahren, mitten in der Arbeit der übrigen. Genau das ist bis
// v1.0.1 passiert.
//
// Die Entscheidung gehört deshalb an die einzige Stelle mit Überblick: hier.
// Ausgeschaltet wird erst, wenn
//
//  1. der Verteiler in ALLEN Bereichen leer ist (Umwandeln, Zerlegen,
//     Zusammenfügen, beobachteter Ordner),
//  2. kein Ordner beobachtet wird — ein Dauerauftrag wird nie "fertig",
//  3. auch außerhalb dieses Fensters kein Konverter läuft (processes.go),
//  4. und der Stapel nicht von Hand abgebrochen wurde.
//
// Ausgeschaltet wird weiterhin von Windows selbst ("shutdown /s"), nicht von
// diesem Programm: Das ist der Weg, den Windows für seine eigene Warnung und
// für "shutdown /a" vorsieht. Dieses Fenster setzt den Befehl nur ab und zeigt
// den Countdown mit einem Abbrechen-Knopf noch einmal deutlich an.
package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

const (
	// Wie lange Windows nach dem Auslösen wartet. Eine Minute, damit ein
	// Abbruch auch dann noch möglich ist, wenn man den Raum gerade erst
	// wieder betritt.
	shutdownDelaySeconds = 60

	// Karenz zwischen "der letzte Lauf ist zu Ende" und dem Nachsehen.
	// Ein gerade beendeter Konverter kann noch ein FFmpeg im Auslaufen haben,
	// und die Fensterseite reiht Folgearbeit erst nach dem Ende ein.
	shutdownSettleDelay = 2 * time.Second
)

// ShutdownState ist das, was die Oberfläche über das Ausschalten wissen muss.
type ShutdownState struct {
	Armed    bool   `json:"armed"`    // der Wunsch steht
	Counting bool   `json:"counting"` // Windows zählt bereits herunter
	Seconds  int    `json:"seconds"`  // wie lange der Countdown insgesamt läuft
	Note     string `json:"note"`     // warum es abgesagt wurde ("" = nichts)
}

// shutdownGuard hält den Wunsch und entscheidet, wann er ausgeführt wird.
//
// Alles, was nach außen wirkt, steckt in austauschbaren Funktionen: Nur so
// lässt sich der Ablauf prüfen, ohne beim Testen wirklich den Rechner
// auszuschalten.
type shutdownGuard struct {
	mu       sync.Mutex
	armed    bool
	counting bool
	pending  bool // die Karenz läuft gerade

	busy     func() bool                    // ist im Verteiler noch etwas zu tun?
	watching func() bool                    // wird ein Ordner beobachtet?
	note     func(string)                   // Zeile ins Protokoll des Fensters
	announce func(ShutdownState)            // Stand an die Oberfläche melden
	others   func() (map[string]int, error) // fremde Konverter-Prozesse
	command  func(args ...string) error     // ruft shutdown.exe
	settle   time.Duration
}

// newShutdownGuard verdrahtet den Wächter mit seiner Umgebung.
func newShutdownGuard(
	busy func() bool,
	watching func() bool,
	note func(string),
	announce func(ShutdownState),
) *shutdownGuard {
	return &shutdownGuard{
		busy:     busy,
		watching: watching,
		note:     note,
		announce: announce,
		others:   runningConverters,
		command:  runShutdownCommand,
		settle:   shutdownSettleDelay,
	}
}

// runShutdownCommand ruft Windows' eigenes shutdown.exe auf.
//
// Kein CREATE_NO_WINDOW nötig: Dieses Programm hat seit wincon.go eine eigene,
// versteckte Konsole, die das Kind erbt — es blitzt also nichts auf.
func runShutdownCommand(args ...string) error {
	if err := exec.Command("shutdown", args...).Run(); err != nil {
		return fmt.Errorf("shutdown.go: runShutdownCommand %v: %w", args, err)
	}
	return nil
}

// Status liefert den aktuellen Stand.
func (g *shutdownGuard) Status() ShutdownState {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.stateLocked("")
}

// stateLocked baut die Meldung. Nur mit gehaltener Sperre aufrufen.
func (g *shutdownGuard) stateLocked(note string) ShutdownState {
	return ShutdownState{
		Armed:    g.armed,
		Counting: g.counting,
		Seconds:  shutdownDelaySeconds,
		Note:     note,
	}
}

// Arm nimmt den Wunsch an oder zurück.
//
// Er darf jederzeit geändert werden, auch mitten in einem Stapel: Entschieden
// wird erst am Ende, also gilt immer der zuletzt gesetzte Stand. Nur solange
// Windows schon herunterzählt, ändert das Häkchen nichts mehr — dafür gibt es
// den Abbrechen-Knopf.
func (g *shutdownGuard) Arm(on bool) ShutdownState {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.counting {
		return g.stateLocked("")
	}
	g.armed = on
	return g.stateLocked("")
}

// Disarm nimmt den Wunsch zurück, weil etwas dagegen spricht — ein Abbruch von
// Hand, ein eingeschalteter beobachteter Ordner, ein fremder Konverter.
//
// Der Grund steht ausdrücklich im Protokoll: Ein stillschweigend gelöschtes
// Häkchen wäre für den Nutzer nicht von einem Fehler zu unterscheiden.
func (g *shutdownGuard) Disarm(reason string) ShutdownState {
	g.mu.Lock()
	wasArmed := g.armed
	g.armed = false
	state := g.stateLocked(reason)
	counting := g.counting
	g.mu.Unlock()

	if wasArmed && !counting && reason != "" {
		g.note("Shut down when finished: switched off — " + reason)
	}
	g.announce(state)
	return state
}

// MaybeFire wird gerufen, sobald im Verteiler nichts mehr läuft und nichts
// mehr wartet.
//
// Die eigentliche Prüfung geschieht nach einer kurzen Karenz in einer eigenen
// Goroutine: Sie darf den Verteiler nicht aufhalten, und der Blick in die
// Prozessliste ist erst danach aussagekräftig.
func (g *shutdownGuard) MaybeFire() {
	g.mu.Lock()
	if !g.armed || g.counting || g.pending {
		g.mu.Unlock()
		return
	}
	g.pending = true
	settle := g.settle
	g.mu.Unlock()

	go func() {
		time.Sleep(settle)
		g.fireNow()
	}()
}

// fireNow prüft die letzten Bedingungen und setzt den Befehl ab.
func (g *shutdownGuard) fireNow() {
	g.mu.Lock()
	g.pending = false
	if !g.armed || g.counting {
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()

	// Inzwischen wieder Arbeit da? Dann ist nichts verloren: Der Wunsch bleibt
	// stehen und wird beim nächsten Leerlauf erneut geprüft.
	if g.busy() {
		return
	}
	if g.watching() {
		g.Disarm("a folder is being watched, so the batch never really ends")
		return
	}

	found, err := g.others()
	if err != nil {
		// Lieber angelassen als blind ausgeschaltet: Wenn die Prozessliste
		// nicht zu lesen ist, ist unbekannt, ob nebenher noch umgewandelt wird.
		g.Disarm("the running programs could not be checked (" + err.Error() + ")")
		return
	}
	if len(found) > 0 {
		g.Disarm("something is still converting outside this window (" + describeProcesses(found) + ")")
		return
	}

	if err := g.command("/s", "/t", strconv.Itoa(shutdownDelaySeconds)); err != nil {
		g.Disarm("Windows refused to schedule it (" + err.Error() + ")")
		return
	}

	g.mu.Lock()
	g.counting = true
	state := g.stateLocked("")
	g.mu.Unlock()

	g.note(fmt.Sprintf("Everything is done — the PC shuts down in %d seconds. Cancel is on screen.", shutdownDelaySeconds))
	g.announce(state)
}

// CancelCountdown bläst einen laufenden Countdown ab und nimmt den Wunsch
// gleich mit zurück: Wer abbricht, will nicht, dass die nächste fertige Datei
// dasselbe noch einmal auslöst.
func (g *shutdownGuard) CancelCountdown() (ShutdownState, error) {
	g.mu.Lock()
	counting := g.counting
	g.mu.Unlock()

	var cancelErr error
	if counting {
		cancelErr = g.command("/a")
	}

	g.mu.Lock()
	g.counting = false
	g.armed = false
	state := g.stateLocked("")
	g.mu.Unlock()

	if cancelErr != nil {
		// Der Befehl scheiterte — der Countdown läuft also womöglich weiter.
		// Das muss unmissverständlich im Protokoll stehen, samt dem Weg, der
		// von Hand immer funktioniert.
		g.note("Could not cancel the shutdown: " + cancelErr.Error() + " — run \"shutdown /a\" yourself.")
		g.announce(state)
		return state, cancelErr
	}
	if counting {
		g.note("Shutdown cancelled. The PC stays on.")
	}
	g.announce(state)
	return state, nil
}
