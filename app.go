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

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App hält den Zustand des Fensters.
type App struct {
	ctx    context.Context
	runner *Runner
}

// NewApp erzeugt die Anwendung.
func NewApp() *App {
	app := &App{}
	app.runner = NewRunner(app.emit)
	return app
}

// startup merkt sich den Zusammenhang, den Wails für Ereignisse und Dialoge
// braucht, und schaltet das Hineinziehen von Dateien frei.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	wailsruntime.OnFileDrop(ctx, func(x, y int, paths []string) {
		wailsruntime.EventsEmit(ctx, "queue:dropped", expandPaths(paths))
	})
}

// beforeClose verhindert das Schließen, solange ein Lauf aktiv ist.
//
// Grund: Beim Schließen bräche die Verbindung zum Konverter ab, er liefe aber
// unsichtbar weiter. Lieber ein klarer Hinweis als ein Programm, das im
// Hintergrund weiterrechnet, ohne dass es jemand sieht.
func (a *App) beforeClose(ctx context.Context) bool {
	if !a.runner.Running() {
		return false
	}
	a.emit("conv:log", "[gui] A conversion is still running — stop it first, then close the window.")
	return true
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
	GPU       GPUInfo         `json:"gpu"`
	Converter ConverterStatus `json:"converter"`
}

// GetStartupInfo liefert Grafikkarte und Zustand der Programmdatei.
func (a *App) GetStartupInfo() StartupInfo {
	return StartupInfo{GPU: queryGPU(), Converter: converterStatus()}
}

// GetConverterStatus prüft die Programmdatei erneut.
func (a *App) GetConverterStatus() ConverterStatus {
	return converterStatus()
}

// DownloadConverter holt die neueste NVENCForge.exe von GitHub.
func (a *App) DownloadConverter() (ConverterStatus, error) {
	return downloadConverter(a.ctx, func(done, total int64) {
		a.emit("conv:download", map[string]any{"done": done, "total": total})
	})
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
	return expandPaths(paths), nil
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
	return expandPaths([]string{folder}), nil
}

// IsRunning meldet, ob gerade konvertiert wird.
func (a *App) IsRunning() bool {
	return a.runner.Running()
}

// StartRun startet den Konverter mit den gewählten Einstellungen.
func (a *App) StartRun(request RunRequest) error {
	status := converterStatus()
	if !status.Found {
		return errors.New("NVENCForge.exe was not found — download it first")
	}

	args, err := buildConverterArgs(request, status.EventChannel)
	if err != nil {
		return err
	}
	if !status.EventChannel {
		a.emit("conv:log",
			"[gui] This NVENCForge.exe has no event channel (-json): the progress bars stay empty, only the log fills up.")
	}

	// Arbeitsverzeichnis ist der tools-Ordner: dort liegen die Programmdatei,
	// ihre INI und ihr FFmpeg. Die Ergebnisse entstehen weiterhin neben den
	// Quelldateien — darüber entscheidet der Konverter, nicht dieses Fenster.
	return a.runner.Start(status.Path, filepath.Dir(status.Path), args)
}

// StopRun löst den sauberen Abbruch aus.
func (a *App) StopRun() error {
	return a.runner.RequestStop()
}
