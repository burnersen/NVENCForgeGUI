// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// savings.go — wie viel Platz das Umwandeln bisher eingespart hat.
//
// Buch geführt wird TAGEWEISE, nicht als zwei fertige Summen "diese Woche" und
// "diesen Monat". Zwei Summen müssten am Montag und am Monatsersten von selbst
// zurückspringen — dafür bräuchte das Programm einen Wecker, und wer sein
// Fenster über den Wochenwechsel offen lässt oder eine Woche gar nicht startet,
// bekäme falsche Zahlen. Aus Tageszeilen rechnet die Anzeige beide Werte
// jederzeit richtig aus, ganz gleich wann zuletzt jemand hingesehen hat.
//
// Die Datei liegt neben der exe, aus demselben Grund wie der Fensterzustand:
// Das Programm verspricht, nichts zu installieren und nichts in der
// Registrierung zu hinterlassen (siehe windowstate.go).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// savingsFileName ist das Sparbuch neben der exe.
	savingsFileName = "NVENCForgeGUI.savings"

	// savingsKeepDays ist, wie lange Tageszeilen aufgehoben werden. Gebraucht
	// wird höchstens der laufende Monat; ein gutes Jahr Vorrat kostet nichts
	// (eine Zeile ist ein paar Dutzend Bytes) und lässt Raum für eine spätere
	// Jahresansicht, ohne dass heute Zahlen weggeworfen werden.
	savingsKeepDays = 400

	// savingsDayFormat ist der Schlüssel einer Tageszeile. Sortierbar und für
	// einen Menschen lesbar, falls jemand die Datei öffnet.
	savingsDayFormat = "2006-01-02"
)

// savingsDay ist die Bilanz EINES Tages.
type savingsDay struct {
	Files int     `json:"files"`
	MB    float64 `json:"mb"`
}

// savingsBook ist der Inhalt der Datei.
type savingsBook struct {
	Days map[string]savingsDay `json:"days"`
}

// SavingsReport ist, was die Leiste unten im Fenster anzeigt.
type SavingsReport struct {
	WeekMB     float64 `json:"weekMB"`
	WeekFiles  int     `json:"weekFiles"`
	MonthMB    float64 `json:"monthMB"`
	MonthFiles int     `json:"monthFiles"`
}

// savingsLedger führt das Sparbuch. Die Ergebnisse kommen aus mehreren
// Konvertern gleichzeitig, deshalb die Sperre: zwei Läufer, die im selben
// Augenblick fertig werden, dürfen sich nicht gegenseitig überschreiben.
type savingsLedger struct {
	mu   sync.Mutex
	book savingsBook

	// now ist die Uhr. Als Feld, damit die Tests einen Wochen- oder
	// Monatswechsel nachstellen können, ohne die Systemzeit anzufassen.
	now func() time.Time

	// path ist die Datei. Leer heißt: nur im Speicher (Tests).
	path string
}

// newSavingsLedger öffnet das Sparbuch neben der exe. Lässt es sich nicht
// lesen, fängt das Programm bei null an, statt den Start zu verweigern — eine
// Statistik ist nichts, wofür ein Fenster nicht aufgehen darf.
func newSavingsLedger() *savingsLedger {
	ledger := &savingsLedger{now: time.Now, book: savingsBook{Days: map[string]savingsDay{}}}
	path, err := savingsPath()
	if err != nil {
		return ledger
	}
	ledger.path = path
	ledger.book = loadSavingsBook(path)
	return ledger
}

// loadSavingsBook liest das Sparbuch von der Platte. Fehlt es oder ist es
// beschädigt, geht es bei null los: Eine Statistik ist nichts, wofür ein
// Fenster nicht aufgehen darf.
func loadSavingsBook(path string) savingsBook {
	empty := savingsBook{Days: map[string]savingsDay{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		return empty
	}
	var book savingsBook
	if json.Unmarshal(raw, &book) != nil || book.Days == nil {
		return empty
	}
	return book
}

func savingsPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("savings.go: savingsPath: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), savingsFileName), nil
}

// Add bucht eine fertige Datei ein. savedMB darf negativ sein: Auch das ist
// ein ehrliches Ergebnis (eine Datei kann größer werden), und es stillschweigend
// als Null zu zählen würde die Bilanz schönen.
func (l *savingsLedger) Add(savedMB float64) SavingsReport {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.now().Format(savingsDayFormat)
	day := l.book.Days[key]
	day.Files++
	day.MB += savedMB
	l.book.Days[key] = day

	l.forgetOldDays()
	l.write()
	return l.report()
}

// Report liefert den Stand, ohne etwas zu ändern.
func (l *savingsLedger) Report() SavingsReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.report()
}

