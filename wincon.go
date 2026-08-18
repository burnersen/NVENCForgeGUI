// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// wincon.go — Windows console handling and the clean stop.
//
// Warum diese Datei existiert: NVENCForge ist ein Konsolenprogramm und beendet
// sich bei Strg+C SAUBER — das angefangene Stück wird als abspielbare Vorschau
// abgeschlossen, das Original bleibt unangetastet. Den Prozess einfach
// abzuschießen würde genau diese Sicherheit umgehen und ein zerrissenes
// Fragment hinterlassen.
//
// Damit dieses Fenster denselben Weg auslösen kann, sind zwei Dinge nötig:
//
//  1. Eine Konsole. Ein Fensterprogramm hat keine, GenerateConsoleCtrlEvent
//     verlangt aber eine. Also legt das Programm beim Start eine eigene an und
//     versteckt sie sofort wieder.
//  2. Eine eigene Prozessgruppe je Lauf (CREATE_NEW_PROCESS_GROUP), damit das
//     Signal genau diesen einen Lauf trifft — und nicht dieses Fenster selbst.
//
// WICHTIG: Der Konverter darf NICHT mit CREATE_NO_WINDOW gestartet werden. Das
// gäbe ihm eine eigene Konsole, und ein Ctrl-Ereignis von hier käme dort nie an.
// Die versteckte Konsole aus Punkt 1 sorgt dafür, dass trotzdem kein schwarzes
// Fenster aufblitzt.
//
// Geprüft 2026-08-17 im Go-Quelltext (runtime/os_windows.go, ctrlHandler): Go
// bildet CTRL_BREAK_EVENT auf SIGINT ab. Der Konverter hört mit
// signal.Notify(os.Interrupt, SIGINT, SIGTERM) genau darauf — er braucht für
// diese Oberfläche also keine einzige Änderung.
package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	user32               = windows.NewLazySystemDLL("user32.dll")
	procAllocConsole     = kernel32.NewProc("AllocConsole")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procShowWindow       = user32.NewProc("ShowWindow")
)

// swHide ist der ShowWindow-Befehl "Fenster verbergen".
const swHide = 0

// setupHiddenConsole gibt diesem Fensterprogramm eine unsichtbare Konsole.
//
// Schlägt AllocConsole fehl, gibt es bereits eine — das ist beim Entwickeln mit
// "wails dev" der Fall. Dann darf hier nichts versteckt werden, sonst
// verschwindet das Terminal des Entwicklers.
func setupHiddenConsole() {
	created, _, _ := procAllocConsole.Call()
	if created == 0 {
		return
	}
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		_, _, _ = procShowWindow.Call(hwnd, swHide)
	}
}

// requestCleanStop löst im angegebenen Prozess denselben Ablauf aus wie Strg+C
// im Terminal. Beim zweiten Mal beendet sich der Konverter selbst sofort — das
// ist sein eigenes Verhalten und bleibt hier bewusst unangetastet.
func requestCleanStop(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("wincon.go: requestCleanStop: invalid process id %d", pid)
	}
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pid)); err != nil {
		return fmt.Errorf("wincon.go: requestCleanStop (GenerateConsoleCtrlEvent): %w", err)
	}
	return nil
}
