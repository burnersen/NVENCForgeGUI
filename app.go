// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// app.go — die Brücke zwischen dem Fenster und dem Konverter.
//
// Alles, was die Oberfläche aufrufen darf, steht hier als Methode von App.
// Wails macht daraus automatisch die JavaScript-Seite; deshalb bleibt jede
// Methode klein und tut genau eine Sache.
package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App hält den Zustand des Fensters.
type App struct {
	ctx        context.Context
	dispatcher *Dispatcher
	watcher    *FolderWatcher

	// window ist der beim Start geladene Fensterzustand. windowRemembered sagt,
	// ob er aus der Merkdatei stammt — beim allerersten Start gibt es noch
	// keinen Platz, den man wiederherstellen könnte.
	window           windowState
	windowRemembered bool

	// savings ist das Sparbuch: wie viel Platz das Umwandeln gebracht hat.
	savings *savingsLedger

	// profiles sind die gespeicherten Optionssätze der Konvertieren-Seite.
	profiles *profileStore
}

// NewApp erzeugt die Anwendung mit dem Fensterzustand, mit dem sie startet.
func NewApp(window windowState, windowRemembered bool) *App {
	app := &App{window: window, windowRemembered: windowRemembered}
	app.savings = newSavingsLedger()
	app.profiles = newProfileStore()
	// Der Verteiler meldet NICHT mehr geradewegs an die Oberfläche, sondern
	// über bookAndForward: Was gespart wurde, wird im Vorbeigehen eingetragen.
	app.dispatcher = NewDispatcher(app.bookAndForward)
	// Der Beobachter meldet nur, WAS er gefunden hat. Ob daraufhin ein Lauf
	// beginnt, entscheidet die Oberfläche: Sie allein weiß, ob gerade schon
	// konvertiert wird und mit welchen Einstellungen.
	app.watcher = NewFolderWatcher(
		func(items []QueueItem) { app.emit("watch:files", items) },
		app.noteWatch,
	)
	return app
}

// startup merkt sich den Zusammenhang, den Wails für Ereignisse und Dialoge
// braucht.
//
// Das Hineinziehen von Dateien wird NICHT hier angemeldet: Die Ereigniskette
// wird erst scharf, wenn die Fensterseite runtime.OnFileDrop aufruft. Ohne das
// öffnet die eingebaute Webansicht die Datei einfach selbst — genau dieser
// Fehler ist am 2026-08-17 aufgefallen. Der Aufruf steht deshalb in index.html,
// dort wo er wirkt.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.restoreWindowPlace()
}

// restoreWindowPlace schiebt das Fenster auf seinen gemerkten Platz.
//
// Passt der Platz nicht mehr — abgezogener zweiter Bildschirm, geänderte
// Auflösung —, bleibt es bei der Mitte, die Wails von sich aus wählt. Ein
// Fenster außerhalb des Sichtbaren wäre von einem Absturz nicht zu
// unterscheiden.
func (a *App) restoreWindowPlace() {
	if !a.windowRemembered || a.window.Maximised {
		return
	}
	if !rectIsOnAScreen(a.window.X, a.window.Y, a.window.Width, a.window.Height) {
		return
	}
	wailsruntime.WindowSetPosition(a.ctx, a.window.X, a.window.Y)
}

// ----------------------------------------------------------------------------
// Der Fensterrahmen als Fortschrittsanzeige
//
// Wer einen Stapel anwirft, verkleinert das Fenster und arbeitet weiter. Damit
// er nicht dauernd nachsehen muss, trägt der Rahmen selbst den Stand: die
// Prozentzahl im Titel (Windows zeigt ihn am Taskleisten-Knopf) und der Balken
// im Knopf. Am Ende eines Stapels blinkt der Knopf — ohne Ton, so gewünscht.
//
// Wie viel Prozent es sind, rechnet die Fensterseite aus: Nur sie kennt alle
// Listen und meldet sich erst, wenn sich die ganze Zahl wirklich geändert hat.
// ----------------------------------------------------------------------------

// ShowProgress meldet den Gesamtfortschritt der laufenden Stapel.
func (a *App) ShowProgress(percent int) {
	a.setWindowTitle(progressTitle(percent))
	showTaskbarProgress(percent)
}

