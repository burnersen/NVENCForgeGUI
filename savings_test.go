// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// savings_test.go — das Sparbuch, vor allem an seinen Kanten.
//
// Die teuren Fehler hier fallen erst viel später auf: eine Zahl, die nach
// einem Neustart kleiner ist als vorher, oder eine Bilanz aus der Zeit vor dem
// Umbau, die beim ersten Start der neuen Fassung stillschweigend verschwindet.
// Beides wird hier festgehalten.
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testLedger ist ein Sparbuch ohne Datei — nur im Speicher.
func testLedger() *savingsLedger {
	return &savingsLedger{}
}

func TestSavingsAddsUpForever(t *testing.T) {
	ledger := testLedger()
	ledger.Add(100, 60)
	report := ledger.Add(50.5, 30)

	if report.TotalMB != 150.5 {
		t.Errorf("Gesamtersparnis %v MB, erwartet 150.5", report.TotalMB)
	}
	if report.TotalFiles != 2 {
		t.Errorf("Gesamtzahl %d Dateien, erwartet 2", report.TotalFiles)
	}
	if report.TotalSeconds != 90 {
		t.Errorf("Gesamtzeit %v s, erwartet 90", report.TotalSeconds)
	}
}

// TestSavingsCountsAFileThatGrew hält fest, dass eine größer gewordene Datei
// die Bilanz drücken darf. Sie als Null zu buchen wäre eine geschönte Zahl.
func TestSavingsCountsAFileThatGrew(t *testing.T) {
	ledger := testLedger()
	ledger.Add(100, 10)
	report := ledger.Add(-30, 10)

	if report.TotalMB != 70 {
		t.Errorf("Gesamtersparnis %v MB, erwartet 70", report.TotalMB)
	}
	if report.TotalFiles != 2 {
		t.Errorf("auch die größer gewordene Datei zählt als Datei: %d statt 2", report.TotalFiles)
	}
}

// TestSavingsRejectsNegativeTime prüft den einen Wert, der niemals stimmen
// kann. Eine zurückgestellte Uhr darf die Gesamtzeit nicht schrumpfen lassen.
func TestSavingsRejectsNegativeTime(t *testing.T) {
	ledger := testLedger()
	ledger.Add(10, 120)
	report := ledger.Add(10, -500)

	if report.TotalSeconds != 120 {
		t.Errorf("Gesamtzeit %v s, erwartet 120 — eine negative Dauer gehört verworfen", report.TotalSeconds)
	}
}

// TestSavingsSurvivesARestart ist der eigentliche Zweck der Datei: Was gezählt
// wurde, muss beim nächsten Start noch da sein.
func TestSavingsSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), savingsFileName)

	first := &savingsLedger{path: path}
	first.Add(200, 300)
	first.Add(100, 150)

	second := &savingsLedger{path: path, book: loadSavingsBook(path)}
	report := second.Report()
	if report.TotalMB != 300 || report.TotalFiles != 2 || report.TotalSeconds != 450 {
		t.Errorf("nach dem Neustart: %+v — erwartet 300 MB, 2 Dateien, 450 s", report)
	}
}

// TestSavingsIsWrittenAfterEveryFile ist die Zusage an den Nutzer: Stürzt das
// Programm mitten im Stapel ab, steht das bis dahin Erreichte auf der Platte.
func TestSavingsIsWrittenAfterEveryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), savingsFileName)
	ledger := &savingsLedger{path: path}

	ledger.Add(80, 42)

	book := loadSavingsBook(path)
	if book.MB != 80 || book.Files != 1 || book.Seconds != 42 {
		t.Errorf("auf der Platte steht %+v — erwartet 80 MB, 1 Datei, 42 s", book)
	}
}