// Reset leert das Sparbuch. Der Nutzer hat sich das auf der Einstellungsseite
// gewünscht und nicht in der Leiste selbst: Was nicht rückgängig zu machen ist,
// gehört nicht neben eine Anzeige, an der man täglich vorbeisieht.
func (l *savingsLedger) Reset() SavingsReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.book.Days = map[string]savingsDay{}
	l.write()
	return l.report()
}

// report rechnet Woche und Monat aus den Tageszeilen aus. Erwartet die Sperre.
func (l *savingsLedger) report() SavingsReport {
	now := l.now()
	weekStart := startOfWeek(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var report SavingsReport
	for key, day := range l.book.Days {
		when, err := time.ParseInLocation(savingsDayFormat, key, now.Location())
		if err != nil {
			continue // beschädigte Zeile: überspringen, nicht raten
		}
		if !when.Before(weekStart) {
			report.WeekMB += day.MB
			report.WeekFiles += day.Files
		}
		if !when.Before(monthStart) {
			report.MonthMB += day.MB
			report.MonthFiles += day.Files
		}
	}
	return report
}

// startOfWeek ist Montag 00:00 — die Woche, wie sie hierzulande gezählt wird.
// Gos Wochentage fangen bei Sonntag an, deshalb die Sonderbehandlung: ohne sie
// stünde der Sonntag am ANFANG einer neuen Woche statt am Ende der alten.
func startOfWeek(now time.Time) time.Time {
	daysSinceMonday := (int(now.Weekday()) + 6) % 7
	day := now.AddDate(0, 0, -daysSinceMonday)
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location())
}

// forgetOldDays hält die Datei klein. Erwartet die Sperre.
func (l *savingsLedger) forgetOldDays() {
	oldest := l.now().AddDate(0, 0, -savingsKeepDays).Format(savingsDayFormat)
	for key := range l.book.Days {
		// Das Datumsformat ist so gebaut, dass der Textvergleich der
		// Zeitvergleich ist — deshalb reicht hier ein "kleiner als".
		if key < oldest {
			delete(l.book.Days, key)
		}
	}
}

// write legt das Sparbuch ab. Ein Fehler wird bewusst verschluckt: Er darf
// weder einen laufenden Stapel anhalten noch eine Meldung über die Anzeige
// legen. Die Zahlen des laufenden Fensters stimmen weiter, nur der nächste
// Start fängt dann früher an. Erwartet die Sperre.
func (l *savingsLedger) write() {
	if l.path == "" {
		return
	}
	if raw, err := json.Marshal(l.book); err == nil {
		_ = os.WriteFile(l.path, raw, 0o644)
	}
}

// savedMBFromEvent liest die eingesparten Megabyte aus einem Ergebnis-Ereignis
// des Konverters — und nur aus einem, das wirklich eine Datei umgewandelt hat.
//
// "skipped" und "failed" haben nichts gespart, und "preview" ist ein Abbruch:
// Dort steht zwar eine Teildatei, aber das Original ist noch da, also ist auf
// der Platte nichts frei geworden. Sie alle mitzuzählen wäre eine Zahl, die
// sich mit dem Explorer nicht deckt.
func savedMBFromEvent(event map[string]any) (float64, bool) {
	if kind, _ := event["ev"].(string); kind != "result" {
		return 0, false
	}
	if status, _ := event["status"].(string); status != "success" {
		return 0, false
	}
	// Zahlen kommen als JSON durch und sind deshalb float64.
	saved, ok := event["saved_mb"].(float64)
	if !ok {
		return 0, false
	}
	return saved, true
}
