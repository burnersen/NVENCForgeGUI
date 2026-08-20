// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// savings_test.go — das Sparbuch, vor allem an seinen Kanten.
//
// Die teuren Fehler hier fallen erst Tage später auf: eine Wochensumme, die am
// Montag nicht zurückspringt, oder eine Monatssumme, die den Vormonat
// mitschleppt. Beides lässt sich nur prüfen, indem die Uhr gestellt wird —
// deshalb ist sie im Sparbuch ein Feld.
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// at baut eine feste Uhrzeit. Alles in der lokalen Zeitzone, weil das Sparbuch
// nach Kalendertagen rechnet und nicht nach UTC.
func at(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.Local)
}

// testLedger ist ein Sparbuch mit gestellter Uhr und ohne Datei.
func testLedger(now time.Time) *savingsLedger {
	clock := now
	return &savingsLedger{
		book: savingsBook{Days: map[string]savingsDay{}},
		now:  func() time.Time { return clock },
	}
}

// ledgerAt stellt die Uhr eines bestehenden Sparbuchs.
func ledgerAt(ledger *savingsLedger, now time.Time) {
	ledger.now = func() time.Time { return now }
}

func TestSavingsCountsWeekAndMonth(t *testing.T) {
	// Mittwoch, 19. August 2026.
	ledger := testLedger(at(2026, time.August, 19, 12))
	ledger.Add(100)
	ledger.Add(50.5)

	report := ledger.Report()
	if report.WeekFiles != 2 || report.MonthFiles != 2 {
		t.Fatalf("Dateien: Woche %d, Monat %d — erwartet 2 und 2", report.WeekFiles, report.MonthFiles)
	}
	if report.WeekMB != 150.5 || report.MonthMB != 150.5 {
		t.Fatalf("MB: Woche %v, Monat %v — erwartet je 150.5", report.WeekMB, report.MonthMB)
	}
}

func TestSavingsWeekEndsOnSunday(t *testing.T) {
	// Sonntag, 16. August 2026: letzter Tag der Woche.
	ledger := testLedger(at(2026, time.August, 16, 20))
	ledger.Add(500)

	// Montag, 17. August: neue Woche, derselbe Monat.
	ledgerAt(ledger, at(2026, time.August, 17, 9))
	ledger.Add(20)

	report := ledger.Report()
	if report.WeekMB != 20 {
		t.Errorf("die neue Woche zählt %v MB — erwartet 20 (der Sonntag gehört zur alten)", report.WeekMB)
	}
	if report.MonthMB != 520 {
		t.Errorf("der Monat zählt %v MB — erwartet 520 (beide Tage)", report.MonthMB)
	}
}

func TestSavingsMonthDoesNotCarryOver(t *testing.T) {
	// Montag, 31. August 2026 — derselbe Woche wie der 1. September.
	ledger := testLedger(at(2026, time.August, 31, 18))
	ledger.Add(300)

	// Dienstag, 1. September: neuer Monat, gleiche Woche.
	ledgerAt(ledger, at(2026, time.September, 1, 10))
	ledger.Add(40)

	report := ledger.Report()
	if report.MonthMB != 40 {
		t.Errorf("der neue Monat zählt %v MB — erwartet 40", report.MonthMB)
	}
	if report.WeekMB != 340 {
		t.Errorf("die Woche zählt %v MB — erwartet 340: sie läuft über den Monatswechsel", report.WeekMB)
	}
}

func TestSavingsSurvivesARestart(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, savingsFileName)

	first := testLedger(at(2026, time.August, 19, 12))
	first.path = path
	first.Add(250)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("das Sparbuch wurde nicht geschrieben: %v", err)
	}

	// Ein zweiter Start liest dieselbe Datei.
	second := &savingsLedger{book: loadSavingsBook(path), path: path}
	ledgerAt(second, at(2026, time.August, 20, 8))
	if got := second.Report().WeekMB; got != 250 {
		t.Errorf("nach dem Neustart %v MB — erwartet 250", got)
	}
}

func TestSavingsResetEmptiesTheBook(t *testing.T) {
	ledger := testLedger(at(2026, time.August, 19, 12))
	ledger.Add(700)
	if ledger.Reset().WeekMB != 0 {
		t.Fatal("nach dem Zurücksetzen steht noch etwas in der Woche")
	}
	if ledger.Report().MonthFiles != 0 {
		t.Error("nach dem Zurücksetzen zählt der Monat noch Dateien")
	}
}

func TestSavingsForgetsOldDays(t *testing.T) {
	ledger := testLedger(at(2024, time.January, 2, 12))
	ledger.Add(10)

	// Weit über die Aufbewahrung hinaus.
	ledgerAt(ledger, at(2026, time.August, 19, 12))
	ledger.Add(20)

	if len(ledger.book.Days) != 1 {
		t.Errorf("%d Tageszeilen — erwartet 1, die alte gehört vergessen", len(ledger.book.Days))
	}
}

// Nur Ergebnisse, bei denen wirklich Platz frei wurde, zählen. Ein
// übersprungener oder abgebrochener Lauf hat nichts gespart, und ihn
// mitzuzählen ergäbe eine Zahl, die sich mit dem Explorer nicht deckt.
func TestOnlyRealResultsAreCounted(t *testing.T) {
	cases := []struct {
		name    string
		event   map[string]any
		counted bool
		savedMB float64
	}{
		{"eine fertige Datei", map[string]any{"ev": "result", "status": "success", "saved_mb": 120.0}, true, 120},
		{"übersprungen", map[string]any{"ev": "result", "status": "skipped", "saved_mb": 0.0}, false, 0},
		{"fehlgeschlagen", map[string]any{"ev": "result", "status": "failed", "saved_mb": 0.0}, false, 0},
		{"abgebrochen, Teildatei", map[string]any{"ev": "result", "status": "preview", "saved_mb": 40.0}, false, 0},
		{"gar kein Ergebnis", map[string]any{"ev": "progress", "pct": 50.0}, false, 0},
		{"Ergebnis ohne Zahl", map[string]any{"ev": "result", "status": "success"}, false, 0},
		// Auch das kommt vor: eine Datei wird größer. Das ist ein ehrliches
		// Ergebnis und darf die Bilanz drücken.
		{"größer geworden", map[string]any{"ev": "result", "status": "success", "saved_mb": -15.0}, true, -15},
	}
	for _, c := range cases {
		savedMB, counted := savedMBFromEvent(c.event)
		if counted != c.counted || savedMB != c.savedMB {
			t.Errorf("%s: %v / %v MB — erwartet %v / %v MB", c.name, counted, savedMB, c.counted, c.savedMB)
		}
	}
}
