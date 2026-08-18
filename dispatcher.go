// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// dispatcher.go — mehrere Konverter-Läufe gleichzeitig, geordnet.
//
// Warum überhaupt mehrere: Gemessen am 2026-08-18 (4 Stücke à 90 s, 1080p) sind
// zwei gleichzeitige Läufe 26 % schneller als einer — die CPU-lastige
// Auto-CQ-Analyse des einen läuft neben dem GPU-Encode des anderen. Ein dritter
// bringt nichts mehr, die Karte ist dann ausgelastet.
//
// Warum EIN Prozess je Datei, statt allen Instanzen dieselbe Liste zu geben:
// Der Konverter verteilt sich zwar selbst über seine .lock-Dateien, aber es
// bleibt ein Wettrennen. Räumt die eine Instanz das fertige Original nach
// "originals", während die andere die Datei gerade prüft, meldet diese
// "No such file or directory" — die Datei ist in Wahrheit fertig, die Bilanz
// zeigt trotzdem einen Fehler. Gemessen: gleiche Geschwindigkeit (53 s gegen
// 52 s), aber saubere Meldungen. Details:
// _MediaForge-Beta\_claude_memory\messreihe-parallele-instanzen-2026-08-18.md
package main

import (
	"fmt"
	"sync"
)

// maxSlots ist die Obergrenze für gleichzeitige Läufe. Drei, weil die Messung
// oberhalb von zwei keinen Gewinn mehr zeigte und eine vierte Zahl nur eine
// Einstellung wäre, die nichts verbessert.
const maxSlots = 3

// job ist ein Auftrag, der auf einen freien Platz wartet.
type job struct {
	label string   // was in der Warteschlange angezeigt wird (Dateiname)
	args  []string // die vollständige Befehlszeile für genau diesen Auftrag
}

// QueueState sagt der Oberfläche, wie es um die Plätze steht.
type QueueState struct {
	Active  int `json:"active"`  // laufende Konverter
	Pending int `json:"pending"` // Aufträge, die noch warten
	Limit   int `json:"limit"`   // wie viele gleichzeitig laufen dürfen
}

// Dispatcher verwaltet die Plätze und reicht wartende Aufträge nach.
type Dispatcher struct {
	mu      sync.Mutex
	runners []*Runner
	pending []job
	limit   int
	exePath string
	workDir string
	emit    func(name string, data ...any)
}

// NewDispatcher erzeugt den Verteiler mit seinen Plätzen. Die Läufer entstehen
// einmal und bleiben; ein Platz ist einfach frei, wenn auf ihm nichts läuft.
func NewDispatcher(emit func(name string, data ...any)) *Dispatcher {
	dispatcher := &Dispatcher{limit: 1, emit: emit}
	for slot := 1; slot <= maxSlots; slot++ {
		dispatcher.runners = append(dispatcher.runners, NewRunner(slot, dispatcher.forward))
	}
	return dispatcher
}

// forward reicht jede Meldung eines Läufers an die Oberfläche weiter — und
// nimmt das Ende eines Laufs zum Anlass, den nächsten Auftrag zu starten.
func (d *Dispatcher) forward(name string, data ...any) {
	d.emit(name, data...)

	if name != "conv:state" || len(data) != 1 {
		return
	}
	state, ok := data[0].(RunState)
	if !ok || state.Running {
		return
	}
	d.fill()
	d.announceQueue()
}

// Submit nimmt Aufträge an und startet, was auf die freien Plätze passt.
//
// limit ist die Zahl gleichzeitiger Läufe, die der Nutzer eingestellt hat.
// Sie wird bei jedem Auftrag neu gesetzt: Wer sie ändert, während etwas läuft,
// soll das beim nächsten Start spüren, ohne erst alles anhalten zu müssen.
func (d *Dispatcher) Submit(exePath, workDir string, limit int, jobs []job) error {
	if len(jobs) == 0 {
		return fmt.Errorf("dispatcher.go: Submit: nothing to do")
	}

	d.mu.Lock()
	d.exePath = exePath
	d.workDir = workDir
	d.limit = clampSlots(limit)
	d.pending = append(d.pending, jobs...)
	d.mu.Unlock()

	d.fill()
	d.announceQueue()
	return nil
}

// clampSlots hält die Zahl im erlaubten Bereich. Eine 0 aus einem leeren
// Auswahlfeld würde sonst jeden Lauf verhindern.
func clampSlots(limit int) int {
	if limit < 1 {
		return 1
	}
	if limit > maxSlots {
		return maxSlots
	}
	return limit
}

