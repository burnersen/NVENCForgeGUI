// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNeedsSetupSeesAMissingIni: Frisch heruntergeladen fehlt dem Konverter
// seine INI — genau daran erkennt das Fenster, dass er einmal laufen muss,
// bevor die Einstellungsseite etwas anzuzeigen hat.
func TestNeedsSetupSeesAMissingIni(t *testing.T) {
	dir := t.TempDir()
	status := ConverterStatus{
		Found:         true,
		ToolsDir:      dir,
		Path:          filepath.Join(dir, "NVENCForge.exe"),
		FFmpegPresent: true,
	}

	if !needsSetup(status) {
		t.Error("ohne INI muss die Einrichtung anstehen")
	}

	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte("[x]\n"), 0o644); err != nil {
		t.Fatalf("Test-INI: %v", err)
	}
	if needsSetup(status) {
		t.Error("mit INI und FFmpeg ist nichts mehr zu tun")
	}

	// Ohne eigenes FFmpeg fehlt ebenfalls etwas: Der erste Lauf bliebe sonst
	// stehen, um es herunterzuladen.
	status.FFmpegPresent = false
	if !needsSetup(status) {
		t.Error("ohne FFmpeg muss die Einrichtung anstehen")
	}

	// Ist gar keine Programmdatei da, gibt es nichts einzurichten — dann ist
	// erst der Download dran.
	if needsSetup(ConverterStatus{Found: false, ToolsDir: dir}) {
		t.Error("ohne NVENCForge.exe darf nichts angeboten werden")
	}
}

// TestRunSetupRefusesWithoutConverter: Ohne Programmdatei muss der Aufruf klar
// scheitern, statt still nichts zu tun.
func TestRunSetupRefusesWithoutConverter(t *testing.T) {
	if err := runSetup(ConverterStatus{Found: false}, func(string) {}); err == nil {
		t.Error("ohne NVENCForge.exe muss runSetup einen Fehler melden")
	}
}

// TestRunConverterIdleReportsAFailedStartAndCleansUp deckt den Protokoll-Zweig
// des gemeinsamen Anstoßweges ab: Lässt sich das Programm nicht starten, muss
// das als Fehler zurückkommen (die Erstausstattung entscheidet daran, ob sie
// gescheitert ist) und der angelegte Arbeitsordner darf nicht liegenbleiben.
func TestRunConverterIdleReportsAFailedStartAndCleansUp(t *testing.T) {
	countIdleDirs := func() int {
		entries, err := os.ReadDir(os.TempDir())
		if err != nil {
			t.Fatalf("temp directory not readable: %v", err)
		}
		found := 0
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "NVENCForgeGUI_idle_") {
				found++
			}
		}
		return found
	}

	before := countIdleDirs()
	lines := 0
	err := runConverterIdle(context.Background(),
		filepath.Join(t.TempDir(), "does-not-exist.exe"),
		func(string) { lines++ })
	if err == nil {
		t.Error("a converter that cannot be started must be reported as an error")
	}
	if lines != 0 {
		t.Errorf("nothing can be logged from a process that never ran, got %d lines", lines)
	}
	if after := countIdleDirs(); after != before {
		t.Errorf("work directory was left behind: %d idle folders before, %d after", before, after)
	}
}
