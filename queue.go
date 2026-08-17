// queue.go — aus hineingezogenen Dateien und Ordnern wird die Warteschlange.
//
// Die Listen hier spiegeln bewusst die des Konverters (main.go:83
// videoExtensions und main.go:92 skipInputSuffixes). Zwei getrennte Programme
// können sich keine Liste teilen; wird sie dort erweitert, gehört sie hier
// nachgezogen. Die Entscheidung, ob eine Datei wirklich bearbeitet wird, trifft
// weiterhin ausschließlich der Konverter — überspringt er eine, meldet er das
// als Ergebnis "skipped", und genau das zeigt die Oberfläche dann an.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// videoExtensions sind die Endungen, die der Konverter als Video ansieht.
// Die Reihenfolge spielt nur für den Dateidialog eine Rolle.
var videoExtensions = []string{
	".mp4", ".mkv", ".ts", ".avi", ".mov", ".flv",
	".wmv", ".webm", ".m4v", ".mts", ".m2ts",
}

// convertedSuffixes markieren Dateien, die der Konverter selbst erzeugt hat.
// Beim Durchsuchen eines Ordners bleiben sie außen vor — sonst stünde nach dem
// ersten Lauf jedes Ergebnis wieder in der Warteschlange.
var convertedSuffixes = []string{".h265", ".h264", ".remux", ".preview", ".av1", ".part"}

// QueueItem ist eine Zeile in der Warteschlange.
type QueueItem struct {
	Path    string  `json:"path"`
	Name    string  `json:"name"`
	Folder  string  `json:"folder"`
	SizeMB  float64 `json:"sizeMB"`
	Missing bool    `json:"missing"`
}

// expandPaths macht aus dem, was jemand ins Fenster gezogen hat, eine flache
// Dateiliste. Ordner werden vollständig durchsucht — genauso wie der Konverter
// es täte, wenn man ihm den Ordner übergibt.
//
// Ausdrücklich hineingezogene EINZELDATEIEN bleiben immer stehen, auch wenn sie
// wie ein früheres Ergebnis aussehen: Wer eine Datei bewusst hinzufügt, soll
// sie in der Liste wiederfinden und nicht rätseln, wo sie geblieben ist.
func expandPaths(paths []string) []QueueItem {
	var items []QueueItem
	seen := make(map[string]bool)

	appendFile := func(path string) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			absolute = path
		}
		key := strings.ToLower(absolute)
		if seen[key] {
			return
		}
		seen[key] = true
		items = append(items, newQueueItem(absolute))
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			appendFile(path)
			continue
		}
		if !info.IsDir() {
			if isVideoFile(path) {
				appendFile(path)
			}
			continue
		}
		for _, found := range scanFolder(path) {
			appendFile(found)
		}
	}
	return items
}

// scanFolder sammelt alle Videodateien eines Ordners samt Unterordnern.
func scanFolder(root string) []string {
	var found []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		// Ein unlesbarer Unterordner (fehlende Rechte) darf den Rest des
		// Durchlaufs nicht abbrechen.
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if isVideoFile(path) && !looksConverted(path) {
			found = append(found, path)
		}
		return nil
	})
	sort.Slice(found, func(a, b int) bool {
		return strings.ToLower(found[a]) < strings.ToLower(found[b])
	})
	return found
}

// newQueueItem liest die Angaben, die in der Liste stehen sollen.
func newQueueItem(path string) QueueItem {
	item := QueueItem{
		Path:   path,
		Name:   filepath.Base(path),
		Folder: filepath.Dir(path),
	}
	info, err := os.Stat(path)
	if err != nil {
		item.Missing = true
		return item
	}
	item.SizeMB = float64(info.Size()) / 1024 / 1024
	return item
}

// isVideoFile prüft nur die Endung — genau wie der Konverter beim Einsammeln.
func isVideoFile(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	for _, known := range videoExtensions {
		if known == extension {
			return true
		}
	}
	return false
}

// videoFilterPattern baut das Muster für den Dateidialog, z. B. "*.mp4;*.mkv".
func videoFilterPattern() string {
	patterns := make([]string, 0, len(videoExtensions))
	for _, extension := range videoExtensions {
		patterns = append(patterns, "*"+extension)
	}
	return strings.Join(patterns, ";")
}

// looksConverted erkennt die eigenen Ergebnisse des Konverters an ihrem
// Zwischen-Namensteil, zum Beispiel "film.h265.mkv" oder "film.part.mkv".
func looksConverted(path string) bool {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	lower := strings.ToLower(base)
	for _, suffix := range convertedSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
