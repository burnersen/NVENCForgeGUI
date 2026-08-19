// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// windowstate.go — Größe und Platz des Fensters über Programmstarts hinweg merken.
//
// Warum eine eigene kleine Datei NEBEN der exe und nicht die Registrierung oder
// ein Ordner unter AppData: Das Fenster soll tragbar bleiben. Im README steht
// die Zusage „nichts wird installiert, nichts in die Registrierung geschrieben —
// Ordner löschen und es ist weg". Diese Datei hält sich daran.
//
// Der heikle Teil ist nicht das Speichern, sondern das Zurückholen. Wird ein
// zweiter Bildschirm abgezogen, zeigt der gemerkte Platz ins Nichts, und das
// Fenster öffnete unsichtbar — für den Benutzer nicht von einem Absturz zu
// unterscheiden. Deshalb fragt das Programm vor dem Setzen bei Windows nach, ob
// der gemerkte Bereich überhaupt noch auf einem Bildschirm liegt.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"
)

const (
	// windowStateFileName ist die Merkdatei neben der exe.
	windowStateFileName = "NVENCForgeGUI.window"

	// Startmaße, wenn es noch nichts zu merken gab.
	defaultWindowWidth  = 1180
	defaultWindowHeight = 860

	// Unter diese Maße darf das Fenster nicht, sonst überlappen die Bereiche.
	minWindowWidth  = 920
	minWindowHeight = 640

	// maxWindowExtent fängt beschädigte Zahlen ab. Großzügig gewählt: auch
	// mehrere 4K-Bildschirme nebeneinander bleiben deutlich darunter.
	maxWindowExtent = 20000
)

// Die beiden Farbstimmungen der Oberfläche. Sie stehen hier und nicht in der
// Fensterseite, weil sie in der Merkdatei landen: ein Wert, den beide Seiten
// gleich schreiben müssen, gehört an eine Stelle.
const (
	themeDark  = "dark"
	themeLight = "light"
)

// windowState ist der gemerkte Zustand des Fensters.
type windowState struct {
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Maximised bool   `json:"maximised"`
	Theme     string `json:"theme"`
}

// defaultWindowState liefert den Zustand für den allerersten Start.
func defaultWindowState() windowState {
	return windowState{Width: defaultWindowWidth, Height: defaultWindowHeight, Theme: themeDark}
}

// normaliseTheme lässt nur die beiden bekannten Werte durch.
//
// Alles andere — leeres Feld einer alten Merkdatei, Tippfehler von Hand,
// halb geschriebene Datei — wird zur dunklen Stimmung, mit der das Fenster
// seit jeher aufgeht.
func normaliseTheme(theme string) string {
	if theme == themeLight {
		return themeLight
	}
	return themeDark
}

// sizeIsSensible prüft die gemerkten Maße, bevor ihnen jemand vertraut.
//
// Eine von Hand verstellte oder halb geschriebene Datei darf nicht dazu führen,
// dass das Fenster als Strich oder als Riese aufgeht.
func (s windowState) sizeIsSensible() bool {
	if s.Width < minWindowWidth || s.Height < minWindowHeight {
		return false
	}
	return s.Width <= maxWindowExtent && s.Height <= maxWindowExtent
}

// windowStatePath nennt den Ort der Merkdatei: der Ordner der eigenen exe.
func windowStatePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("windowstate.go: windowStatePath: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), windowStateFileName), nil
}

// loadWindowState holt den gemerkten Zustand.
//
// Der zweite Rückgabewert sagt, ob wirklich etwas Brauchbares gefunden wurde.
// Fehlt die Datei oder ist sie unlesbar, ist das kein Fehler, den jemand sehen
// müsste: Das Fenster geht dann eben in Standardgröße auf.
func loadWindowState() (windowState, bool) {
	path, err := windowStatePath()
	if err != nil {
		return defaultWindowState(), false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return defaultWindowState(), false
	}
	var state windowState
	if err := json.Unmarshal(raw, &state); err != nil {
		return defaultWindowState(), false
	}
	state.Theme = normaliseTheme(state.Theme)
	if !state.sizeIsSensible() {
		// Die Farbstimmung überlebt eine unbrauchbare Größe: Sie hat mit den
		// Maßen nichts zu tun, und sie wegzuwerfen hieße, den Nutzer nach
		// einer verunglückten Merkdatei zweimal zu bestrafen.
		fallback := defaultWindowState()
		fallback.Theme = state.Theme
		return fallback, false
	}
	return state, true
}

// saveWindowState schreibt den Zustand weg.
func saveWindowState(state windowState) error {
	path, err := windowStatePath()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("windowstate.go: saveWindowState (Marshal): %w", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("windowstate.go: saveWindowState (WriteFile): %w", err)
	}
	return nil
}

// saveTheme merkt sich die gewählte Farbstimmung, ohne alles andere anzufassen.
//
// Der Zustand wird frisch gelesen statt aus dem Gedächtnis genommen: Größe und
// Platz stehen erst beim Schließen fest, und ein hier weggeschriebener
// veralteter Wert würde sie überschreiben.
func saveTheme(theme string) error {
	state, ok := loadWindowState()
	if !ok {
		state = defaultWindowState()
	}
	state.Theme = normaliseTheme(theme)
	return saveWindowState(state)
}

// procMonitorFromRect beantwortet die Frage „liegt dieser Bereich auf einem
// Bildschirm?". user32 ist bereits in wincon.go geladen.
var procMonitorFromRect = user32.NewProc("MonitorFromRect")

// monitorDefaultToNull lässt MonitorFromRect eine 0 liefern, wenn der Bereich
// auf keinem Bildschirm liegt — genau die Auskunft, die hier gebraucht wird.
// Die anderen Werte würden ersatzweise irgendeinen Bildschirm nennen.
const monitorDefaultToNull = 0

// winRect entspricht der Windows-Struktur RECT.
type winRect struct {
	Left, Top, Right, Bottom int32
}

// rectIsOnAScreen meldet, ob der Bereich noch auf einem angeschlossenen
// Bildschirm liegt.
//
// Geprüft wird die Überschneidung mit irgendeinem Bildschirm, nicht die volle
// Sichtbarkeit: Ein Fenster, das ein Stück über den Rand ragt, ist üblich und
// lässt sich zurückziehen. Nur das vollständig verschwundene Fenster ist das
// Problem, das hier verhindert wird.
func rectIsOnAScreen(x, y, width, height int) bool {
	rect := winRect{
		Left:   int32(x),
		Top:    int32(y),
		Right:  int32(x + width),
		Bottom: int32(y + height),
	}
	handle, _, _ := procMonitorFromRect.Call(
		uintptr(unsafe.Pointer(&rect)),
		monitorDefaultToNull,
	)
	return handle != 0
}
