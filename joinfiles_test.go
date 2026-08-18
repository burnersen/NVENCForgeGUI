// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kindOf sucht die Einordnung einer Datei in der fertigen Ablage.
func kindOf(files []JoinFile, name string) string {
	for _, file := range files {
		if strings.EqualFold(file.Name, name) {
			return file.Kind
		}
	}
	return "not in the list"
}

// TestJoinSortsByExtension: Die Ablage entscheidet allein an der Endung, was
// Bild, Ton und Untertitel ist. Läge hier eine Endung falsch, zeigte die
// Oberfläche eine Gruppe an, die der Konverter anders sieht — und der Lauf
// bräche mit "Unknown file types" ab, obwohl das Fenster grünes Licht gab.
func TestJoinSortsByExtension(t *testing.T) {
	files := classifyJoinFiles([]string{
		`C:\v\film.NoSound.mkv`,
		`C:\v\film.mp4`,
		`C:\v\film.ger.m4a`,
		`C:\v\film.eng.ac3`,
		`C:\v\film.thd`,
		`C:\v\film.ger.srt`,
		`C:\v\film.eng.sup`,
		`C:\v\notiz.txt`,
	})

	expected := map[string]string{
		"film.NoSound.mkv": joinKindVideo,
		"film.mp4":         joinKindVideo,
		"film.ger.m4a":     joinKindAudio,
		"film.eng.ac3":     joinKindAudio,
		"film.thd":         joinKindAudio,
		"film.ger.srt":     joinKindSubtitle,
		"film.eng.sup":     joinKindSubtitle,
		"notiz.txt":        joinKindUnusable,
	}
	for name, want := range expected {
		if got := kindOf(files, name); got != want {
			t.Errorf("%s: erwartet %q, bekommen %q", name, want, got)
		}
	}
}

// TestJoinRefusesVideoTypesTheConverterCannotUse: Die Warteschlange der
// Konvertier-Seite kennt elf Video-Endungen, der Konverter nimmt als
// Bild-Grundlage aber nur vier. Würde die Join-Ablage einfach dieselbe Liste
// benutzen, sähe eine .ts-Datei wie ein gültiges Bild aus.
func TestJoinRefusesVideoTypesTheConverterCannotUse(t *testing.T) {
	files := classifyJoinFiles([]string{`C:\v\film.ts`, `C:\v\film.avi`, `C:\v\film.webm`})
	for _, file := range files {
		if file.Kind != joinKindUnusable {
			t.Errorf("%s: erwartet %q, bekommen %q", file.Name, joinKindUnusable, file.Kind)
		}
	}
}

// TestSubNeedsItsIdx bildet die Regel des Konverters ab: Eine .sub allein
// lässt den ganzen Lauf abbrechen; zusammen mit ihrer .idx wird sie still
// mitgenommen. Beides muss die Ablage unterscheiden, sonst verspricht sie
// einen Lauf, der sofort scheitert.
func TestSubNeedsItsIdx(t *testing.T) {
	alone := classifyJoinFiles([]string{`C:\v\film.NoSound.mkv`, `C:\v\film.ger.sub`})
	if got := kindOf(alone, "film.ger.sub"); got != joinKindUnusable {
		t.Errorf("ohne .idx erwartet %q, bekommen %q", joinKindUnusable, got)
	}

	together := classifyJoinFiles([]string{
		`C:\v\film.NoSound.mkv`, `C:\v\film.ger.sub`, `C:\v\film.ger.idx`,
	})
	if got := kindOf(together, "film.ger.sub"); got != joinKindCompanion {
		t.Errorf("mit .idx erwartet %q, bekommen %q", joinKindCompanion, got)
	}
	if got := kindOf(together, "film.ger.idx"); got != joinKindSubtitle {
		t.Errorf(".idx erwartet %q, bekommen %q", joinKindSubtitle, got)
	}

	// Die Reihenfolge des Ablegens darf keine Rolle spielen: Wer erst die .sub
	// und dann die .idx hineinzieht, bekommt dasselbe Ergebnis.
	reversed := classifyJoinFiles([]string{
		`C:\v\film.ger.idx`, `C:\v\film.ger.sub`, `C:\v\film.NoSound.mkv`,
	})
	if got := kindOf(reversed, "film.ger.sub"); got != joinKindCompanion {
		t.Errorf("umgekehrt abgelegt erwartet %q, bekommen %q", joinKindCompanion, got)
	}
}

// TestJoinArgOrder: Der Konverter bekommt die Bild-Grundlage zuerst, danach
// Ton, danach Untertitel — und die .sub gar nicht (FFmpeg liest sie selbst
// neben ihrer .idx).
func TestJoinArgOrder(t *testing.T) {
	args, err := joinArgOrder([]string{
		`C:\v\film.ger.srt`,
		`C:\v\film.ger.sub`,
		`C:\v\film.ger.idx`,
		`C:\v\film.ger.m4a`,
		`C:\v\film.NoSound.mkv`,
	})
	if err != nil {
		t.Fatalf("joinArgOrder: %v", err)
	}
	if len(args) != 4 {
		t.Fatalf("erwartet 4 Argumente (Bild, Ton, 2 Untertitel), bekommen %v", args)
	}
	if filepath.Base(args[0]) != "film.NoSound.mkv" {
		t.Errorf("das Bild muss zuerst kommen, bekommen %s", args[0])
	}
	if filepath.Base(args[1]) != "film.ger.m4a" {
		t.Errorf("danach der Ton, bekommen %s", args[1])
	}
	for _, arg := range args {
		if strings.HasSuffix(strings.ToLower(arg), ".sub") {
			t.Errorf("die .sub darf nicht übergeben werden: %s", arg)
		}
	}
}

