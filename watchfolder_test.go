// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// writeWatchFile legt eine Datei mit der gewünschten Größe an.
func writeWatchFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("Testordner: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("Testdatei: %v", err)
	}
}

// newTestWatcher baut einen Beobachter, der auf den Ordner zeigt, ohne den
// Hintergrund-Ablauf zu starten — geprüft wird der einzelne Durchgang.
func newTestWatcher(folder string) *FolderWatcher {
	watcher := NewFolderWatcher(func([]QueueItem) {}, func(string) {})
	watcher.folder = folder
	return watcher
}

func names(items []QueueItem) map[string]bool {
	found := make(map[string]bool)
	for _, item := range items {
		found[item.Name] = true
	}
	return found
}

// TestWatchWaitsUntilTheFileIsQuiet ist der Kern: Eine Datei, die noch wächst,
// darf nicht gemeldet werden. Würde sie es, liefe der Konverter auf einen halben
// Download — und das Ergebnis sähe fertig aus.
func TestWatchWaitsUntilTheFileIsQuiet(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "film.mkv")
	writeWatchFile(t, path, 1000)

	watcher := newTestWatcher(folder)
	start := time.Now()

	if found := watcher.scan(start); len(found) != 0 {
		t.Fatalf("beim ersten Sehen darf nichts gemeldet werden, bekommen %v", names(found))
	}
	if found := watcher.scan(start.Add(watchQuietTime - time.Second)); len(found) != 0 {
		t.Fatalf("vor Ablauf der Ruhezeit darf nichts kommen, bekommen %v", names(found))
	}

	// Die Datei wächst weiter — die Ruhezeit beginnt von vorn.
	writeWatchFile(t, path, 2000)
	if found := watcher.scan(start.Add(watchQuietTime + time.Second)); len(found) != 0 {
		t.Fatalf("nach dem Wachsen muss neu gewartet werden, bekommen %v", names(found))
	}

	found := watcher.scan(start.Add(2*watchQuietTime + 2*time.Second))
	if len(found) != 1 || found[0].Name != "film.mkv" {
		t.Fatalf("nach der Ruhezeit erwartet [film.mkv], bekommen %v", names(found))
	}

	// Und danach nie wieder: sonst konvertierte dieselbe Datei endlos.
	if again := watcher.scan(start.Add(3 * watchQuietTime)); len(again) != 0 {
		t.Fatalf("eine gemeldete Datei darf nicht wiederkommen, bekommen %v", names(again))
	}

	// Und auch nicht nach beliebig vielen weiteren Durchgängen: Die Datei liegt
	// ja unverändert weiter im Ordner. Ohne die Merkliste käme sie nach jeder
	// abgelaufenen Ruhezeit erneut — dieselbe Datei würde endlos konvertiert.
	for _, later := range []time.Duration{4 * watchQuietTime, 6 * watchQuietTime} {
		if again := watcher.scan(start.Add(later)); len(again) != 0 {
			t.Fatalf("auch später darf sie nicht wiederkommen, bekommen %v", names(again))
		}
	}
}

// TestWatchLooksIntoSubfolders: Der Nutzer beobachtet den Download-Ordner, und
// Downloads landen oft in einem Unterordner.
func TestWatchLooksIntoSubfolders(t *testing.T) {
	folder := t.TempDir()
	writeWatchFile(t, filepath.Join(folder, "serie", "folge1.mkv"), 100)

	watcher := newTestWatcher(folder)
	start := time.Now()
	watcher.scan(start)

	found := names(watcher.scan(start.Add(watchQuietTime + time.Second)))
	if !found["folge1.mkv"] {
		t.Errorf("die Datei aus dem Unterordner fehlt, bekommen %v", found)
	}
}

// TestWatchSkipsWhatIsAlreadyDone: Ergebnisse des Konverters dürfen nicht
// erneut in die Warteschlange wandern — weder das Ergebnis selbst noch die
// Quelle, zu der bereits ein Ergebnis daneben liegt.
func TestWatchSkipsWhatIsAlreadyDone(t *testing.T) {
	folder := t.TempDir()
	writeWatchFile(t, filepath.Join(folder, "alt.h265.mp4"), 100)  // ein Ergebnis
	writeWatchFile(t, filepath.Join(folder, "alt.mkv"), 500)       // dessen Quelle
	writeWatchFile(t, filepath.Join(folder, "neu.mkv"), 500)       // wirklich neu
	writeWatchFile(t, filepath.Join(folder, "halb.part.mkv"), 500) // Bruchstück
	writeWatchFile(t, filepath.Join(folder, "originals", "weg.mkv"), 500)

	watcher := newTestWatcher(folder)
	start := time.Now()
	watcher.scan(start)
	found := names(watcher.scan(start.Add(watchQuietTime + time.Second)))

	if !found["neu.mkv"] {
		t.Errorf("die neue Datei fehlt, bekommen %v", found)
	}
	for _, unwanted := range []string{"alt.h265.mp4", "alt.mkv", "halb.part.mkv", "weg.mkv"} {
		if found[unwanted] {
			t.Errorf("%s darf nicht gemeldet werden, bekommen %v", unwanted, found)
		}
	}
}