// TestOldDailyBookIsCarriedOver ist der Umzug vom Tagesformat auf die
// Gesamtsummen. Was vor dem Umbau gespart wurde, darf nicht verfallen.
func TestOldDailyBookIsCarriedOver(t *testing.T) {
	path := filepath.Join(t.TempDir(), savingsFileName)
	old := "{\"days\":{\"2026-08-18\":{\"files\":3,\"mb\":1200},\"2026-08-19\":{\"files\":2,\"mb\":800.5}}}"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatalf("Testdatei schreiben: %v", err)
	}

	ledger := &savingsLedger{path: path, book: loadSavingsBook(path)}
	report := ledger.Report()
	if report.TotalMB != 2000.5 || report.TotalFiles != 5 {
		t.Errorf("übernommen wurde %+v — erwartet 2000.5 MB aus 5 Dateien", report)
	}
	// Die Zeit gab es im alten Format nicht und darf nicht geraten werden.
	if report.TotalSeconds != 0 {
		t.Errorf("Zeit %v s — aus dem Tagesformat ist keine bekannt", report.TotalSeconds)
	}

	// Und nach dem ersten Schreiben sind die Tageszeilen weg, statt beim
	// nächsten Start ein zweites Mal dazuaddiert zu werden.
	ledger.Add(0, 0)
	again := loadSavingsBook(path)
	if again.Days != nil {
		t.Errorf("die Tageszeilen stehen noch in der Datei: %v", again.Days)
	}
	if again.MB != 2000.5 {
		t.Errorf("nach dem Umzug stehen %v MB in der Datei, erwartet 2000.5", again.MB)
	}
}

// TestBrokenSavingsFileStartsAtZero: Eine Statistik ist nichts, wofür ein
// Fenster nicht aufgehen darf.
func TestBrokenSavingsFileStartsAtZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), savingsFileName)
	if err := os.WriteFile(path, []byte("{kaputt"), 0o644); err != nil {
		t.Fatalf("Testdatei schreiben: %v", err)
	}
	if book := loadSavingsBook(path); book.MB != 0 || book.Files != 0 {
		t.Errorf("aus einer beschädigten Datei kam %+v statt einer leeren Bilanz", book)
	}
}

func TestSavingsResetEmptiesTheBook(t *testing.T) {
	ledger := testLedger()
	ledger.Add(500, 600)
	report := ledger.Reset()
	if report.TotalMB != 0 || report.TotalFiles != 0 || report.TotalSeconds != 0 {
		t.Errorf("nach dem Zurücksetzen: %+v — erwartet lauter Nullen", report)
	}
}

// TestSavingsFileSitsInTheToolsFolder hält die Nutzer-Entscheidung fest: neben
// der exe soll nur noch die exe und der tools-Ordner liegen.
func TestSavingsFileSitsInTheToolsFolder(t *testing.T) {
	path, err := savingsPath()
	if err != nil {
		t.Fatalf("savingsPath: %v", err)
	}
	if filepath.Base(filepath.Dir(path)) != dataDirName {
		t.Errorf("das Sparbuch liegt in %q, erwartet wurde der Ordner %q", filepath.Dir(path), dataDirName)
	}
}

// TestFileClockMeasuresPerSlot ist der Grund, warum die Uhr eine Zuordnung
// führt und keine einzelne Zeitangabe: Laufen zwei Konverter, darf die zweite
// gestartete Datei die Startzeit der ersten nicht überschreiben.
func TestFileClockMeasuresPerSlot(t *testing.T) {
	base := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.Local)
	now := base
	clock := &fileClock{started: map[int]time.Time{}, now: func() time.Time { return now }}

	clock.Start(0)
	now = base.Add(10 * time.Second)
	clock.Start(1)
	now = base.Add(70 * time.Second)

	if got := clock.Stop(0); got != 70 {
		t.Errorf("Platz 0 lief %v s, erwartet 70", got)
	}
	if got := clock.Stop(1); got != 60 {
		t.Errorf("Platz 1 lief %v s, erwartet 60", got)
	}
}

// TestFileClockWithoutAStartReportsZero: lieber keine Zahl als eine geratene.
func TestFileClockWithoutAStartReportsZero(t *testing.T) {
	clock := newFileClock()
	if got := clock.Stop(2); got != 0 {
		t.Errorf("ohne Anfang kam %v s zurück, erwartet 0", got)
	}
}

// TestFileClockForgetsAfterStopping verhindert, dass die Zeit einer Datei ein
// zweites Mal gebucht wird, wenn der Konverter zwei Ergebnisse meldet.
func TestFileClockForgetsAfterStopping(t *testing.T) {
	clock := newFileClock()
	clock.Start(0)
	clock.Stop(0)
	if got := clock.Stop(0); got != 0 {
		t.Errorf("die zweite Abfrage lieferte %v s, erwartet 0", got)
	}
}

