package main

// runner_live_test.go — der Nachweis am echten Video.
//
// Diese Prüfungen starten den wirklichen Konverter auf einer wirklichen Datei
// und brauchen deshalb eine Grafikkarte, FFmpeg und Zeit. Sie laufen nur, wenn
// NVENCFORGEGUI_LIVE=1 gesetzt ist — sonst würden sie jeden normalen Testlauf
// minutenlang blockieren.
//
// Sie prüfen genau die zwei Dinge, die ein Schreibtisch-Test nicht kann:
// kommen die Ereignisse wirklich an, und beendet der Abbruch-Knopf den Lauf
// wirklich sauber?

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// liveCollector sammelt alles, was der Läufer meldet.
type liveCollector struct {
	mu       sync.Mutex
	events   []map[string]any
	logLines []string
	finished chan RunState
	once     sync.Once
}

func newLiveCollector() *liveCollector {
	return &liveCollector{finished: make(chan RunState, 1)}
}

func (c *liveCollector) emit(name string, data ...any) {
	if len(data) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	switch name {
	case "conv:event":
		if event, ok := data[0].(map[string]any); ok {
			c.events = append(c.events, event)
		}
	case "conv:log":
		if line, ok := data[0].(LogLine); ok && line.Text != "" {
			c.logLines = append(c.logLines, line.Text)
		}
	case "conv:state":
		if state, ok := data[0].(RunState); ok && !state.Running {
			c.once.Do(func() { c.finished <- state })
		}
	}
}

func (c *liveCollector) count(kind string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := 0
	for _, event := range c.events {
		if event["ev"] == kind {
			total++
		}
	}
	return total
}

func (c *liveCollector) dump(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	from := len(c.logLines) - 15
	if from < 0 {
		from = 0
	}
	t.Logf("last log lines:\n%s", strings.Join(c.logLines[from:], "\n"))
}

// liveSource kopiert die Quelldatei in einen Testordner.
//
// Kopiert wird bewusst: Der Konverter räumt Originale weg, und an den echten
// Testvideos darf dabei nichts passieren.
func liveSource(t *testing.T, envName string) string {
	t.Helper()
	original := os.Getenv(envName)
	if original == "" {
		t.Skipf("%s is not set", envName)
	}
	source, err := os.Open(original)
	if err != nil {
		t.Fatalf("cannot open %s: %v", original, err)
	}
	defer source.Close()

	target := filepath.Join(t.TempDir(), filepath.Base(original))
	destination, err := os.Create(target)
	if err != nil {
		t.Fatalf("cannot create %s: %v", target, err)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		t.Fatalf("cannot copy the test video: %v", err)
	}
	return target
}

// liveRunner baut den Läufer mitsamt Konverter-Pfad auf.
func liveRunner(t *testing.T) (*Runner, *liveCollector, ConverterStatus) {
	t.Helper()
	if os.Getenv("NVENCFORGEGUI_LIVE") != "1" {
		t.Skip("live check disabled (set NVENCFORGEGUI_LIVE=1)")
	}
	status := converterStatus()
	if !status.Found {
		t.Fatal("NVENCForge.exe was not found next to the test")
	}
	if !status.EventChannel {
		t.Fatal("this NVENCForge.exe has no event channel — nothing to verify")
	}
	setupHiddenConsole()

	collector := newLiveCollector()
	return NewRunner(collector.emit), collector, status
}

func TestLiveFullRun(t *testing.T) {
	runner, collector, status := liveRunner(t)
	video := liveSource(t, "NVENCFORGEGUI_LIVE_SHORT")

	args, err := buildConverterArgs(
		RunRequest{Files: []string{video}, Quality: qualityOff}, true)
	if err != nil {
		t.Fatalf("buildConverterArgs: %v", err)
	}
	if err := runner.Start(status.Path, filepath.Dir(status.Path), args); err != nil {
		t.Fatalf("Start: %v", err)
	}

	select {
	case state := <-collector.finished:
		if state.Error != "" {
			collector.dump(t)
			t.Fatalf("run failed: %s", state.Error)
		}
	case <-time.After(10 * time.Minute):
		t.Fatal("the run did not finish within 10 minutes")
	}

	for kind, minimum := range map[string]int{
		"run": 1, "file": 1, "stage": 1, "progress": 3, "result": 1, "summary": 1,
	} {
		if got := collector.count(kind); got < minimum {
			collector.dump(t)
			t.Errorf("expected at least %d %q events, got %d", minimum, kind, got)
		}
	}
}

