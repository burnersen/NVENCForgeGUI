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

	items := expandPaths([]string{dir})
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

	items := expandPaths([]string{path})
	if len(items) != 1 {
		t.Fatalf("an explicitly added file must stay, got %d entries", len(items))
	}
}

func TestDuplicatesAreDroppedOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.mkv")
	writeFile(t, path)

	items := expandPaths([]string{path, path, dir})
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
