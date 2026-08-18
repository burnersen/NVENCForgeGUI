package main

import (
	"os"
	"path/filepath"
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