// TestWatchIgnoresNonVideoFiles: In einem Download-Ordner liegt alles Mögliche.
func TestWatchIgnoresNonVideoFiles(t *testing.T) {
	folder := t.TempDir()
	writeWatchFile(t, filepath.Join(folder, "handbuch.pdf"), 100)
	writeWatchFile(t, filepath.Join(folder, "musik.mp3"), 100)
	writeWatchFile(t, filepath.Join(folder, "film.mkv.crdownload"), 100)

	watcher := newTestWatcher(folder)
	start := time.Now()
	watcher.scan(start)

	if found := watcher.scan(start.Add(watchQuietTime + time.Second)); len(found) != 0 {
		t.Errorf("nichts davon ist ein fertiges Video, bekommen %v", names(found))
	}
}

// TestWatchStartForgetsThePreviousRound: Beim Einschalten soll der vorhandene
// Bestand mitgenommen werden (Entscheidung des Nutzers 2026-08-18). Nach einem
// Stop und einem neuen Start darf also nichts von früher blockieren.
func TestWatchStartForgetsThePreviousRound(t *testing.T) {
	folder := t.TempDir()
	writeWatchFile(t, filepath.Join(folder, "film.mkv"), 100)

	watcher := NewFolderWatcher(func([]QueueItem) {}, func(string) {})
	if err := watcher.Start(folder); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !watcher.Watching() || watcher.Folder() != folder {
		t.Error("nach dem Start muss der Ordner beobachtet werden")
	}
	watcher.mu.Lock()
	watcher.seen[filepath.Join(folder, "film.mkv")] = true
	watcher.mu.Unlock()

	watcher.Stop()
	if watcher.Watching() || watcher.Folder() != "" {
		t.Error("nach dem Stop darf nichts mehr beobachtet werden")
	}

	if err := watcher.Start(folder); err != nil {
		t.Fatalf("zweiter Start: %v", err)
	}
	defer watcher.Stop()
	watcher.mu.Lock()
	remembered := len(watcher.seen)
	watcher.mu.Unlock()
	if remembered != 0 {
		t.Errorf("ein neuer Start beginnt von vorn, gemerkt sind aber %d Dateien", remembered)
	}
}

// TestWatchRefusesWhatIsNoFolder: Ein Tippfehler im Pfad muss sofort auffallen,
// statt still nichts zu tun.
func TestWatchRefusesWhatIsNoFolder(t *testing.T) {
	watcher := NewFolderWatcher(func([]QueueItem) {}, func(string) {})
	if err := watcher.Start(filepath.Join(t.TempDir(), "gibt-es-nicht")); err == nil {
		t.Error("ein nicht vorhandener Ordner muss abgelehnt werden")
	}
	file := filepath.Join(t.TempDir(), "keine-mappe.txt")
	writeWatchFile(t, file, 10)
	if err := watcher.Start(file); err == nil {
		t.Error("eine Datei ist kein Ordner und muss abgelehnt werden")
	}
	if watcher.Watching() {
		t.Error("nach einem gescheiterten Start darf nichts laufen")
	}
}

// TestLiveWatchPicksUpANewFile ist der Nachweis am echten Dateisystem: Beobachter
// starten, Datei hineinschreiben, warten, gemeldet bekommen. Er prüft, was die
// Schreibtisch-Tests nicht können — den Takt, die echte Uhr und den Weg über
// die Meldefunktion.
//
// Er dauert über eine halbe Minute und läuft deshalb nur mit
// NVENCFORGEGUI_LIVE=1, sonst hinge jeder normale Testlauf daran.
func TestLiveWatchPicksUpANewFile(t *testing.T) {
	if os.Getenv("NVENCFORGEGUI_LIVE") != "1" {
		t.Skip("live check disabled (set NVENCFORGEGUI_LIVE=1)")
	}

	folder := t.TempDir()
	var mu sync.Mutex
	var reported []string
	watcher := NewFolderWatcher(func(items []QueueItem) {
		mu.Lock()
		defer mu.Unlock()
		for _, item := range items {
			reported = append(reported, item.Name)
		}
	}, func(string) {})

	if err := watcher.Start(folder); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer watcher.Stop()

	writeWatchFile(t, filepath.Join(folder, "frisch.mkv"), 4096)
	began := time.Now()

	deadline := time.Now().Add(watchQuietTime + 4*watchScanInterval)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(reported) > 0
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(time.Second)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reported) != 1 || reported[0] != "frisch.mkv" {
		t.Fatalf("erwartet [frisch.mkv], bekommen %v", reported)
	}
	if waited := time.Since(began); waited < watchQuietTime {
		t.Fatalf("die Datei kam nach %v — die Ruhezeit von %v wurde nicht abgewartet", waited, watchQuietTime)
	}
	t.Logf("gemeldet nach %v (Ruhezeit %v, Takt %v)", time.Since(began).Round(time.Second), watchQuietTime, watchScanInterval)
}
