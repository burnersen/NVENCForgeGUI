// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// mirror_test.go — die Go-Seite der INI-Spiegelung.
//
// Geprüft wird, was im Fenster selbst nicht zu sehen ist: dass die
// Gegenschalter nur an eine Programmdatei gehen, die sie kennt, dass ein Profil
// seine Einstellungen wirklich mitbringt, und dass eine fremde Zeile in der
// Profildatei nicht in die Konfiguration des Nutzers wandert.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCounterFlagsOnlyForNewerConverters: Ohne die Auskunft "diese exe kennt
// die Gegenschalter" darf keiner davon mitgeschickt werden — eine ältere
// Programmdatei meckert bei jedem Lauf über eine unbekannte Option, und der
// Lauf sähe trotzdem normal aus.
func TestCounterFlagsOnlyForNewerConverters(t *testing.T) {
	request := RunRequest{
		Files:      []string{"film.mkv"},
		Encoder:    encoderNvidia,
		Container:  containerMKV,
		Resolution: resolutionDownscale,
		Audio:      audioAAC,
		BitDepth:   bitDepth10,
	}

	old, err := buildConverterArgs(request, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	for _, flag := range []string{"-gpu", "-mkv", "-downscale", "-aac", "-10bit", "-nokeep"} {
		if hasArg(old, flag) {
			t.Errorf("%s ging an eine exe, die es nicht kennt: %v", flag, old)
		}
	}

	request.CounterFlags = true
	fresh, err := buildConverterArgs(request, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	for _, flag := range []string{"-gpu", "-mkv", "-downscale", "-aac", "-10bit", "-nokeep"} {
		if !hasArg(fresh, flag) {
			t.Errorf("%s fehlt, obwohl die exe es kennt: %v", flag, fresh)
		}
	}
}

// TestKeepSourceKeepsItsCounterSwitch: "-keep" und "-nokeep" schließen sich
// aus. Beides zusammen zu schicken wäre widersprüchlich, und ohne das zweite
// gewönne keepSource=true aus der Datei gegen das leere Kästchen im Fenster.
func TestKeepSourceKeepsItsCounterSwitch(t *testing.T) {
	on, err := buildConverterArgs(RunRequest{
		Files:        []string{"film.mkv"},
		KeepSource:   true,
		CounterFlags: true,
	}, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if !hasArg(on, "-keep") || hasArg(on, "-nokeep") {
		t.Errorf("angehaktes Kästchen ergab %v", on)
	}

	off, err := buildConverterArgs(RunRequest{
		Files:        []string{"film.mkv"},
		CounterFlags: true,
	}, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if !hasArg(off, "-nokeep") || hasArg(off, "-keep") {
		t.Errorf("leeres Kästchen ergab %v", off)
	}
}

// TestConfigViewReadsTheBasicSettings: Die Konvertieren-Seite baut ihre
// Bedienelemente aus diesen Werten. Kommt einer nicht an, zeigt das Fenster
// wieder etwas anderes an, als der Konverter tut — genau der Fehler, wegen dem
// der Umbau überhaupt entstand.
func TestConfigViewReadsTheBasicSettings(t *testing.T) {
	content := strings.Join([]string{
		"# Kommentar",
		"codec=av1",
		"container=mp4",
		"audioMode=copy",
		"bitDepth=8",
		"keepResolution=true",
		"keepSource=true",
		"encoder=cpu",
		"autoCrop=true",
		"autoShutdown=false",
		"targetCQ=26",
	}, "\n")

	entries := settingsByKey(parseSettings(content))
	view := ConfigView{}
	view.Codec, view.CodecKnown = wordEntry(entries, "codec")
	view.Container, view.ContainerKnown = wordEntry(entries, "container")
	view.AudioMode, view.AudioModeKnown = wordEntry(entries, "audioMode")
	view.Encoder, view.EncoderKnown = wordEntry(entries, "encoder")
	view.BitDepth = intEntry(entries, "bitDepth")
	view.KeepResolution, view.KeepResolutionKnown = boolEntry(entries, "keepResolution")
	view.KeepSource, view.KeepSourceKnown = boolEntry(entries, "keepSource")
	view.AutoCrop, view.AutoCropKnown = boolEntry(entries, "autoCrop")
	view.AutoShutdown, view.AutoShutdownKnown = boolEntry(entries, "autoShutdown")

	if view.Codec != "av1" || !view.CodecKnown {
		t.Errorf("codec = %q/%v", view.Codec, view.CodecKnown)
	}
	if view.Container != "mp4" || view.AudioMode != "copy" || view.Encoder != "cpu" {
		t.Errorf("container/audio/encoder = %q/%q/%q", view.Container, view.AudioMode, view.Encoder)
	}
	if view.BitDepth != 8 {
		t.Errorf("bitDepth = %d", view.BitDepth)
	}
	if !view.KeepResolution || !view.KeepSource || !view.AutoCrop {
		t.Errorf("Schalter = %v/%v/%v", view.KeepResolution, view.KeepSource, view.AutoCrop)
	}
	// "steht auf false" ist etwas anderes als "steht gar nicht da". Ohne diese
	// Unterscheidung sähe eine fehlende Zeile aus wie ein bewusstes Nein.
	if view.AutoShutdown || !view.AutoShutdownKnown {
		t.Errorf("autoShutdown = %v (bekannt: %v)", view.AutoShutdown, view.AutoShutdownKnown)
	}
	if _, known := wordEntry(entries, "gibtEsNicht"); known {
		t.Error("ein fehlender Schlüssel gilt als bekannt")
	}
}

// TestProfileSettingsAreCleaned: Was hier durchkommt, wird beim Laden eines
// Profils in die Konfigurationsdatei des Nutzers geschrieben. Ein Zeilenumbruch
// im Wert würde die Datei zerlegen.
func TestProfileSettingsAreCleaned(t *testing.T) {
	profile := sanitiseProfile(Profile{
		Name: "Archiv",
		Settings: map[string]string{
			"targetCQ":              " 26 ",
			"":                      "ohne Namen",
			"kaputt":                "erste Zeile\nzweite Zeile",
			strings.Repeat("x", 99): "zu langer Schlüssel",
		},
	})

	if profile.Settings["targetCQ"] != "26" {
		t.Errorf("targetCQ = %q, erwartet \"26\"", profile.Settings["targetCQ"])
	}
	if len(profile.Settings) != 1 {
		t.Errorf("übrig geblieben: %v — erwartet nur targetCQ", profile.Settings)
	}
}

// TestProfileKeepsTheBlackBarTick hält den Fehler fest, den es zu beheben galt:
// Das Fenster schickte das Kästchen mit, die Struktur hatte kein Feld dafür,
// und beim Speichern verschwand es lautlos.
func TestProfileKeepsTheBlackBarTick(t *testing.T) {
	store := &profileStore{path: filepath.Join(t.TempDir(), "NVENCForgeGUI.profiles")}
	if _, err := store.Save(Profile{Name: "Archiv", Crop: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), `"crop": true`) {
		t.Errorf("das Häkchen steht nicht in der Datei: %s", raw)
	}

	reloaded := loadProfiles(store.path)
	if len(reloaded) != 1 || !reloaded[0].Crop {
		t.Errorf("nach dem Neuladen: %+v", reloaded)
	}
}

// TestWriteKnownSettingsSkipsUnknownKeys: Ein Profil kann aus einer anderen
// NVENCForge-Ausgabe stammen. Ein einziger unbekannter Schlüssel darf nicht
// dazu führen, dass gar nichts gesetzt wird — gesagt werden muss es trotzdem.
func TestWriteKnownSettingsSkipsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	original := "# Ziel-CQ\n# Allowed: 1 to 51   |   Default: 26\ntargetCQ=26\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := writeSettingsTo(path, map[string]string{"targetCQ": "30"})
	if err != nil {
		t.Fatalf("writeSettingsTo: %v", err)
	}
	if result.Written != 1 {
		t.Errorf("geschrieben: %d, erwartet 1", result.Written)
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(updated), "targetCQ=30") {
		t.Errorf("Wert nicht gesetzt: %s", updated)
	}
	// Kommentare und Reihenfolge bleiben Zeichen für Zeichen erhalten.
	if !strings.HasPrefix(string(updated), "# Ziel-CQ\n") {
		t.Errorf("der Kommentar hat gelitten: %s", updated)
	}

	// Und die Sicherung vom Programmstart liegt daneben.
	if result.SessionBackupPath == "" {
		t.Fatal("keine Sicherung vom Programmstart gemeldet")
	}
	saved, err := os.ReadFile(result.SessionBackupPath)
	if err != nil {
		t.Fatalf("Sitzungssicherung fehlt: %v", err)
	}
	if string(saved) != original {
		t.Errorf("die Sicherung hält nicht den Ausgangsstand: %s", saved)
	}
}