// fill startet wartende Aufträge, solange Plätze frei sind.
func (d *Dispatcher) fill() {
	for {
		d.mu.Lock()
		if len(d.pending) == 0 || d.activeCount() >= d.limit {
			d.mu.Unlock()
			return
		}
		runner := d.freeRunner()
		if runner == nil {
			d.mu.Unlock()
			return
		}
		next := d.pending[0]
		d.pending = d.pending[1:]
		exePath, workDir := d.exePath, d.workDir
		d.mu.Unlock()

		if err := runner.Start(exePath, workDir, next.args); err != nil {
			// Der Platz bleibt frei, also weiter mit dem nächsten Auftrag —
			// aber der Fehler gehört sichtbar ins Protokoll, sonst verschwände
			// eine Datei stillschweigend aus der Warteschlange.
			d.emit("conv:log", LogLine{Text: "[gui] could not start " + next.label + ": " + err.Error()})
			continue
		}
	}
}

// activeCount zählt die laufenden Plätze. Nur mit gehaltener Sperre aufrufen.
func (d *Dispatcher) activeCount() int {
	active := 0
	for _, runner := range d.runners {
		if runner.Running() {
			active++
		}
	}
	return active
}

// freeRunner sucht einen Platz, auf dem nichts läuft. Nur mit gehaltener
// Sperre aufrufen.
func (d *Dispatcher) freeRunner() *Runner {
	for _, runner := range d.runners {
		if !runner.Running() {
			return runner
		}
	}
	return nil
}

// announceQueue meldet den Stand der Plätze an die Oberfläche.
func (d *Dispatcher) announceQueue() {
	d.emit("conv:queue", d.QueueStatus())
}

// QueueStatus liefert den aktuellen Stand.
func (d *Dispatcher) QueueStatus() QueueState {
	d.mu.Lock()
	defer d.mu.Unlock()
	return QueueState{Active: d.activeCount(), Pending: len(d.pending), Limit: d.limit}
}

// Busy sagt, ob überhaupt noch etwas zu tun ist — laufend oder wartend.
func (d *Dispatcher) Busy() bool {
	status := d.QueueStatus()
	return status.Active > 0 || status.Pending > 0
}

// RequestStop hält alles an: Erst den Vorrat leeren, dann die laufenden
// Konverter bitten, sauber aufzuhören.
//
// Reihenfolge mit Absicht: Würden zuerst die Läufe abgebrochen, startete das
// Nachrücken sofort den nächsten wartenden Auftrag — ein Abbruch, der neue
// Arbeit anfängt.
func (d *Dispatcher) RequestStop() error {
	d.mu.Lock()
	d.pending = nil
	runners := append([]*Runner{}, d.runners...)
	d.mu.Unlock()

	var firstErr error
	for _, runner := range runners {
		if !runner.Running() {
			continue
		}
		if err := runner.RequestStop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	d.announceQueue()
	return firstErr
}

// StopSlot hält genau EINEN Konverter an.
//
// Der Vorrat bleibt unangetastet und die übrigen Plätze laufen weiter: Wer
// eine einzelne Datei abbricht, will nicht den ganzen Stapel verlieren. Auf dem
// frei werdenden Platz rückt deshalb auch der nächste Auftrag nach — das ist
// derselbe Weg wie bei einer regulär fertigen Datei.
func (d *Dispatcher) StopSlot(slot int) error {
	runner := d.runnerFor(slot)
	if runner == nil {
		return fmt.Errorf("dispatcher.go: StopSlot: there is no slot %d", slot)
	}
	return runner.RequestStop()
}

// runnerFor sucht den Läufer eines Platzes.
func (d *Dispatcher) runnerFor(slot int) *Runner {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, runner := range d.runners {
		if runner.Slot() == slot {
			return runner
		}
	}
	return nil
}

// Answer beantwortet die Rückfrage eines bestimmten Platzes.
//
// Die Nummer kommt aus dem Frage-Ereignis. Ohne sie ginge die Antwort bei zwei
// gleichzeitigen Läufen womöglich an den falschen Konverter — und der bekäme
// eine Spurauswahl, die für eine ganz andere Datei gedacht war.
func (d *Dispatcher) Answer(slot int, text string) error {
	runner := d.runnerFor(slot)
	if runner == nil {
		return fmt.Errorf("dispatcher.go: Answer: there is no slot %d", slot)
	}
	return runner.Answer(text)
}
