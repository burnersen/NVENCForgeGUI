// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// watchfolder.go — ein Ordner wird beobachtet, neue Videos wandern von selbst
// in die Warteschlange.
//
// Bewusst durch regelmäßiges Nachsehen statt über Windows-Ereignisse: Der
// Beobachter muss rekursiv arbeiten, neu angelegte Unterordner mitbekommen und
// auch auf Netzlaufwerken zuverlässig sein — dort melden Datei-Ereignisse
// bekanntermaßen nicht alles. Ein Durchlauf über einen Download-Ordner kostet
// Millisekunden; diese Ruhe ist die zusätzliche Abhängigkeit nicht wert.
//
// Die zweite Aufgabe ist wichtiger als das Finden: NICHT zu früh melden. Eine
// Datei, die noch geschrieben wird, würde halb konvertiert.
package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// watchScanInterval ist der Takt, in dem nachgesehen wird.
	watchScanInterval = 5 * time.Second

	// watchQuietTime ist die Ruhe, die eine Datei zeigen muss, bevor sie als
	// fertig geschrieben gilt: Ihre Größe darf sich so lange nicht mehr ändern.
	// 30 Sekunden ist die Vorgabe des Nutzers (2026-08-18) — lieber eine halbe
	// Minute warten als ein halb heruntergeladenes Video konvertieren.
	watchQuietTime = 30 * time.Second
)

// watchSample hält fest, welche Größe eine Datei hatte und seit wann.
type watchSample struct {
	size  int64
	since time.Time
}

// FolderWatcher beobachtet genau einen Ordner. Alles, was von außen darauf
// zugreift, geht über die Sperre — der Beobachter läuft in einem eigenen
// Ablauf, die Oberfläche fragt aus einem anderen.
type FolderWatcher struct {
	mu       sync.Mutex
	folder   string
	cancel   context.CancelFunc
	seen     map[string]bool        // schon gemeldet, kommt nicht noch einmal
	samples  map[string]watchSample // Größe je Datei, für die Ruhe-Prüfung
	announce func([]QueueItem)
	note     func(string)
}

// NewFolderWatcher erzeugt den Beobachter. announce bekommt jede Datei, die
// fertig geschrieben ist; note schreibt Meldungen ins Protokoll des Fensters.
func NewFolderWatcher(announce func([]QueueItem), note func(string)) *FolderWatcher {
	return &FolderWatcher{
		seen:     make(map[string]bool),
		samples:  make(map[string]watchSample),
		announce: announce,
		note:     note,
	}
}

// Start beginnt die Beobachtung. Ein zweiter Aufruf löst den vorherigen ab.
func (w *FolderWatcher) Start(folder string) error {
	info, err := os.Stat(folder)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return &os.PathError{Op: "watch", Path: folder, Err: os.ErrInvalid}
	}

	w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	w.mu.Lock()
	w.folder = folder
	w.cancel = cancel
	// Frischer Anfang: Was in einem früheren Durchgang gemeldet wurde, gilt für
	// diesen nicht mehr. So nimmt das Einschalten den vorhandenen Bestand mit —
	// so, wie der Nutzer es wollte.
	w.seen = make(map[string]bool)
	w.samples = make(map[string]watchSample)
	w.mu.Unlock()

	go w.loop(ctx)
	return nil
}

// Stop beendet die Beobachtung. Mehrfaches Aufrufen schadet nicht.
func (w *FolderWatcher) Stop() {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Watching sagt, ob gerade beobachtet wird.
func (w *FolderWatcher) Watching() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cancel != nil
}

// Folder liefert den beobachteten Ordner ("" wenn keiner).
func (w *FolderWatcher) Folder() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel == nil {
		return ""
	}
	return w.folder
}

