// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// datadir_test.go — der Umzug der Merkdateien in den tools-Ordner.
//
// Das ist die einzige Stelle des Fensters, an der ein Fehler eine Datei des
// Nutzers kosten könnte: Wer hier die alte Datei wegwirft, bevor die neue
// steht, löscht eine Sparbilanz, die über Monate gewachsen ist.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

const testFileName = "NVENCForgeGUI.savings"

func TestNewInstallUsesTheToolsFolder(t *testing.T) {
	home := t.TempDir()

	path := dataFilePathIn(home, testFileName)

	if path != filepath.Join(home, dataDirName, testFileName) {
		t.Errorf("neue Ablage in %q, erwartet im Ordner %q", path, dataDirName)
	}
	if _, err := os.Stat(filepath.Join(home, dataDirName)); err != nil {
		t.Errorf("der Ordner wurde nicht angelegt: %v", err)
	}
}

// TestOldFileMovesAlong ist der eigentliche Zweck: Wer von 0.9.4 kommt, hat
// seine drei Dateien neben der exe. Sie müssen mitkommen, nicht neu anfangen.
func TestOldFileMovesAlong(t *testing.T) {
	home := t.TempDir()
	beside := filepath.Join(home, testFileName)
	if err := os.WriteFile(beside, []byte("alte Bilanz"), 0o644); err != nil {
		t.Fatalf("Testdatei schreiben: %v", err)
	}

	path := dataFilePathIn(home, testFileName)

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("die umgezogene Datei ist nicht lesbar: %v", err)
	}
	if string(content) != "alte Bilanz" {
		t.Errorf("Inhalt %q — der Umzug hat den Inhalt verändert", content)
	}
	if fileExists(beside) {
		t.Error("die alte Datei liegt noch neben der exe — dann sieht der Ordner aus wie vorher")
	}
}

// TestExistingNewFileWins hält den Fall fest, dass beide Dateien da sind (etwa
// weil noch eine alte Fassung des Fensters gestartet wurde). Die neue Ablage
// ist die gültige; die alte wird NICHT darübergeschrieben und nicht gelöscht.
func TestExistingNewFileWins(t *testing.T) {
	home := t.TempDir()
	beside := filepath.Join(home, testFileName)
	inside := filepath.Join(home, dataDirName, testFileName)
	if err := os.MkdirAll(filepath.Join(home, dataDirName), 0o755); err != nil {
		t.Fatalf("Ordner anlegen: %v", err)
	}
	if err := os.WriteFile(beside, []byte("alt"), 0o644); err != nil {
		t.Fatalf("Testdatei schreiben: %v", err)
	}
	if err := os.WriteFile(inside, []byte("neu"), 0o644); err != nil {
		t.Fatalf("Testdatei schreiben: %v", err)
	}

	path := dataFilePathIn(home, testFileName)

	if path != inside {
		t.Errorf("benutzt wird %q, erwartet die neue Ablage %q", path, inside)
	}
	content, err := os.ReadFile(inside)
	if err != nil || string(content) != "neu" {
		t.Errorf("die neue Datei wurde überschrieben: %q (%v)", content, err)
	}
	if !fileExists(beside) {
		t.Error("die alte Datei wurde gelöscht — hier wird nichts weggeworfen")
	}
}

// TestUnwritableHomeFallsBackNextToTheExe: Ein Ordner, der sich nicht anlegen
// lässt, darf das Fenster nicht aufhalten. Dann liegt die Datei eben wie
// früher neben der exe.
func TestUnwritableHomeFallsBackNextToTheExe(t *testing.T) {
	home := t.TempDir()
	// Eine DATEI namens "tools" blockiert den Ordner mit demselben Namen —
	// der billigste Weg, ein fehlschlagendes MkdirAll nachzustellen.
	if err := os.WriteFile(filepath.Join(home, dataDirName), []byte("kein Ordner"), 0o644); err != nil {
		t.Fatalf("Sperrdatei schreiben: %v", err)
	}

	path := dataFilePathIn(home, testFileName)

	if path != filepath.Join(home, testFileName) {
		t.Errorf("Ausweichort %q, erwartet neben der exe in %q", path, home)
	}
}
