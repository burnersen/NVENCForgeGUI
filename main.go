// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// NVENCForgeGUI — ein Fenster für NVENCForge.
//
// Das Fenster ruft die unveränderte NVENCForge.exe als eigenes Programm auf und
// liest deren Datenkanal (-json) mit. Der Konverter weiß nichts von dieser
// Oberfläche, und die Oberfläche kennt ihn nur über seine Ereignisse.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// guiVersion ist die einzige Stelle, an der die Version dieses Fensters
// steht. Angezeigt wird sie im Fensterkopf (Titelleiste) und in der
// Kopfzeile der Oberfläche selbst (siehe StartupInfo in app.go).
const guiVersion = "1.0.2"

// singleInstanceID hält die Startsperre. Der Name muss auf dem Rechner
// einmalig sein und darf sich nie ändern — sonst erkennt eine neue Ausgabe die
// bereits laufende alte nicht mehr.
const singleInstanceID = "NVENCForgeGUI-6f2c1a70-window"

// startBackground nennt die Farbe, mit der das Fenster aufgeht, bevor die
// Oberfläche gezeichnet ist.
//
// Ohne das blitzt bei heller Stimmung für einen Augenblick der dunkle Grund
// auf — ein Zucken, das wie ein Fehler aussieht. Die Werte sind dieselben wie
// --bg in index.html; stehen sie dort anders, blitzt es wieder.
func startBackground(theme string) *options.RGBA {
	if normaliseTheme(theme) == themeLight {
		return &options.RGBA{R: 242, G: 242, B: 245, A: 1}
	}
	return &options.RGBA{R: 18, G: 18, B: 20, A: 1}
}

func main() {
	// Muss vor dem Fenster passieren: ohne eigene Konsole gibt es später keinen
	// sauberen Abbruch (siehe wincon.go).
	setupHiddenConsole()

	window, remembered := loadWindowState()
	app := NewApp(window, remembered)

	windowStart := options.Normal
	if window.Maximised {
		windowStart = options.Maximised
	}

	err := wails.Run(&options.App{
		// Genau der Titel, den HideProgress später wiederherstellt. Er steht
		// deshalb nur an einer Stelle (taskbar.go).
		Title:            baseWindowTitle(),
		Width:            window.Width,
		Height:           window.Height,
		MinWidth:         minWindowWidth,
		MinHeight:        minWindowHeight,
		WindowStartState: windowStart,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: startBackground(window.Theme),
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
			// DisableWebViewDrop darf hier NICHT gesetzt werden: Es schaltet
			// über AllowExternalDrag(false) den kompletten Weg ab, über den
			// die Dateipfade überhaupt erst hereinkommen. Verhindert wird das
			// Öffnen in der Webansicht stattdessen dadurch, dass die
			// Fensterseite runtime.OnFileDrop anmeldet (siehe index.html).
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		OnStartup:     app.startup,
		OnBeforeClose: app.beforeClose,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               singleInstanceID,
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		Bind: []any{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
