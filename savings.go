// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// savings.go — was das Umwandeln bisher gebracht hat: gesparter Platz und
// aufgewendete Zeit, beides seit der allerersten Benutzung.
//
// Bis Version 0.9.4 wurde tageweise Buch geführt, damit die Leiste "diese
// Woche" und "diesen Monat" zeigen konnte. Das ist auf Wunsch des Nutzers
// entfallen: Zwei Gesamtsummen brauchen keine Wochen- und Monatsgrenzen, keine
// Zeitzonen und keinen Gedanken daran, wann zuletzt jemand hingesehen hat.
// Was bleibt, sind zwei Zahlen, die nur wachsen — und an denen jeder selbst
// ablesen kann, was ihm das Programm wert ist.
//
// Gebucht wird nach JEDER fertigen Datei und sofort auf die Platte
// geschrieben. Stürzt mitten im Stapel etwas ab, ist alles bis dahin
// Erreichte gesichert.
//
// Die Datei liegt im tools-Ordner neben der exe (siehe datadir.go).
package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// savingsFileName ist das Sparbuch.
const savingsFileName = "NVENCForgeGUI.savings"

// savingsBook ist der Inhalt der Datei.
//
// Days ist das abgelöste Tagesformat und steht nur noch hier, um eine ältere
// Datei einlesen zu können: Was jemand vor dem Umbau gespart hat, soll nicht
// verfallen. Nach dem ersten Schreiben ist das Feld verschwunden.
type savingsBook struct {
	Files   int     `json:"files"`
	MB      float64 `json:"mb"`
	Seconds float64 `json:"seconds"`

	Days map[string]savingsDay `json:"days,omitempty"`
}

// savingsDay ist eine Zeile aus dem alten Tagesformat.
type savingsDay struct {
	Files int     `json:"files"`
	MB    float64 `json:"mb"`
}

// SavingsReport ist, was die Leiste unten im Fenster anzeigt.
type SavingsReport struct {
	TotalMB      float64 `json:"totalMB"`
	TotalFiles   int     `json:"totalFiles"`
	TotalSeconds float64 `json:"totalSeconds"`
}

// savingsLedger führt das Sparbuch. Die Ergebnisse kommen aus mehreren
// Konvertern gleichzeitig, deshalb die Sperre: zwei Läufer, die im selben
// Augenblick fertig werden, dürfen sich nicht gegenseitig überschreiben.
type savingsLedger struct {
	mu   sync.Mutex
	book savingsBook

	// path ist die Datei. Leer heißt: nur im Speicher (Tests).
	path string
}

// newSavingsLedger öffnet das Sparbuch. Lässt es sich nicht lesen, fängt das
// Programm bei null an, statt den Start zu verweigern — eine Statistik ist
// nichts, wofür ein Fenster nicht aufgehen darf.
func newSavingsLedger() *savingsLedger {
	ledger := &savingsLedger{}
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
	raw, err := os.ReadFile(path)
	if err != nil {
		return savingsBook{}
	}
	var book savingsBook
	if json.Unmarshal(raw, &book) != nil {
		return savingsBook{}
	}
	return mergeLegacyDays(book)
}

// mergeLegacyDays rechnet die Tageszeilen einer älteren Datei in die
// Gesamtsummen ein und wirft sie danach weg.
//
// Ein Rückweg ist nicht vorgesehen und wird auch nicht gebraucht: Aus zwei
// Summen lassen sich keine Tage mehr machen, aber aus Tagen jederzeit Summen.
// Die Zeit fängt zwangsläufig bei null an — sie wurde vorher nie gemessen.
func mergeLegacyDays(book savingsBook) savingsBook {
	for _, day := range book.Days {
		book.Files += day.Files
		book.MB += day.MB
	}
	book.Days = nil
	return book
}

// savingsPath nennt den Ort des Sparbuchs.
func savingsPath() (string, error) {
	return dataFilePath(savingsFileName)
}