func TestSlotFromEvent(t *testing.T) {
	cases := []struct {
		name  string
		event map[string]any
		slot  int
	}{
		{"vom Läufer gesetzt", map[string]any{"slot": 2}, 2},
		{"durch JSON gelaufen", map[string]any{"slot": 2.0}, 2},
		{"gar keine Nummer", map[string]any{"ev": "file"}, 0},
	}
	for _, c := range cases {
		if got := slotFromEvent(c.event); got != c.slot {
			t.Errorf("%s: Platz %d, erwartet %d", c.name, got, c.slot)
		}
	}
}

func TestEventIsFileStart(t *testing.T) {
	if !eventIsFileStart(map[string]any{"ev": "file", "name": "a.mp4"}) {
		t.Error("der Anfang einer Datei wurde nicht erkannt")
	}
	if eventIsFileStart(map[string]any{"ev": "progress", "pct": 10.0}) {
		t.Error("eine Fortschrittsmeldung wurde für den Anfang einer Datei gehalten")
	}
}

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

// TestFileEventsAreBookedWithTheirTime geht den ganzen Weg, den ein Ergebnis
// im Betrieb nimmt: Der Konverter meldet den Anfang einer Datei, später ihr
// Ergebnis, und beides läuft durch bookAndForward.
//
// Ohne diese Prüfung könnten Uhr und Sparbuch einzeln richtig sein, während
// die Zeit nie im Buch ankommt — dem Nutzer fiele das erst nach Wochen auf,
// wenn die Leiste immer noch "—" zeigt.
func TestFileEventsAreBookedWithTheirTime(t *testing.T) {
	base := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.Local)
	now := base
	app := &App{
		savings: &savingsLedger{},
		clock:   &fileClock{started: map[int]time.Time{}, now: func() time.Time { return now }},
	}

	app.bookAndForward("conv:event", map[string]any{"ev": "file", "slot": 0, "name": "a.mp4"})
	now = base.Add(5 * time.Minute)
	app.bookAndForward("conv:event", map[string]any{
		"ev": "result", "slot": 0, "status": "success", "saved_mb": 2090.0,
	})

	report := app.savings.Report()
	if report.TotalMB != 2090 || report.TotalFiles != 1 {
		t.Errorf("gebucht wurde %+v — erwartet 2090 MB aus einer Datei", report)
	}
	if report.TotalSeconds != 300 {
		t.Errorf("Zeit %v s, erwartet 300 — die Uhr kommt nicht im Sparbuch an", report.TotalSeconds)
	}
}

// TestASkippedFileCostsNoTime: Eine übersprungene Datei wird nicht gebucht,
// und ihre angefangene Messung darf nicht der NÄCHSTEN Datei angehängt werden.
func TestASkippedFileCostsNoTime(t *testing.T) {
	base := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.Local)
	now := base
	app := &App{
		savings: &savingsLedger{},
		clock:   &fileClock{started: map[int]time.Time{}, now: func() time.Time { return now }},
	}

	app.bookAndForward("conv:event", map[string]any{"ev": "file", "slot": 0, "name": "a.mp4"})
	now = base.Add(time.Hour)
	app.bookAndForward("conv:event", map[string]any{"ev": "result", "slot": 0, "status": "skipped"})

	// Die nächste Datei fängt frisch an — sie erbt die Stunde nicht.
	app.bookAndForward("conv:event", map[string]any{"ev": "file", "slot": 0, "name": "b.mp4"})
	now = base.Add(time.Hour + 30*time.Second)
	app.bookAndForward("conv:event", map[string]any{
		"ev": "result", "slot": 0, "status": "success", "saved_mb": 100.0,
	})

	report := app.savings.Report()
	if report.TotalFiles != 1 {
		t.Errorf("%d Dateien gebucht, erwartet 1 — die übersprungene zählt nicht", report.TotalFiles)
	}
	if report.TotalSeconds != 30 {
		t.Errorf("Zeit %v s, erwartet 30 — die übersprungene Datei hat ihre Zeit weitergereicht", report.TotalSeconds)
	}
}