// TestJoinArgOrderRefusesImpossibleRuns: Der Konverter meldet eine falsche
// Zusammenstellung NICHT im Datenkanal — gemessen am 2026-08-18 schickt er nur
// sein "run"-Ereignis und endet wortlos mit Rückgabewert 0. Das Fenster wartete
// dann auf eine Zusammenfassung, die nie kommt. Deshalb wird hier abgelehnt.
func TestJoinArgOrderRefusesImpossibleRuns(t *testing.T) {
	cases := map[string][]string{
		"ohne Bild":            {`C:\v\film.ger.m4a`},
		"zwei Bilder":          {`C:\v\a.mkv`, `C:\v\b.mkv`, `C:\v\film.ger.m4a`},
		"Bild ohne Beigabe":    {`C:\v\film.NoSound.mkv`},
		"unbrauchbare Datei":   {`C:\v\film.NoSound.mkv`, `C:\v\film.ger.m4a`, `C:\v\notiz.txt`},
		"verwaiste .sub-Datei": {`C:\v\film.NoSound.mkv`, `C:\v\film.ger.sub`},
	}
	for name, files := range cases {
		if _, err := joinArgOrder(files); err == nil {
			t.Errorf("%s: muss abgelehnt werden, wurde aber angenommen", name)
		}
	}
}

// TestJoinArgOrderAcceptsSubtitleOnly: Nur Untertitel ist ausdrücklich erlaubt
// — dann behält die Bild-Grundlage ihren eigenen Ton (Konverter seit v1.3.4).
func TestJoinArgOrderAcceptsSubtitleOnly(t *testing.T) {
	if _, err := joinArgOrder([]string{`C:\v\film.mkv`, `C:\v\film.ger.srt`}); err != nil {
		t.Errorf("nur Untertitel muss erlaubt sein: %v", err)
	}
}

// TestJoinFolderIsReadOneLevelDeep: Ein hineingezogener Ordner wird eine Ebene
// tief gelesen — die Teile eines zerlegten Films liegen immer nebeneinander.
// Unterordner bleiben außen vor, sonst sammelte ein Filmarchiv hunderte
// Dateien ein, aus denen niemand mehr eine Bild-Grundlage heraussucht.
func TestJoinFolderIsReadOneLevelDeep(t *testing.T) {
	folder := t.TempDir()
	deeper := filepath.Join(folder, "unterordner")
	if err := os.Mkdir(deeper, 0o755); err != nil {
		t.Fatalf("Testordner: %v", err)
	}
	for path, content := range map[string]string{
		filepath.Join(folder, "film.NoSound.mkv"): "bild",
		filepath.Join(folder, "film.ger.m4a"):     "ton",
		filepath.Join(deeper, "film.eng.m4a"):     "tiefer",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("Testdatei: %v", err)
		}
	}

	files := classifyJoinFiles([]string{folder})
	if len(files) != 2 {
		t.Fatalf("erwartet 2 Dateien aus der obersten Ebene, bekommen %d", len(files))
	}
	if kindOf(files, "film.eng.m4a") != "not in the list" {
		t.Error("die Datei aus dem Unterordner darf nicht mitkommen")
	}
}

// TestJoinListHasNoDuplicates: Dieselbe Datei zweimal übergeben (einmal einzeln,
// einmal über ihren Ordner) darf sie nicht doppelt in die Liste bringen — sonst
// bekäme der Konverter dieselbe Tonspur zweimal.
func TestJoinListHasNoDuplicates(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "film.ger.m4a")
	if err := os.WriteFile(path, []byte("ton"), 0o644); err != nil {
		t.Fatalf("Testdatei: %v", err)
	}
	files := classifyJoinFiles([]string{path, folder, path})
	if len(files) != 1 {
		t.Fatalf("erwartet 1 Eintrag, bekommen %d", len(files))
	}
}

// TestJoinMarksMissingFiles: Eine inzwischen verschobene Datei muss sichtbar
// bleiben und als fehlend markiert sein, statt kommentarlos zu verschwinden.
func TestJoinMarksMissingFiles(t *testing.T) {
	files := classifyJoinFiles([]string{`C:\gibt\es\nicht\film.ger.m4a`})
	if len(files) != 1 {
		t.Fatalf("erwartet 1 Eintrag, bekommen %d", len(files))
	}
	if !files[0].Missing {
		t.Error("die verschwundene Datei muss als fehlend markiert sein")
	}
	if files[0].Kind != joinKindAudio {
		t.Errorf("die Einordnung hängt nur an der Endung, bekommen %q", files[0].Kind)
	}
}