// ShowBusy meldet "es läuft", wo es keine ehrliche Prozentzahl gibt: Beim
// Zusammenfügen entsteht EINE Datei, da ist nichts abzuzählen.
func (a *App) ShowBusy() {
	a.setWindowTitle(busyTitle())
	showTaskbarBusy()
}

// HideProgress stellt den Ruhezustand her — Titel wie beim Start, kein Balken.
func (a *App) HideProgress() {
	a.setWindowTitle(baseWindowTitle())
	hideTaskbarProgress()
}

// SignalDone ist die Fertig-Meldung am Ende eines Stapels.
func (a *App) SignalDone() {
	flashTaskbar()
}

// setWindowTitle schreibt den Titel, sobald es ein Fenster gibt.
func (a *App) setWindowTitle(title string) {
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowSetTitle(a.ctx, title)
}

// rememberWindow schreibt Größe und Platz des Fensters weg.
//
// Ist das Fenster maximiert oder zum Symbol verkleinert, bleiben die Maße
// unangetastet: Gespeichert würde sonst die Bildschirmgröße bzw. Unsinn, und
// „verkleinern" hätte nach dem nächsten Start keine sinnvolle Größe mehr, zu
// der es zurückkehren könnte.
func (a *App) rememberWindow() {
	if a.ctx == nil {
		return
	}
	state, ok := loadWindowState()
	if !ok {
		state = defaultWindowState()
	}
	state.Maximised = wailsruntime.WindowIsMaximised(a.ctx)
	if !state.Maximised && !wailsruntime.WindowIsMinimised(a.ctx) {
		state.Width, state.Height = wailsruntime.WindowGetSize(a.ctx)
		state.X, state.Y = wailsruntime.WindowGetPosition(a.ctx)
	}
	if err := saveWindowState(state); err != nil {
		// Kein Grund, das Schließen aufzuhalten — es geht nur um Bequemlichkeit.
		// Das Fenster öffnet beim nächsten Mal eben in Standardgröße.
		a.note("Could not remember the window size: " + err.Error())
	}
}

// beforeClose verhindert das Schließen, solange ein Lauf aktiv ist.
//
// Grund: Beim Schließen bräche die Verbindung zum Konverter ab, er liefe aber
// unsichtbar weiter. Lieber ein klarer Hinweis als ein Programm, das im
// Hintergrund weiterrechnet, ohne dass es jemand sieht.
func (a *App) beforeClose(ctx context.Context) bool {
	if a.dispatcher.Busy() {
		a.note("A conversion is still running — stop it first, then close the window.")
		return true
	}
	// Hier und nicht in OnShutdown: Das Fenster steht jetzt noch, seine Größe
	// lässt sich also überhaupt noch erfragen.
	a.rememberWindow()
	return false
}

// onSecondInstance holt das bereits offene Fenster nach vorn, wenn das Programm
// ein zweites Mal gestartet wird.
//
// Zwei Fenster wären kein harmloser Doppelstart: Beide würden denselben
// überwachten Ordner abgrasen und sich um dieselben Dateien streiten.
func (a *App) onSecondInstance(options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.WindowShow(a.ctx)
	a.note("NVENCForgeGUI is already open — this is that window.")
}

// note schreibt eine Meldung des Fensters selbst ins Protokoll. Die Vorsilbe
// macht auf einen Blick klar, dass sie nicht vom Konverter stammt.
func (a *App) note(text string) {
	a.emit("conv:log", LogLine{Text: "[gui] " + text})
}

// noteWatch ist dasselbe für den beobachteten Ordner. Die Platznummer ist das
// Einzige, woran die Fensterseite erkennt, in welches der beiden Protokolle
// eine Zeile gehört — ohne sie landete jede Meldung des Ordners mitten im
// Protokoll eines von Hand gestarteten Stapels.
func (a *App) noteWatch(text string) {
	a.emit("conv:log", LogLine{Text: "[gui] " + text, Slot: watchSlot})
}