func TestLiveCleanStop(t *testing.T) {
	runner, collector, status := liveRunner(t)
	video := liveSource(t, "NVENCFORGEGUI_LIVE_LONG")
	folder := filepath.Dir(video)

	// Ohne Auto-CQ, damit der Lauf sofort ins Encoden geht: Nur dort entsteht
	// überhaupt ein Stück, das als Vorschau gerettet werden kann.
	args, err := buildConverterArgs(
		RunRequest{Files: []string{video}, Quality: qualityOff}, true)
	if err != nil {
		t.Fatalf("buildConverterArgs: %v", err)
	}
	if err := runner.Start(status.Path, filepath.Dir(status.Path), args); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Erst abbrechen, wenn wirklich encodiert wird — vorher gäbe es nichts zu
	// retten und die Prüfung wäre wertlos.
	deadline := time.Now().Add(3 * time.Minute)
	for collector.count("progress") < 5 {
		if time.Now().After(deadline) {
			collector.dump(t)
			t.Fatal("no encoding progress arrived within 3 minutes")
		}
		time.Sleep(250 * time.Millisecond)
	}

	if err := runner.RequestStop(); err != nil {
		t.Fatalf("RequestStop: %v", err)
	}

	select {
	case <-collector.finished:
	case <-time.After(3 * time.Minute):
		t.Fatal("the run did not stop within 3 minutes")
	}

	// Gemessen 2026-08-17: Der Konverter schreibt seine Ergebnisse NICHT neben
	// die Quelle, sondern in einen Unterordner "output". Deshalb wird der ganze
	// Testordner durchsucht statt nur seine oberste Ebene.
	var found []string
	if err := filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			found = append(found, entry.Name())
		}
		return nil
	}); err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	if !hasSuffixIn(found, ".preview.mkv") {
		collector.dump(t)
		t.Errorf("no playable preview was left behind, folder holds: %v", found)
	}
	if hasSuffixIn(found, ".part.mkv") {
		t.Errorf("a torn fragment was left behind, folder holds: %v", found)
	}
	if !containsIn(found, filepath.Base(video)) {
		t.Errorf("the source file must stay untouched, folder holds: %v", found)
	}
}

func hasSuffixIn(names []string, suffix string) bool {
	for _, name := range names {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return true
		}
	}
	return false
}

func containsIn(names []string, wanted string) bool {
	for _, name := range names {
		if strings.EqualFold(name, wanted) {
			return true
		}
	}
	return false
}

// waitForEvent wartet, bis ein Ereignis der gesuchten Art angekommen ist.
//
// Kurze Wartezeiten in der Schleife statt einer festen Pause: Die Frage kommt
// je nach Datei nach einer halben oder nach zehn Sekunden, und beides soll den
// Test weder verlangsamen noch scheitern lassen.
func waitForEvent(t *testing.T, collector *liveCollector, kind string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		collector.mu.Lock()
		for _, event := range collector.events {
			if event["ev"] == kind {
				collector.mu.Unlock()
				return event
			}
		}
		collector.mu.Unlock()
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// countFiles zählt die Dateien eines Ordners, deren Name auf die Endung passt.
func countFiles(t *testing.T, dir, extension string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read %s: %v", dir, err)
	}
	found := 0
	for _, entry := range entries {
		if strings.EqualFold(filepath.Ext(entry.Name()), extension) {
			found++
		}
	}
	return found
}

// TestLiveTrackQuestionIsAnswered prüft die ganze Kette an einem echten Video
// mit mehreren Tonspuren: Der Konverter kündigt die Auswahl an, das Fenster
// antwortet über die Eingabeleitung, und die Antwort wirkt sich wirklich aus.
//
// Der letzte Punkt ist der wichtige. Ein Dialog, der sich richtig anfühlt, aber
// dessen Auswahl nirgends ankommt, wäre der schlimmste Fall: Man merkt es erst
// an den fehlenden Dateien.
func TestLiveTrackQuestionIsAnswered(t *testing.T) {
	runner, collector, status := liveRunner(t)
	video := liveSource(t, "NVENCFORGEGUI_LIVE_TRACKS")
	workDir := filepath.Dir(video)

	if err := runner.Start(status.Path, filepath.Dir(status.Path),
		[]string{"-json", "-davinci", video}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	question := waitForEvent(t, collector, "question", 2*time.Minute)
	if question == nil {
		collector.dump(t)
		t.Fatal("no question event arrived — does this NVENCForge.exe know it?")
	}
	options, _ := question["options"].([]any)
	if len(options) < 2 {
		t.Fatalf("a question with %d options is not a choice", len(options))
	}
	if err := runner.Answer("1"); err != nil {
		t.Fatalf("Answer: %v", err)
	}

	select {
	case state := <-collector.finished:
		if state.Error != "" {
			collector.dump(t)
			t.Fatalf("run failed: %s", state.Error)
		}
	case <-time.After(5 * time.Minute):
		t.Fatal("the run did not finish within 5 minutes")
	}

	// Genau eine Tonspur war gewählt, also darf auch nur eine Tondatei
	// dastehen. Ohne wirksame Antwort wären es alle.
	if got := countFiles(t, workDir, ".m4a"); got != 1 {
		t.Errorf("expected exactly 1 audio file for the answer \"1\", found %d", got)
	}
}

// TestLiveStopReleasesAnOpenQuestion sichert den Fall ab, der sonst ein
// hängendes Programm bedeutet: Es steht eine Frage offen, niemand antwortet,
// und der Nutzer drückt Abbrechen. Das Abbruch-Signal allein erreicht die
// wartende Lesestelle nicht — erst das Schließen der Eingabeleitung löst sie.
func TestLiveStopReleasesAnOpenQuestion(t *testing.T) {
	runner, collector, status := liveRunner(t)
	video := liveSource(t, "NVENCFORGEGUI_LIVE_TRACKS")

	if err := runner.Start(status.Path, filepath.Dir(status.Path),
		[]string{"-json", "-davinci", video}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if question := waitForEvent(t, collector, "question", 2*time.Minute); question == nil {
		collector.dump(t)
		t.Fatal("no question event arrived")
	}

	if err := runner.RequestStop(); err != nil {
		t.Fatalf("RequestStop: %v", err)
	}
	select {
	case <-collector.finished:
		// Gut: Der Lauf hat sich gelöst.
	case <-time.After(2 * time.Minute):
		collector.dump(t)
		t.Fatal("the run stayed stuck on the open question after Stop")
	}
}
