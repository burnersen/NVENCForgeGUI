// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

// selfupdate_live_test.go — der Nachweis am echten GitHub-Verzeichnis.
//
// Diese Prüfung fragt wirklich bei GitHub nach und lädt wirklich die
// veröffentlichte Programmdatei herunter. Sie braucht also eine
// Internetverbindung und ein paar Sekunden, und sie läuft nur, wenn
// NVENCFORGEGUI_LIVE=1 gesetzt ist — wie die Prüfungen am echten Video.
//
// Ersetzt wird dabei nichts: Die geladene Datei landet in einem Ordner, den der
// Test selbst anlegt und danach wieder wegräumt. Der Tausch der laufenden
// Programmdatei wird in selfupdate_test.go mit Attrappen geprüft.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestLiveUpdateCheckAndDownload geht den Weg bis kurz vor den Tausch:
// nachfragen, den Anhang finden, ihn laden und die Größe prüfen.
func TestLiveUpdateCheckAndDownload(t *testing.T) {
	if os.Getenv("NVENCFORGEGUI_LIVE") != "1" {
		t.Skip("Live-Prüfung übersprungen (NVENCFORGEGUI_LIVE=1 setzt sie in Gang)")
	}

	ctx := context.Background()

	check, err := checkForUpdate(ctx)
	if err != nil {
		t.Fatalf("checkForUpdate: %v", err)
	}
	if check.Latest == "" {
		t.Fatal("GitHub nannte keine Ausgabe")
	}
	if check.SizeBytes <= 0 {
		t.Fatalf("Anhang %s hat keine Größe", guiExeName)
	}
	t.Logf("laufend: %s · veröffentlicht: %s (%d Bytes) · neuer: %v",
		check.Current, check.Latest, check.SizeBytes, check.Newer)

	release, err := fetchRelease(ctx, guiLatestReleaseURL)
	if err != nil {
		t.Fatalf("fetchRelease: %v", err)
	}
	asset, err := pickAsset(release, guiExeName)
	if err != nil {
		t.Fatalf("pickAsset: %v", err)
	}

	target := filepath.Join(t.TempDir(), guiExeName+".part")
	var lastDone, lastTotal int64
	if err := downloadToFile(ctx, asset, target, func(done, total int64) {
		lastDone, lastTotal = done, total
	}); err != nil {
		t.Fatalf("downloadToFile: %v", err)
	}
	if err := verifyDownloadedSize(target, asset.Size); err != nil {
		t.Fatalf("verifyDownloadedSize: %v", err)
	}
	if lastTotal != asset.Size || lastDone != asset.Size {
		t.Errorf("Fortschritt endete bei %d/%d, erwartet %d/%d",
			lastDone, lastTotal, asset.Size, asset.Size)
	}

	// Eine echte Programmdatei fängt mit "MZ" an. Der Test kostet nichts und
	// würde eine Fehlerseite auffallen lassen, die zufällig die richtige Größe
	// hätte.
	head := make([]byte, 2)
	file, err := os.Open(target)
	if err != nil {
		t.Fatalf("geladene Datei öffnen: %v", err)
	}
	defer file.Close()
	if _, err := file.Read(head); err != nil {
		t.Fatalf("geladene Datei lesen: %v", err)
	}
	if string(head) != "MZ" {
		t.Errorf("geladene Datei beginnt mit %q, erwartet %q", string(head), "MZ")
	}
}