// bookAndForward trägt jede fertig umgewandelte Datei ins Sparbuch ein und
// reicht die Meldung dann weiter wie bisher.
//
// Warum hier und nicht in der Fensterseite: Die Zahl soll auch dann stimmen,
// wenn niemand hinsieht. Die Oberfläche zeigt nur an, was hier gezählt wurde —
// eine zweite Buchführung im Fenster wäre eine zweite Wahrheit, und beim
// nächsten Start wäre nicht mehr zu sagen, welche die richtige war.
func (a *App) bookAndForward(name string, data ...any) {
	if name == "conv:event" && len(data) == 1 {
		if event, ok := data[0].(map[string]any); ok {
			if savedMB, counted := savedMBFromEvent(event); counted {
				a.emit("conv:savings", a.savings.Add(savedMB))
			}
		}
	}
	a.emit(name, data...)
}

// GetSavings liefert den Stand für die Leiste unten im Fenster.
func (a *App) GetSavings() SavingsReport {
	return a.savings.Report()
}

// ResetSavings setzt die Statistik zurück. Der Knopf dafür steht auf der
// Einstellungsseite, nicht in der Leiste: Was sich nicht rückgängig machen
// lässt, gehört nicht neben eine Anzeige, an der man täglich vorbeisieht.
func (a *App) ResetSavings() SavingsReport {
	report := a.savings.Reset()
	a.emit("conv:savings", report)
	return report
}

// GetProfiles liefert die gespeicherten Optionssätze, nach Namen sortiert.
func (a *App) GetProfiles() []Profile {
	return a.profiles.List()
}

// SaveProfile legt einen Optionssatz unter seinem Namen ab und liefert die
// neue Liste zurück. Ein Fehler wird ausdrücklich weitergereicht: Wer
// "Speichern" drückt, muss erfahren, wenn nichts gespeichert wurde.
func (a *App) SaveProfile(profile Profile) ([]Profile, error) {
	return a.profiles.Save(profile)
}

// DeleteProfile nimmt einen Optionssatz wieder heraus.
func (a *App) DeleteProfile(name string) ([]Profile, error) {
	return a.profiles.Delete(name)
}

// emit schickt eine Meldung an die Oberfläche. Vor dem Start des Fensters gibt
// es noch keinen Empfänger; dann verfällt die Meldung still, statt abzustürzen.
func (a *App) emit(name string, data ...any) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, name, data...)
}

// StartupInfo ist das, was die Oberfläche beim Start einmal abfragt.
type StartupInfo struct {
	GUIVersion string          `json:"guiVersion"`
	GPU        GPUInfo         `json:"gpu"`
	Converter  ConverterStatus `json:"converter"`
	Config     ConfigView      `json:"config"`
}

// GetStartupInfo liefert die Version dieses Fensters, Grafikkarte, Zustand
// der Programmdatei und die Einstellungen, die gerade gelten.
func (a *App) GetStartupInfo() StartupInfo {
	return StartupInfo{
		GUIVersion: guiVersion,
		GPU:        queryGPU(),
		Converter:  converterStatus(),
		Config:     readConfigView(),
	}
}

// GetTheme nennt die zuletzt gewählte Farbstimmung.
//
// Bewusst ein eigener, winziger Aufruf und nicht Teil von GetStartupInfo:
// Der Startbericht befragt die Grafikkarte und braucht dafür spürbar Zeit.
// Die Oberfläche muss aber schon vor dem ersten Bild wissen, welche Farben
// sie malen soll — sonst blitzt bei heller Stimmung kurz der dunkle Grund auf.
func (a *App) GetTheme() string {
	return normaliseTheme(a.window.Theme)
}

// SaveTheme merkt sich die gewählte Farbstimmung sofort.
//
// Sofort und nicht erst beim Schließen: Wer umschaltet und das Fenster danach
// über den Aufgabenmanager verliert, soll seine Wahl trotzdem wiederfinden.
func (a *App) SaveTheme(theme string) error {
	a.window.Theme = normaliseTheme(theme)
	return saveTheme(a.window.Theme)
}

// GetConverterStatus prüft die Programmdatei erneut.
func (a *App) GetConverterStatus() ConverterStatus {
	return converterStatus()
}

