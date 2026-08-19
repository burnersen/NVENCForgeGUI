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

// Die beiden Bereiche, aus denen Arbeit kommt. Sie teilen sich die Grafikkarte,
// aber sonst nichts: Wer von Hand einen Stapel umwandelt, soll davon nichts
// mitbekommen, dass nebenbei ein beobachteter Ordner abgearbeitet wird — und
// umgekehrt.
const (
	areaConvert = "convert"
	areaWatch   = "watch"
	areaSplit   = "split"
	areaJoin    = "join"
)

// areas nennt sie in der Reihenfolge, in der die Oberfläche sie zeigt.
var areas = []string{areaConvert, areaSplit, areaJoin, areaWatch}

// Nur diese beiden nehmen die Grafikkarte in Anspruch und teilen sich deshalb
// die gemeinsame Obergrenze. Zerlegen und Zusammenfügen kopieren bloß Spuren —
// dort ist die Festplatte die Grenze, nicht die Karte —, also stehen sie
// daneben und nicht dahinter.
func usesTheCard(area string) bool {
	return area == areaConvert || area == areaWatch
}

// Die Platzvergabe.
//
// Plätze 1..3 gehören dem Umwandeln von Hand, Platz 4 ist fest für den
// beobachteten Ordner reserviert. Feste Nummern statt einer frei vergebenen
// Zuteilung, weil die Fensterseite an der Nummer allein erkennen muss, in
// welche Anzeige eine Meldung gehört — sonst müsste jede einzelne Meldung
// zusätzlich sagen, woher sie kommt.
const (
	maxConvertSlots = 3
	watchSlot       = 4
	splitSlot       = 5
	joinSlot        = 6
)

// Wie viele Konverter überhaupt gleichzeitig laufen dürfen, über beide Bereiche
// zusammen.
//
// Drei, weil die Messung vom 2026-08-18 oberhalb von zwei keinen Gewinn mehr
// zeigte — die Karte ist dann ausgelastet, ein vierter Lauf verteilt dieselbe
// Arbeit nur auf mehr Prozesse. Rechnet einer davon auf der CPU, sind es zwei:
// dort kostet jeder weitere Lauf echte Kerne, die dem anderen fehlen.
const (
	maxTotalSlots    = 3
	maxTotalSlotsCPU = 2
)

// job ist ein Auftrag, der auf einen freien Platz wartet.
type job struct {
	label string   // was in der Warteschlange angezeigt wird (Dateiname)
	args  []string // die vollständige Befehlszeile für genau diesen Auftrag
	area  string   // aus welchem Bereich er kommt (areaConvert / areaWatch)
}

// AreaState ist der Stand EINES Bereichs.
type AreaState struct {
	Active  int `json:"active"`  // laufende Konverter dort
	Pending int `json:"pending"` // Aufträge dort, die noch warten
	Limit   int `json:"limit"`   // wie viele davon gleichzeitig laufen dürfen
}

// QueueState sagt der Oberfläche, wie es um die Plätze steht — für jeden
// Bereich einzeln.
//
// Eine Karte statt einzelner Felder je Bereich: Die Oberfläche zeigt jeden
// Bereich für sich, und ein fünfter Bereich hieße sonst wieder drei neue
// Felder, die überall mitgeschleppt werden müssen.
//
// TotalLimit ist die gemeinsame Obergrenze der Bereiche, die die Karte
// belegen. Sie erklärt der Oberfläche, warum sie beim Umwandeln vielleicht
// weniger anbieten darf, als der Nutzer eingestellt hat.
type QueueState struct {
	Areas      map[string]AreaState `json:"areas"`
	TotalLimit int                  `json:"totalLimit"`
}

// Dispatcher verwaltet die Plätze und reicht wartende Aufträge nach.
type Dispatcher struct {
	mu      sync.Mutex
	runners []*Runner
	pending []job
	limit   int             // gleichzeitige Läufe im Bereich Umwandeln
	usesCPU map[string]bool // je Bereich: rechnet dort einer auf der CPU?
	exePath string
	workDir string
	emit    func(name string, data ...any)
}

