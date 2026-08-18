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

// writeFile legt eine Testdatei mit etwas Inhalt an.
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestIsVideoFile(t *testing.T) {
	yes := []string{"a.mkv", "A.MP4", "clip.m2ts", "x.webm"}
	no := []string{"a.txt", "a.mkv.txt", "a", "a.jpg"}
	for _, name := range yes {
		if !isVideoFile(name) {
			t.Errorf("%q should count as a video", name)
		}
	}
	for _, name := range no {
		if isVideoFile(name) {
			t.Errorf("%q should not count as a video", name)
		}
	}
}

func TestLooksConverted(t *testing.T) {
	converted := []string{"film.h265.mkv", "film.AV1.mkv", "film.preview.mkv", "film.h265.part.mkv"}
	untouched := []string{"film.mkv", "h265.mkv", "my.h265 movie.mkv"}
	for _, name := range converted {
		if !looksConverted(name) {
			t.Errorf("%q is one of our own results", name)
		}
	}
	for _, name := range untouched {
		if looksConverted(name) {
			t.Errorf("%q is a normal source file", name)
		}
	}
}

func TestFolderScanSkipsOwnResults(t *testing.T) {
	// Ohne diese Regel stünde nach dem ersten Lauf jedes Ergebnis wieder in der
	// Warteschlange und würde ein zweites Mal umgerechnet.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "movie.mkv"))
	writeFile(t, filepath.Join(dir, "movie.h265.mkv"))
	writeFile(t, filepath.Join(dir, "notes.txt"))
	writeFile(t, filepath.Join(dir, "sub", "clip.mp4"))

	items, _ := expandPaths([]string{dir})
	var names []string
	for _, item := range items {
		names = append(names, item.Name)
	}
	joinedNames := strings.Join(names, ",")

	if len(items) != 2 {
		t.Fatalf("expected 2 files, got %d (%s)", len(items), joinedNames)
	}
	if !strings.Contains(joinedNames, "movie.mkv") || !strings.Contains(joinedNames, "clip.mp4") {
		t.Errorf("wrong files collected: %s", joinedNames)
	}
	if strings.Contains(joinedNames, "movie.h265.mkv") {
		t.Errorf("an earlier result was collected again: %s", joinedNames)
	}
}

func TestExplicitFileIsAlwaysKept(t *testing.T) {
	// Wer eine Datei bewusst hineinzieht, muss sie in der Liste wiederfinden —
	// auch wenn sie wie ein früheres Ergebnis aussieht.
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.h265.mkv")
	writeFile(t, path)

	items, _ := expandPaths([]string{path})
	if len(items) != 1 {
		t.Fatalf("an explicitly added file must stay, got %d entries", len(items))
	}
}

func TestDuplicatesAreDroppedOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.mkv")
	writeFile(t, path)

	items, _ := expandPaths([]string{path, path, dir})
	if len(items) != 1 {
		t.Fatalf("the same file must appear once, got %d entries", len(items))
	}
}

func TestVideoFilterPatternCoversEveryExtension(t *testing.T) {
	pattern := videoFilterPattern()
	for _, extension := range videoExtensions {
		if !strings.Contains(pattern, "*"+extension) {
			t.Errorf("file dialog pattern is missing %q: %s", extension, pattern)
		}
	}
}

// TestExpandPathsReportsRejectedFiles: Eine abgelegte Ton- oder Untertiteldatei
// verschwand früher wortlos aus der Warteschlange. Genau das tut jemand, der
// das Zusammenfügen sucht — er soll erfahren, dass die Datei hier nichts
// verloren hat, statt sie sich auflösen zu sehen.
func TestExpandPathsReportsRejectedFiles(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "film.mkv")
	audio := filepath.Join(dir, "film.ger.ac3")
	subs := filepath.Join(dir, "film.ger.srt")
	for _, path := range []string{video, audio, subs} {
		writeFile(t, path)
	}

	items, rejected := expandPaths([]string{video, audio, subs})
	if len(items) != 1 {
		t.Fatalf("%d Dateien in der Warteschlange, erwartet 1: %+v", len(items), items)
	}
	if len(rejected) != 2 {
		t.Fatalf("%d abgelehnte gemeldet, erwartet 2: %v", len(rejected), rejected)
	}
	for _, name := range rejected {
		if name != "film.ger.ac3" && name != "film.ger.srt" {
			t.Errorf("unerwartete Meldung: %q", name)
		}
	}
}

// Ein durchsuchter ORDNER meldet nichts: Dort ist Fremdmaterial normal, und
// eine Zeile je Datei würde das Protokoll fluten.
func TestExpandPathsStaysQuietForFolders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "film.mkv"))
	writeFile(t, filepath.Join(dir, "notiz.txt"))
	writeFile(t, filepath.Join(dir, "bild.jpg"))

	items, rejected := expandPaths([]string{dir})
	if len(items) != 1 {
		t.Fatalf("%d Dateien gefunden, erwartet 1", len(items))
	}
	if len(rejected) != 0 {
		t.Errorf("aus einem Ordner darf nichts gemeldet werden, bekommen: %v", rejected)
	}
}
