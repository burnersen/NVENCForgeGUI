// runner.go — einen Konverter-Lauf starten, mitlesen und sauber beenden.
//
// Der Datenkanal (-json) liefert auf dem Hauptkanal je Ereignis eine Zeile
// JSON; die für Menschen gedachte Anzeige läuft parallel über den Fehlerkanal
// und landet unverändert im Protokoll. Diese Trennung ist der ganze Grund,
// warum hier nichts mehr geraten werden muss.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

// maxEventLine begrenzt eine einzelne Zeile. Dateinamen können lang sein,
// deshalb großzügig — aber nicht unbegrenzt, damit eine kaputte Gegenseite den
// Speicher nicht vollschreiben kann.
const maxEventLine = 1 << 20

// ansiPattern entfernt Farb- und Cursorbefehle aus der Bildschirmausgabe.
// Im Protokollfenster wären sie unlesbarer Zeichensalat.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)

// RunState beschreibt, was gerade läuft — und wie ein Lauf ausgegangen ist.
type RunState struct {
	Running  bool   `json:"running"`
	Stopping bool   `json:"stopping"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error"`
}

// Runner überwacht genau einen Konverter-Prozess.
//
// Mehr als einer ist erst für die Mehrfach-Verarbeitung vorgesehen; bis dahin
// verhindert diese Beschränkung, dass zwei Läufe still übereinander schreiben.
type Runner struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	pid      int
	stopping bool
	emit     func(name string, data ...any)
}

// NewRunner erzeugt den Läufer. emit reicht Meldungen an die Oberfläche weiter.
func NewRunner(emit func(name string, data ...any)) *Runner {
	return &Runner{emit: emit}
}

// Running meldet, ob gerade ein Lauf aktiv ist.
func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd != nil
}

// Start startet den Konverter und beginnt, seine beiden Kanäle mitzulesen.
func (r *Runner) Start(exePath, workDir string, args []string) error {
	r.mu.Lock()
	if r.cmd != nil {
		r.mu.Unlock()
		return errors.New("a conversion is already running")
	}

	cmd := exec.Command(exePath, args...)
	cmd.Dir = workDir
	// Eigene Prozessgruppe: nur so trifft das Abbruch-Signal später genau
	// diesen Lauf. CREATE_NO_WINDOW darf hier NICHT dazu — siehe wincon.go.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("runner.go: Start (StdoutPipe): %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("runner.go: Start (StderrPipe): %w", err)
	}

	if err := cmd.Start(); err != nil {
		r.mu.Unlock()
		return fmt.Errorf("runner.go: Start (Start): %w", err)
	}

	r.cmd = cmd
	r.pid = cmd.Process.Pid
	r.stopping = false
	r.mu.Unlock()

	r.emit("conv:state", RunState{Running: true})

	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		r.readEvents(stdout)
	}()
	go func() {
		defer readers.Done()
		r.readLog(stderr)
	}()

	go func() {
		// Erst wenn beide Kanäle zu Ende gelesen sind, darf gewartet werden:
		// Wait schließt die Leitungen, und was dann noch unterwegs ist, ginge
		// verloren — ausgerechnet die letzten Zeilen sind aber die
		// interessantesten.
		readers.Wait()
		waitErr := cmd.Wait()
		r.finish(waitErr)
	}()
	return nil
}

// finish räumt auf und meldet das Ende genau einmal.
func (r *Runner) finish(waitErr error) {
	r.mu.Lock()
	wasStopping := r.stopping
	r.cmd = nil
	r.pid = 0
	r.stopping = false
	r.mu.Unlock()

	state := RunState{Running: false, Stopping: wasStopping}
	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
		// Sauber durchgelaufen.
	case errors.As(waitErr, &exitErr):
		state.ExitCode = exitErr.ExitCode()
	default:
		state.Error = waitErr.Error()
	}
	r.emit("conv:state", state)
}

// readEvents liest den Datenkanal: eine Zeile JSON je Ereignis.
//
// Was sich nicht als Ereignis lesen lässt, wird nicht verworfen, sondern ins
// Protokoll gereicht. Stiller Verlust wäre der schlechteste Ausgang: dann
// stünde die Anzeige still, ohne dass jemand den Grund sähe.
func (r *Runner) readEvents(pipe io.Reader) {
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventLine)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			r.emit("conv:log", line)
			continue
		}
		if _, ok := event["ev"]; !ok {
			r.emit("conv:log", line)
			continue
		}
		r.emit("conv:event", event)
	}
	if err := scanner.Err(); err != nil {
		r.emit("conv:log", "[event channel] "+err.Error())
	}
}

// readLog liest die Bildschirmausgabe des Konverters für das Protokollfenster.
func (r *Runner) readLog(pipe io.Reader) {
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventLine)
	scanner.Split(splitLinesAndReturns)

	for scanner.Scan() {
		line := strings.TrimRight(ansiPattern.ReplaceAllString(scanner.Text(), ""), " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		r.emit("conv:log", line)
	}
	if err := scanner.Err(); err != nil {
		r.emit("conv:log", "[log channel] "+err.Error())
	}
}

// splitLinesAndReturns trennt zusätzlich am Wagenrücklauf.
//
// Die Fortschrittsanzeige zeichnet sich immer wieder über dieselbe Zeile und
// beendet sie mit "\r" statt mit einem Zeilenumbruch. Ohne diese Trennung
// wüchse daraus eine einzige riesige Zeile, die erst am Ende des Laufs im
// Protokoll auftauchte.
func splitLinesAndReturns(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for index, character := range data {
		if character == '\n' || character == '\r' {
			return index + 1, data[:index], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// RequestStop löst den sauberen Abbruch aus — denselben, den Strg+C im
// Terminal auslöst. Ein zweiter Aufruf beendet den Konverter sofort; das ist
// sein eingebautes Verhalten und wird hier bewusst nicht nachgebaut.
func (r *Runner) RequestStop() error {
	r.mu.Lock()
	pid := r.pid
	if r.cmd == nil || pid == 0 {
		r.mu.Unlock()
		return errors.New("nothing is running")
	}
	alreadyStopping := r.stopping
	r.stopping = true
	r.mu.Unlock()

	if err := requestCleanStop(pid); err != nil {
		return err
	}
	if alreadyStopping {
		r.emit("conv:log", "[gui] Stop requested a second time — NVENCForge quits immediately.")
	} else {
		r.emit("conv:log", "[gui] Clean stop requested. The part already encoded is finalised as a playable preview.")
	}
	r.emit("conv:state", RunState{Running: true, Stopping: true})
	return nil
}