// NewDispatcher erzeugt den Verteiler mit seinen Plätzen. Die Läufer entstehen
// einmal und bleiben; ein Platz ist einfach frei, wenn auf ihm nichts läuft.
func NewDispatcher(emit func(name string, data ...any)) *Dispatcher {
	dispatcher := &Dispatcher{limit: 1, usesCPU: map[string]bool{}, emit: emit}
	for slot := 1; slot <= maxConvertSlots; slot++ {
		dispatcher.runners = append(dispatcher.runners, NewRunner(slot, dispatcher.forward))
	}
	for _, slot := range []int{watchSlot, splitSlot, joinSlot} {
		dispatcher.runners = append(dispatcher.runners, NewRunner(slot, dispatcher.forward))
	}
	return dispatcher
}

// areaOfSlot sagt, wohin ein Platz gehört. Die Fensterseite rechnet genauso.
func areaOfSlot(slot int) string {
	switch slot {
	case watchSlot:
		return areaWatch
	case splitSlot:
		return areaSplit
	case joinSlot:
		return areaJoin
	default:
		return areaConvert
	}
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
// area sagt, aus welchem Bereich die Aufträge kommen; usesCPU, ob dort auf dem
// Prozessor gerechnet wird — davon hängt die gemeinsame Obergrenze ab.
func (d *Dispatcher) Submit(exePath, workDir, area string, limit int, usesCPU bool, jobs []job) error {
	if len(jobs) == 0 {
		return fmt.Errorf("dispatcher.go: Submit: nothing to do")
	}
	if !knownArea(area) {
		return fmt.Errorf("dispatcher.go: Submit: unknown area %q", area)
	}

	d.mu.Lock()
	d.exePath = exePath
	d.workDir = workDir
	// Die eingestellte Zahl gilt nur für das Umwandeln von Hand. Der
	// beobachtete Ordner hat immer genau einen Platz — mehr wäre eine zweite
	// Einstellung für etwas, das im Hintergrund laufen soll.
	if area == areaConvert {
		d.limit = clampSlots(limit)
	}
	d.usesCPU[area] = usesCPU
	for i := range jobs {
		jobs[i].area = area
	}
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
	if limit > maxConvertSlots {
		return maxConvertSlots
	}
	return limit
}

// knownArea weist einen vertippten Bereich ab, statt ihn still als
// „Umwandeln" durchgehen zu lassen.
func knownArea(area string) bool {
	for _, known := range areas {
		if known == area {
			return true
		}
	}
	return false
}

// totalLimit nennt die gemeinsame Obergrenze der Bereiche, die die Karte
// belegen. Nur mit gehaltener Sperre aufrufen.
//
// Es genügt, dass EINER von ihnen auf der CPU rechnet: Der Deckel schützt die
// Kerne, und die teilen sich alle.
func (d *Dispatcher) totalLimit() int {
	for area, cpu := range d.usesCPU {
		if cpu && usesTheCard(area) {
			return maxTotalSlotsCPU
		}
	}
	return maxTotalSlots
}

// limitOf nennt die Obergrenze eines einzelnen Bereichs. Nur mit gehaltener
// Sperre aufrufen.
//
// Alles außer dem Umwandeln von Hand hat genau einen Platz: Der beobachtete
// Ordner soll im Hintergrund bleiben, und Zerlegen wie Zusammenfügen fragen
// nach Spuren — zwei solche Fragen gleichzeitig wären eine Zumutung.
func (d *Dispatcher) limitOf(area string) int {
	if area == areaConvert {
		return d.limit
	}
	return 1
}

// cardCount zählt die Läufe, die gerade die Karte belegen. Nur mit gehaltener
// Sperre aufrufen.
func (d *Dispatcher) cardCount() int {
	active := 0
	for _, runner := range d.runners {
		if runner.Running() && usesTheCard(areaOfSlot(runner.Slot())) {
			active++
		}
	}
	return active
}

// fill startet wartende Aufträge, solange Plätze frei sind.
//
// Es wird nicht stur der vorderste Auftrag genommen: Ein wartender Stapel aus
// dem Umwandeln darf einen Fund des beobachteten Ordners nicht aufhalten und
// umgekehrt. Gesucht wird deshalb der erste Auftrag, für dessen Bereich es
// gerade wirklich einen Platz gibt.
func (d *Dispatcher) fill() {
	for {
		d.mu.Lock()
		if len(d.pending) == 0 {
			d.mu.Unlock()
			return
		}
		index, runner := d.nextStartable()
		if runner == nil {
			d.mu.Unlock()
			return
		}
		next := d.pending[index]
		d.pending = append(d.pending[:index], d.pending[index+1:]...)
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

// nextStartable sucht den ersten wartenden Auftrag, der jetzt anfangen kann,
// und den Platz für ihn. Nur mit gehaltener Sperre aufrufen.
func (d *Dispatcher) nextStartable() (int, *Runner) {
	for index, waiting := range d.pending {
		if d.activeIn(waiting.area) >= d.limitOf(waiting.area) {
			continue
		}
		// Die gemeinsame Obergrenze bremst nur die Bereiche, die wirklich die
		// Karte belegen. Ein Zerlegen daneben kopiert bloß Spuren und darf
		// nicht warten, bis ein Stapel durch ist.
		if usesTheCard(waiting.area) && d.cardCount() >= d.totalLimit() {
			continue
		}
		if runner := d.freeRunnerFor(waiting.area); runner != nil {
			return index, runner
		}
	}
	return -1, nil
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

// activeIn zählt die laufenden Plätze eines Bereichs. Nur mit gehaltener
// Sperre aufrufen.
func (d *Dispatcher) activeIn(area string) int {
	active := 0
	for _, runner := range d.runners {
		if runner.Running() && areaOfSlot(runner.Slot()) == area {
			active++
		}
	}
	return active
}

// pendingIn zählt die wartenden Aufträge eines Bereichs. Nur mit gehaltener
// Sperre aufrufen.
func (d *Dispatcher) pendingIn(area string) int {
	waiting := 0
	for _, entry := range d.pending {
		if entry.area == area {
			waiting++
		}
	}
	return waiting
}

// freeRunnerFor sucht einen freien Platz des Bereichs. Nur mit gehaltener
// Sperre aufrufen.
func (d *Dispatcher) freeRunnerFor(area string) *Runner {
	for _, runner := range d.runners {
		if areaOfSlot(runner.Slot()) != area {
			continue
		}
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
	state := QueueState{Areas: make(map[string]AreaState, len(areas)), TotalLimit: d.totalLimit()}
	for _, area := range areas {
		state.Areas[area] = AreaState{
			Active:  d.activeIn(area),
			Pending: d.pendingIn(area),
			Limit:   d.limitOf(area),
		}
	}
	return state
}

// BusyIn sagt, ob in einem Bereich noch etwas zu tun ist.
func (d *Dispatcher) BusyIn(area string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.activeIn(area) > 0 || d.pendingIn(area) > 0
}

// Busy sagt, ob überhaupt noch etwas zu tun ist — laufend oder wartend, in
// welchem Bereich auch immer. Das Fenster fragt danach, bevor es sich
// schließen lässt.
func (d *Dispatcher) Busy() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.activeCount() > 0 || len(d.pending) > 0
}

// RequestStop hält alles an: Erst den Vorrat leeren, dann die laufenden
// Konverter bitten, sauber aufzuhören.
//
// Reihenfolge mit Absicht: Würden zuerst die Läufe abgebrochen, startete das
// Nachrücken sofort den nächsten wartenden Auftrag — ein Abbruch, der neue
// Arbeit anfängt.
func (d *Dispatcher) RequestStop() error { return d.stop("") }

// RequestStopArea hält nur einen Bereich an. Der andere läuft weiter — das ist
// der ganze Sinn der Trennung: Wer seinen Stapel abbricht, will nicht nebenbei
// den beobachteten Ordner stilllegen.
func (d *Dispatcher) RequestStopArea(area string) error { return d.stop(area) }

// stop hält alles an oder nur einen Bereich. Ein leerer Bereich heißt „alles".
func (d *Dispatcher) stop(area string) error {
	d.mu.Lock()
	if area == "" {
		d.pending = nil
	} else {
		kept := d.pending[:0]
		for _, entry := range d.pending {
			if entry.area != area {
				kept = append(kept, entry)
			}
		}
		d.pending = kept
	}
	runners := make([]*Runner, 0, len(d.runners))
	for _, runner := range d.runners {
		if area == "" || areaOfSlot(runner.Slot()) == area {
			runners = append(runners, runner)
		}
	}
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
