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

func main() {
	// Muss vor dem Fenster passieren: ohne eigene Konsole gibt es später keinen
	// sauberen Abbruch (siehe wincon.go).
	setupHiddenConsole()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "NVENCForge",
		Width:     1180,
		Height:    860,
		MinWidth:  920,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 20, A: 1},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
			// Ohne das Abschalten öffnet die eingebaute Webansicht eine
			// hineingezogene Datei einfach selbst — die Warteschlange bekäme
			// davon nichts mit.
			DisableWebViewDrop: true,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		OnStartup:     app.startup,
		OnBeforeClose: app.beforeClose,
		Bind: []any{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