// loop sieht im Takt nach. Der erste Durchgang kommt sofort, damit der
// vorhandene Bestand nicht erst nach einer Wartezeit auftaucht.
func (w *FolderWatcher) loop(ctx context.Context) {
	ticker := time.NewTicker(watchScanInterval)
	defer ticker.Stop()

	for {
		if found := w.scan(time.Now()); len(found) > 0 {
			w.announce(found)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// scan durchsucht den Ordner einmal und liefert die Dateien, die seit
// watchQuietTime unverändert sind und noch nicht gemeldet wurden.
//
// now wird übergeben statt gelesen, damit die Ruhe-Regel prüfbar ist, ohne im
// Test eine halbe Minute zu warten.
func (w *FolderWatcher) scan(now time.Time) []QueueItem {
	w.mu.Lock()
	folder := w.folder
	w.mu.Unlock()
	if folder == "" {
		return nil
	}

	candidates, finishedStems := collectWatchCandidates(folder)

	var ready []QueueItem
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, candidate := range candidates {
		key := strings.ToLower(candidate.path)
		if w.seen[key] {
			continue
		}
		// Liegt das Ergebnis schon daneben, war diese Datei bereits dran —
		// etwa wenn das Fenster zwischendurch neu gestartet wurde und der
		// Nutzer die Originale behält.
		if finishedStems[watchStemKey(candidate.path)] {
			w.seen[key] = true
			continue
		}

		previous, known := w.samples[key]
		if !known || previous.size != candidate.size {
			w.samples[key] = watchSample{size: candidate.size, since: now}
			continue
		}
		if now.Sub(previous.since) < watchQuietTime {
			continue
		}
		if !canBeOpened(candidate.path) {
			// Noch von jemandem gehalten: nächster Durchgang, neue Ruhezeit.
			w.samples[key] = watchSample{size: candidate.size, since: now}
			continue
		}

		w.seen[key] = true
		delete(w.samples, key)
		ready = append(ready, newQueueItem(candidate.path))
	}
	return ready
}

// watchCandidate ist eine gefundene Videodatei samt ihrer Größe.
type watchCandidate struct {
	path string
	size int64
}

// collectWatchCandidates durchsucht den Ordner samt Unterordnern und liefert
// zweierlei: die Videodateien, die in Frage kommen, und die Namensstämme, zu
// denen bereits ein Ergebnis des Konverters im selben Ordner liegt.
//
// Beides in einem Durchgang, weil sonst für jede Datei einzeln nach einem
// Dutzend möglicher Ergebnisnamen gesucht werden müsste.
func collectWatchCandidates(folder string) ([]watchCandidate, map[string]bool) {
	var candidates []watchCandidate
	finishedStems := make(map[string]bool)

	_ = filepath.WalkDir(folder, func(path string, entry os.DirEntry, err error) error {
		// Ein unlesbarer Unterordner darf den Rest nicht abbrechen.
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			// Der Konverter legt die Originale hier ab; die sind erledigt.
			if strings.EqualFold(entry.Name(), "originals") {
				return filepath.SkipDir
			}
			return nil
		}
		if !isVideoFile(path) {
			return nil
		}
		if looksConverted(path) {
			finishedStems[watchStemKey(path)] = true
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		candidates = append(candidates, watchCandidate{path: path, size: info.Size()})
		return nil
	})

	return candidates, finishedStems
}

// watchStemKey bildet den Schlüssel, unter dem Quelle und Ergebnis zueinander
// finden: Ordner + Name ohne Endung und ohne den Zwischenteil des Konverters.
// "C:\dl\film.h265.mp4" und "C:\dl\film.mkv" ergeben denselben Schlüssel.
func watchStemKey(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	lower := strings.ToLower(base)
	for _, suffix := range convertedSuffixes {
		if strings.HasSuffix(lower, suffix) {
			lower = strings.TrimSuffix(lower, suffix)
			break
		}
	}
	return strings.ToLower(filepath.Dir(path)) + string(filepath.Separator) + lower
}

// canBeOpened prüft, ob die Datei überhaupt lesbar ist. Hält ein Programm sie
// noch exklusiv — manche Downloader tun das —, scheitert das Öffnen, und die
// Datei ist erkennbar noch nicht fertig.
func canBeOpened(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}