// NeedsSetup meldet, ob dem Konverter noch seine Erstausstattung fehlt — also
// INI und eigenes FFmpeg (die Begründung steht im Kopf von setup.go).
func (a *App) NeedsSetup() bool {
	return needsSetup(converterStatus())
}

// RunSetup lässt den Konverter einmal laufen, damit er sich einrichtet.
//
// Der Aufruf kehrt sofort zurück; die Ausgabe läuft ins Protokoll, und am Ende
// meldet das Ereignis "conv:setup", ob es geklappt hat. Grund: Der
// FFmpeg-Download dauert, und ein Fenster, das minutenlang nicht reagiert, sähe
// abgestürzt aus.
func (a *App) RunSetup() {
	status := converterStatus()
	if !status.Found {
		a.note("NVENCForge.exe was not found — download it first.")
		return
	}
	a.note("Setting NVENCForge up: it writes its configuration and fetches its own FFmpeg. This can take a minute.")
	go func() {
		err := runSetup(status, func(text string) {
			a.emit("conv:log", LogLine{Text: text})
		})
		if err != nil {
			a.note("Setup did not finish: " + err.Error())
			a.emit("conv:setup", false)
			return
		}
		a.note("NVENCForge is ready — its settings can be edited now.")
		a.emit("conv:setup", true)
	}()
}

// GetSettingsFile liefert alle Einstellungen für den Bereich "Settings" —
// Werte, Erklärungen, erlaubte Bereiche und Standardwerte, wie sie in der INI
// stehen.
func (a *App) GetSettingsFile() SettingsFile {
	return readSettingsFile()
}

// SaveSettings schreibt die geänderten Werte zurück.
//
// Ein laufender Lauf wird NICHT blockiert: Der Konverter liest seine INI beim
// Start, geänderte Werte gelten also ab dem nächsten Lauf. Das sagt die
// Oberfläche dazu, statt das Speichern zu verbieten.
func (a *App) SaveSettings(values map[string]string) (SaveResult, error) {
	result, err := writeSettings(values)
	if err != nil {
		return result, err
	}
	if result.Written > 0 {
		a.note(fmt.Sprintf("Saved %d setting(s). Previous version kept as %s",
			result.Written, filepath.Base(result.BackupPath)))
	}
	return result, nil
}

// GetSRTCleaner liefert die Phrasenliste des Untertitel-Reinigers.
func (a *App) GetSRTCleaner() SRTCleanerView {
	return readSRTCleaner()
}

// SaveSRTCleaner schreibt die Phrasenliste zurück.
//
// Wie bei den Einstellungen bleibt die vorige Fassung als .bak liegen, und ein
// laufender Lauf wird nicht aufgehalten — der Konverter liest die Liste beim
// Start eines Laufs.
func (a *App) SaveSRTCleaner(phrases []SRTPhrase) (SaveResult, error) {
	result, err := writeSRTCleaner(phrases)
	if err != nil {
		return result, err
	}
	a.note(fmt.Sprintf("Saved %d filter phrase(s). Previous version kept as %s",
		result.Written, filepath.Base(result.BackupPath)))
	return result, nil
}

// GetConfigView liest die INI erneut.
//
// Nötig, weil sie erst entsteht, wenn NVENCForge zum ersten Mal läuft: Beim
// Start des Fensters kann sie also noch fehlen und nach dem ersten Lauf da
// sein.
func (a *App) GetConfigView() ConfigView {
	return readConfigView()
}

// DownloadConverter holt die neueste NVENCForge.exe von GitHub.
//
// force=true spielt sie auch dann ein, wenn dabei der Datenkanal verloren geht.
func (a *App) DownloadConverter(force bool) (DownloadResult, error) {
	return downloadConverter(a.ctx, force, func(done, total int64) {
		a.emit("conv:download", map[string]any{"done": done, "total": total})
	})
}

// AddPaths nimmt Pfade entgegen, die ins Fenster gezogen wurden, und macht
// daraus Einträge für die Warteschlange.
func (a *App) AddPaths(paths []string) []QueueItem {
	items, rejected := expandPaths(paths)
	a.noteRejected(rejected)
	return items
}

