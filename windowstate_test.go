package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSizeIsSensible(t *testing.T) {
	cases := []struct {
		name  string
		state windowState
		want  bool
	}{
		{"normale Größe", windowState{Width: 1180, Height: 860}, true},
		{"genau das Mindestmaß", windowState{Width: minWindowWidth, Height: minWindowHeight}, true},
		{"zu schmal", windowState{Width: minWindowWidth - 1, Height: 860}, false},
		{"zu flach", windowState{Width: 1180, Height: minWindowHeight - 1}, false},
		{"leere Datei", windowState{}, false},
		{"negativ", windowState{Width: -1180, Height: -860}, false},
		{"unsinnig groß", windowState{Width: maxWindowExtent + 1, Height: 860}, false},
	}
	for _, c := range cases {
		if got := c.state.sizeIsSensible(); got != c.want {
			t.Errorf("%s: sizeIsSensible() = %v, erwartet %v", c.name, got, c.want)
		}
	}
}

// writeStateFile legt eine Merkdatei mit vorgegebenem Inhalt an und räumt sie
// nach dem Test wieder weg. Sie landet neben der Testdatei, also in dem
// Wegwerf-Ordner, den "go test" ohnehin anlegt.
func writeStateFile(t *testing.T, content string) string {
	t.Helper()
	path, err := windowStatePath()
	if err != nil {
		t.Fatalf("windowStatePath: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func TestLoadWindowStateRejectsRubbish(t *testing.T) {
	writeStateFile(t, "das ist kein JSON")

	state, ok := loadWindowState()
	if ok {
		t.Error("beschädigte Datei wurde als brauchbar gemeldet")
	}
	if state != defaultWindowState() {
		t.Errorf("erwartet wurden die Standardmaße, bekommen: %+v", state)
	}
}

func TestLoadWindowStateRejectsTinySize(t *testing.T) {
	writeStateFile(t, `{"width":10,"height":10,"x":100,"y":100}`)

	if _, ok := loadWindowState(); ok {
		t.Error("ein Fenster von 10x10 darf nicht angenommen werden")
	}
}

func TestSaveAndLoadWindowStateRoundTrip(t *testing.T) {
	path, err := windowStatePath()
	if err != nil {
		t.Fatalf("windowStatePath: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	want := windowState{Width: 1400, Height: 900, X: 120, Y: 60, Maximised: true}
	if err := saveWindowState(want); err != nil {
		t.Fatalf("saveWindowState: %v", err)
	}

	got, ok := loadWindowState()
	if !ok {
		t.Fatal("gerade geschriebener Zustand wurde nicht angenommen")
	}
	if got != want {
		t.Errorf("zurückgelesen %+v, geschrieben war %+v", got, want)
	}
}

func TestLoadWindowStateWithoutFile(t *testing.T) {
	path, err := windowStatePath()
	if err != nil {
		t.Fatalf("windowStatePath: %v", err)
	}
	_ = os.Remove(path)

	state, ok := loadWindowState()
	if ok {
		t.Error("ohne Datei darf nichts als gemerkt gelten")
	}
	if state != defaultWindowState() {
		t.Errorf("erwartet wurden die Standardmaße, bekommen: %+v", state)
	}
}

func TestWindowStateFileSitsNextToTheExe(t *testing.T) {
	path, err := windowStatePath()
	if err != nil {
		t.Fatalf("windowStatePath: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if filepath.Dir(path) != filepath.Dir(exe) {
		t.Errorf("Merkdatei liegt in %q, erwartet neben der exe in %q",
			filepath.Dir(path), filepath.Dir(exe))
	}
}

// TestRectOffAllScreensIsRejected hält fest, worauf sich restoreWindowPlace
// verlässt: Ein Bereich weit außerhalb jedes Bildschirms muss abgelehnt werden,
// sonst öffnet das Fenster nach dem Abziehen eines Bildschirms unsichtbar.
func TestRectOffAllScreensIsRejected(t *testing.T) {
	if rectIsOnAScreen(-90000, -90000, 1180, 860) {
		t.Error("ein Platz weit außerhalb aller Bildschirme wurde angenommen")
	}
	if !rectIsOnAScreen(0, 0, 1180, 860) {
		t.Error("die linke obere Ecke des Hauptbildschirms wurde abgelehnt")
	}
}

// TestVersionMatchesWailsConfig verhindert das Auseinanderlaufen der zwei
// Stellen, an denen die Version steht: guiVersion im Code bestimmt, was das
// Fenster anzeigt, productVersion in wails.json bestimmt, was Windows in den
// Dateieigenschaften zeigt. Wer nur eine erhöht, merkt es hier.
func TestVersionMatchesWailsConfig(t *testing.T) {
	raw, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("wails.json lesen: %v", err)
	}
	var config struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatalf("wails.json auswerten: %v", err)
	}
	if config.Info.ProductVersion != guiVersion {
		t.Errorf("wails.json sagt %q, guiVersion sagt %q — beide müssen gleich sein",
			config.Info.ProductVersion, guiVersion)
	}
	if strings.TrimSpace(guiVersion) == "" {
		t.Error("guiVersion ist leer")
	}
}
