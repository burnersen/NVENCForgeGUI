// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"strings"
	"testing"
)

// Der Prozentwert kommt aus der Fensterseite und wird dort aus Dateigrößen
// gerechnet. Windows zeichnet daraus einen Balken — ein Wert außerhalb von
// 0..100 ergibt ein Bild, das niemand erklären kann.
func TestClampPercent(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"unterhalb von null", -5, 0},
		{"genau null", 0, 0},
		{"mittendrin", 47, 47},
		{"genau hundert", 100, 100},
		{"darüber hinaus", 250, 100},
	}
	for _, c := range cases {
		if got := clampPercent(c.in); got != c.want {
			t.Errorf("%s: clampPercent(%d) = %d, erwartet %d", c.name, c.in, got, c.want)
		}
	}
}

// Der Taskleisten-Knopf zeigt nur den ANFANG des Titels. Steht die Zahl hinten,
// ist sie genau dann abgeschnitten, wenn man sie braucht.
func TestProgressTitleStartsWithTheFigure(t *testing.T) {
	title := progressTitle(42)
	if !strings.HasPrefix(title, "42 %") {
		t.Errorf("Titel beginnt nicht mit der Prozentzahl: %q", title)
	}
	if !strings.Contains(title, baseWindowTitle()) {
		t.Errorf("Titel nennt das Programm nicht mehr: %q", title)
	}
}

// Ein unmöglicher Wert darf auch im Titel nicht auftauchen.
func TestProgressTitleClamps(t *testing.T) {
	if title := progressTitle(150); !strings.HasPrefix(title, "100 %") {
		t.Errorf("progressTitle(150) = %q, erwartet Beginn mit 100 %%", title)
	}
}

// Der Titel im Ruhezustand muss die Version tragen: Er ist die einzige Stelle,
// an der ein Benutzer ohne Info-Seite sieht, welche Ausgabe läuft. Und
// HideProgress stellt genau diesen Titel wieder her — stimmt er nicht mit dem
// Starttitel aus main.go überein, ändert sich der Fensterkopf nach dem ersten
// Lauf für immer.
func TestBaseWindowTitleCarriesTheVersion(t *testing.T) {
	if !strings.Contains(baseWindowTitle(), guiVersion) {
		t.Errorf("Grundtitel ohne Version: %q (Version %q)", baseWindowTitle(), guiVersion)
	}
	if busy := busyTitle(); !strings.HasPrefix(busy, baseWindowTitle()) {
		t.Errorf("Arbeitstitel baut nicht auf dem Grundtitel auf: %q", busy)
	}
}