// noteRejected sagt, welche abgelegten Dateien hier nichts verloren haben.
//
// Warum überhaupt: Diese Liste nimmt nur Videos. Ton- und Untertiteldateien
// gehören in den Bereich "Join" — wer sie hier ablegt, hat sich in der Seite
// geirrt und soll das erfahren, statt sie verschwinden zu sehen.
func (a *App) noteRejected(rejected []string) {
	if len(rejected) == 0 {
		return
	}
	const maxNamed = 3
	named := rejected
	suffix := ""
	if len(named) > maxNamed {
		named = named[:maxNamed]
		suffix = fmt.Sprintf(" and %d more", len(rejected)-maxNamed)
	}
	a.note(fmt.Sprintf(
		"Not a video, so not added: %s%s. Audio and subtitle files belong in the Join area.",
		strings.Join(named, ", "), suffix))
}

// PickFiles öffnet den Dateidialog.
func (a *App) PickFiles() ([]QueueItem, error) {
	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Add video files",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Video files", Pattern: videoFilterPattern()},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("app.go: PickFiles: %w", err)
	}
	items, rejected := expandPaths(paths)
	a.noteRejected(rejected)
	return items, nil
}

// SortJoinFiles ordnet die Ablage des Bereichs "Join" neu ein.
//
// Die Oberfläche schickt IMMER alle Pfade, die dort liegen — die bereits
// vorhandenen und die neu hinzugekommenen. Grund: Ob eine .sub verwendbar ist,
// entscheidet sich erst im Zusammenhang mit der gleichnamigen .idx (die
// ausführliche Begründung steht bei classifyJoinFiles).
func (a *App) SortJoinFiles(paths []string) []JoinFile {
	return classifyJoinFiles(paths)
}

// PickJoinFiles öffnet den Dateidialog für die Join-Ablage. Eigene Filter,
// weil hier auch Ton- und Untertiteldateien gesucht werden.
func (a *App) PickJoinFiles() ([]JoinFile, error) {
	video, audio, subtitle := joinFilterPatterns()
	paths, err := wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Add video, audio and subtitle files",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Everything that can be joined", Pattern: video + ";" + audio + ";" + subtitle},
			{DisplayName: "Video files", Pattern: video},
			{DisplayName: "Audio files", Pattern: audio},
			{DisplayName: "Subtitle files", Pattern: subtitle},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("app.go: PickJoinFiles: %w", err)
	}
	return classifyJoinFiles(paths), nil
}

// PickFolder öffnet den Ordnerdialog und sammelt alles darin ein.
func (a *App) PickFolder() ([]QueueItem, error) {
	folder, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Add a folder",
	})
	if err != nil {
		return nil, fmt.Errorf("app.go: PickFolder: %w", err)
	}
	if folder == "" {
		return nil, nil
	}
	// Ein ausgewählter ORDNER wird durchsucht; was darin nicht passt, ist kein
	// Irrtum des Benutzers und wird deshalb nicht gemeldet.
	items, _ := expandPaths([]string{folder})
	return items, nil
}

// WatchState ist das, was die Oberfläche über die Ordner-Beobachtung wissen
// muss.
type WatchState struct {
	Watching bool   `json:"watching"`
	Folder   string `json:"folder"`
}

// PickWatchFolder öffnet den Ordnerdialog für die Beobachtung. Er sammelt
// nichts ein — der Ordner wird nur benannt.
func (a *App) PickWatchFolder() (string, error) {
	folder, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Choose the folder to watch",
	})
	if err != nil {
		return "", fmt.Errorf("app.go: PickWatchFolder: %w", err)
	}
	return folder, nil
}

// StartWatching beginnt die Beobachtung eines Ordners.
func (a *App) StartWatching(folder string) (WatchState, error) {
	if err := a.watcher.Start(folder); err != nil {
		return a.WatchStatus(), fmt.Errorf("app.go: StartWatching: %w", err)
	}
	a.noteWatch("Watching " + folder + " and its subfolders. New videos are converted once they stop growing.")
	return a.WatchStatus(), nil
}

