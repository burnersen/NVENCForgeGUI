// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// main_test.go — Prüfungen für die Meldungen der Fensterbibliothek.
package main

import (
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// TestWindowMessagesReplacesTheCrashText prüft, dass der irreführende
// Originaltext wirklich ersetzt ist.
func TestWindowMessagesReplacesTheCrashText(t *testing.T) {
	original := windows.DefaultMessages().WebView2ProcessCrash
	eigener := windowMessages().WebView2ProcessCrash

	if eigener == original {
		t.Error("die Absturzmeldung ist noch der Originaltext von Wails")
	}
	if strings.TrimSpace(eigener) == "" {
		t.Fatal("die Absturzmeldung ist leer — dann stünde im Ernstfall nichts im Fenster")
	}
	// Die zwei Aussagen, um die es geht: Es ist nichts kaputt, und was zu tun ist.
	for _, teil := range []string{"did not crash", "start it again", "Repair"} {
		if !strings.Contains(eigener, teil) {
			t.Errorf("die Meldung sagt nichts über %q: %q", teil, eigener)
		}
	}
}

// TestWindowMessagesKeepsTheOthers ist die eigentliche Regressionsbremse:
// Wer die Meldungen selbst zusammenbaut, statt die Standardliste zu nehmen,
// löscht damit unbemerkt zehn andere Texte — darunter den, der erklärt, wie man
// eine fehlende Webansicht nachinstalliert.
func TestWindowMessagesKeepsTheOthers(t *testing.T) {
	standard := windows.DefaultMessages()
	eigene := windowMessages()

	felder := map[string][2]string{
		"InstallationRequired": {standard.InstallationRequired, eigene.InstallationRequired},
		"UpdateRequired":       {standard.UpdateRequired, eigene.UpdateRequired},
		"MissingRequirements":  {standard.MissingRequirements, eigene.MissingRequirements},
		"Webview2NotInstalled": {standard.Webview2NotInstalled, eigene.Webview2NotInstalled},
		"Error":                {standard.Error, eigene.Error},
		"FailedToInstall":      {standard.FailedToInstall, eigene.FailedToInstall},
		"DownloadPage":         {standard.DownloadPage, eigene.DownloadPage},
		"PressOKToInstall":     {standard.PressOKToInstall, eigene.PressOKToInstall},
		"ContactAdmin":         {standard.ContactAdmin, eigene.ContactAdmin},
		"InvalidFixedWebview2": {standard.InvalidFixedWebview2, eigene.InvalidFixedWebview2},
	}
	for name, werte := range felder {
		if werte[1] == "" {
			t.Errorf("Meldung %s ist leer — sie wurde beim Ersetzen mitgelöscht", name)
			continue
		}
		if werte[0] != werte[1] {
			t.Errorf("Meldung %s wurde verändert: %q statt %q", name, werte[1], werte[0])
		}
	}
}