// Add bucht eine fertig umgewandelte Datei ein und schreibt sofort weg.
//
// savedMB darf negativ sein: Auch das ist ein ehrliches Ergebnis (eine Datei
// kann größer werden), und es stillschweigend als Null zu zählen würde die
// Bilanz schönen. Eine negative Dauer dagegen kann es nicht geben — sie wäre
// immer ein Messfehler (etwa eine zurückgestellte Uhr) und wird verworfen.
func (l *savingsLedger) Add(savedMB, seconds float64) SavingsReport {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.book.Files++
	l.book.MB += savedMB
	if seconds > 0 {
		l.book.Seconds += seconds
	}
	l.write()
	return l.report()
}

// Report liefert den Stand, ohne etwas zu ändern.
func (l *savingsLedger) Report() SavingsReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.report()
}

// Reset leert das Sparbuch. Der Knopf dafür steht auf der Einstellungsseite
// und nicht in der Leiste selbst: Was nicht rückgängig zu machen ist, gehört
// nicht neben eine Anzeige, an der man täglich vorbeisieht.
func (l *savingsLedger) Reset() SavingsReport {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.book = savingsBook{}
	l.write()
	return l.report()
}

// report macht aus dem Buch die Anzeige. Erwartet die Sperre.
func (l *savingsLedger) report() SavingsReport {
	return SavingsReport{
		TotalMB:      l.book.MB,
		TotalFiles:   l.book.Files,
		TotalSeconds: l.book.Seconds,
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

// ----------------------------------------------------------------------------
// Wie lange eine Datei gebraucht hat
// ----------------------------------------------------------------------------

// fileClock misst die Zeit je umgewandelter Datei: von der Meldung "diese
// Datei ist dran" bis zu ihrem Ergebnis.
//
// Warum je Datei und nicht je Stapel, obwohl der Konverter am Ende selbst eine
// Gesamtzeit meldet: Nur so ist die Zahl genauso absturzsicher wie die
// Ersparnis. Wer nach zehn von zwölf Dateien abstürzt, hat die zehn wirklich
// gerechnet — eine erst am Stapelende gebuchte Zeit wäre weg.
//
// Ehrlich dazugesagt: Laufen zwei Konverter gleichzeitig, ist die Summe größer
// als die Zeit, die am Fenster gewartet wurde. Gezählt wird die Arbeit, nicht
// die Wanduhr.
//
// Jeder Konverter hat seine eigene Platznummer, deshalb ist es eine Zuordnung
// und keine einzelne Zeitangabe — sonst würde die zweite gestartete Datei die
// Startzeit der ersten überschreiben.
type fileClock struct {
	mu      sync.Mutex
	started map[int]time.Time

	// now ist die Uhr. Als Feld, damit Tests eine Dauer nachstellen können,
	// ohne wirklich zu warten.
	now func() time.Time
}

func newFileClock() *fileClock {
	return &fileClock{started: map[int]time.Time{}, now: time.Now}
}

// Start hält fest, wann der Konverter auf diesem Platz mit einer Datei
// angefangen hat. Eine noch offene Messung desselben Platzes wird dabei
// überschrieben: Fängt dort eine neue Datei an, ist die alte nicht mehr zu
// Ende zu messen.
func (c *fileClock) Start(slot int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started[slot] = c.now()
}

// Stop liefert die vergangenen Sekunden und vergisst die Messung.
//
// Ist kein Anfang bekannt, kommt 0 zurück statt einer geschätzten Dauer: Eine
// erfundene Zahl in einer Statistik, die nie wieder korrigiert wird, ist
// schlimmer als eine fehlende.
func (c *fileClock) Stop(slot int) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	began, known := c.started[slot]
	if !known {
		return 0
	}
	delete(c.started, slot)
	return c.now().Sub(began).Seconds()
}

// slotFromEvent holt die Platznummer aus einem Ereignis. Sie wird in Go
// gesetzt (siehe Runner.emit) und ist deshalb ein echter int; der zweite Zweig
// fängt den Fall ab, dass sie einmal doch durch JSON gelaufen ist.
func slotFromEvent(event map[string]any) int {
	switch slot := event["slot"].(type) {
	case int:
		return slot
	case float64:
		return int(slot)
	}
	return 0
}

// eventIsFileStart meldet, ob mit diesem Ereignis eine neue Datei anfängt.
func eventIsFileStart(event map[string]any) bool {
	kind, _ := event["ev"].(string)
	return kind == "file"
}