// StopWatching beendet die Beobachtung. Ein laufender Lauf bleibt davon
// unberührt — abgebrochen wird nur über den Stop-Knopf.
func (a *App) StopWatching() WatchState {
	if a.watcher.Watching() {
		a.noteWatch("Stopped watching. What is already running is left to finish.")
	}
	a.watcher.Stop()
	return a.WatchStatus()
}

// WatchStatus sagt, ob und was gerade beobachtet wird.
func (a *App) WatchStatus() WatchState {
	return WatchState{Watching: a.watcher.Watching(), Folder: a.watcher.Folder()}
}

// IsRunning meldet, ob noch etwas zu tun ist — laufend oder wartend.
func (a *App) IsRunning() bool {
	return a.dispatcher.Busy()
}

// GetQueueStatus liefert den Stand der Plätze (laufend, wartend, Obergrenze).
func (a *App) GetQueueStatus() QueueState {
	return a.dispatcher.QueueStatus()
}

// StartRun reiht die Arbeit ein und startet, was auf die freien Plätze passt.
func (a *App) StartRun(request RunRequest) error {
	status := converterStatus()
	if !status.Found {
		return errors.New("NVENCForge.exe was not found — download it first")
	}

	jobs, err := buildJobs(request, status.EventChannel)
	if err != nil {
		return err
	}
	if !status.EventChannel {
		a.note("This NVENCForge.exe has no event channel (-json): the progress bars stay empty, only the log fills up.")
	}

	// Arbeitsverzeichnis ist der tools-Ordner: dort liegen die Programmdatei,
	// ihre INI und ihr FFmpeg. Die Ergebnisse entstehen weiterhin neben den
	// Quelldateien — darüber entscheidet der Konverter, nicht dieses Fenster.
	//
	// Ob auf der CPU gerechnet wird, steht ausschließlich in dieser Anfrage:
	// Der Prozessor-Modus ist ein Schalter der Befehlszeile, kein Wert aus der
	// INI. Eine INI kann ihn also nicht heimlich einschalten.
	return a.dispatcher.Submit(
		status.Path,
		filepath.Dir(status.Path),
		areaOf(request.Area),
		request.Parallel,
		request.Encoder == "cpu",
		jobs,
	)
}

// areaOf nimmt jeden bekannten Bereich an. Alles andere ist das Umwandeln von
// Hand — der Bereich, aus dem eine Anfrage ohne Angabe stammt.
//
// Ein vertippter Bereich läuft damit als Umwandeln los statt abzustürzen; dass
// er überhaupt abgewiesen gehört, prüft der Verteiler in Submit.
func areaOf(area string) string {
	if knownArea(area) {
		return area
	}
	return areaConvert
}

// StopArea hält EINEN Bereich sauber an: seine wartenden Aufträge fallen weg,
// seine laufenden Konverter hören sauber auf.
//
// Die anderen Bereiche bleiben unberührt. Wer seinen Stapel abbricht, will
// nicht nebenbei den beobachteten Ordner stilllegen oder das Zusammenfügen —
// von denen auf seiner Seite gar nichts zu sehen ist.
func (a *App) StopArea(area string) error {
	if !knownArea(area) {
		return fmt.Errorf("app.go: StopArea: unknown area %q", area)
	}
	return a.dispatcher.RequestStopArea(area)
}

// StopSlot hält nur einen einzelnen Konverter an. Die anderen laufen weiter,
// und der nächste wartende Auftrag rückt auf den frei werdenden Platz nach.
func (a *App) StopSlot(slot int) error {
	return a.dispatcher.StopSlot(slot)
}

// AnswerQuestion beantwortet eine Rückfrage des Konverters.
//
// Die Oberfläche schickt genau die Zeile, die auch jemand an der Konsole
// tippen würde: "1,3" für einzelne Spuren, leer für alle. Dazu die Platznummer
// aus dem Frage-Ereignis — laufen zwei Konverter, muss die Antwort zu genau
// der Datei gehören, für die gefragt wurde. Das Fenster darf erst antworten,
// wenn ein question-Ereignis angekommen ist, nie im Voraus (die Begründung
// steht bei Runner.Answer).
func (a *App) AnswerQuestion(slot int, answer string) error {
	return a.dispatcher.Answer(slot, answer)
}
