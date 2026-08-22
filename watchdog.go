// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// watchdog.go — der Start-Wächter gegen den Aussetzer der Windows-Webansicht.
//
// Das Problem: Steigt die Webansicht (WebView2) aus, zeigt Wails eine kleine
// Meldung und beendet das Programm mit os.Exit(-1)
// (internal/frontend/desktop/windows/frontend.go). Ein hartes os.Exit lässt
// sich nicht abfangen — das Programm kann sich also unmöglich selbst retten.
// Gemessen am 2026-08-21 über rund 215 Starts trifft das grob jeden fünften bis
// zwanzigsten Start, und zwar unabhängig von Programmausgabe, WebView2-Ausgabe,
// Datenordner, Grafikbeschleunigung, Startabstand und Startart. Repariert ist
// hier deshalb nichts: Der Fehler passiert weiterhin, er kostet nur keinen
// Handgriff mehr.
//
// Der Aufbau: Dieselbe Programmdatei startet sich selbst noch einmal als
// Kindprozess und gibt ihm das Argument --window-process mit. Nur das KIND
// öffnet ein Fenster; der Elternprozess hält keine Webansicht und ist vom
// Fehler damit gar nicht betroffen. Endet das Kind mit einem Rückgabewert
// ungleich 0, startet der Wächter es neu.
//
// Warum kein zweites Programm daneben: Das Selbst-Update (selfupdate.go)
// ersetzt genau die laufende Programmdatei durch das Release-Asset
// NVENCForgeGUI.exe. Ein umbenannter Kern mit vorgeschaltetem Starter würde
// beim ersten Update überschrieben — mit einer einzigen Datei kann das nicht
// passieren.
//
// Nicht hierher gehört die Startsperre (SingleInstanceLock in main.go): Die
// muss beim Fenster bleiben. Ein zweites Fenster reicht seine Argumente an das
// erste weiter und endet mit 0 (Wails, single_instance.go) — für den Wächter
// also ein ganz normales Ende ohne Neustart.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// windowProcessFlag unterscheidet Kind von Wächter. Der Name taucht nach
	// außen nie auf; wer die exe von Hand damit startet, bekommt einfach ein
	// Fenster ohne Netz.
	windowProcessFlag = "--window-process"

	// maxWindowStarts ist die Zahl der Startversuche EINSCHLIESSLICH des
	// ersten. Bei einer Fehlerrate um 20 % scheitert damit rechnerisch noch
	// etwa jeder hundertste Start; ein Deckel muss sein, damit ein echter,
	// dauerhafter Fehler nicht in einer Endlosschleife endet.
	maxWindowStarts = 3

	// restartPause bremst die Wiederholung. Sie heilt nichts — das Warten vor
	// dem Start wurde am 2026-08-21 gemessen und half nicht. Sie verhindert
	// nur, dass bei einem echten Dauerfehler drei Starts in Sekundenbruchteilen
	// durchrattern und das Fenster wild blinkt.
	restartPause = 1500 * time.Millisecond
)

// runsAsWindow sagt, ob dieser Prozess das Fenster ist. false heißt: Er ist der
// Wächter und startet das Fenster erst noch.
func runsAsWindow(args []string) bool {
	for _, argument := range args {
		if argument == windowProcessFlag {
			return true
		}
	}
	return false
}

// windowArgs baut die Startargumente des Fensters aus den eigenen.
//
// Durchgereicht wird alles, was der Nutzer mitgegeben hat (etwa Dateien, die
// auf die Programmdatei gezogen wurden). Nur beim WIEDERHOLUNGSSTART fällt
// --after-update samt Prozessnummer weg: Der Vorgänger, auf den die frisch
// eingespielte Ausgabe wartet, ist zu diesem Zeitpunkt längst beendet — das
// Warten wäre reine Verzögerung.
func windowArgs(args []string, firstStart bool) []string {
	forwarded := make([]string, 0, len(args)+1)
	forwarded = append(forwarded, windowProcessFlag)

	skipNext := false
	for _, argument := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if argument == afterUpdateFlag && !firstStart {
			// Die Prozessnummer steht direkt dahinter und muss mit weg.
			skipNext = true
			continue
		}
		forwarded = append(forwarded, argument)
	}
	return forwarded
}

// restartWanted sagt, ob nach diesem Ende ein weiterer Versuch folgt.
// Rückgabewert 0 heißt "sauber beendet" — dann ist Schluss, egal wie früh.
func restartWanted(exitCode, start int) bool {
	return exitCode != 0 && start < maxWindowStarts
}

// runWatchdog startet das Fenster und startet es nach einem Absturz neu.
//
// Der zweite Rückgabewert sagt, ob überhaupt bewacht werden konnte: false
// heißt "das hat nicht geklappt, bitte in DIESEM Prozess ein Fenster öffnen".
// Ein Programm ohne Netz ist immer noch besser als gar kein Programm.
func runWatchdog() (int, bool) {
	exePath, err := os.Executable()
	if err != nil {
		return 0, false
	}

	for start := 1; start <= maxWindowStarts; start++ {
		// Beim LETZTEN Versuch bleibt die Meldung stehen. Sonst würde ein
		// dauerhafter Fehler dazu führen, dass gar nichts mehr passiert: kein
		// Fenster, keine Erklärung, nur ein stiller Rechner.
		lastStart := start == maxWindowStarts
		exitCode, err := startWindow(exePath, windowArgs(os.Args[1:], start == 1), !lastStart)
		if err != nil {
			if start == 1 {
				return 0, false
			}
			return 1, true
		}
		if !restartWanted(exitCode, start) {
			return exitCode, true
		}
		time.Sleep(restartPause)
	}
	return 0, true
}

// startWindow startet das Fenster als Kindprozess und wartet, bis es endet.
func startWindow(exePath string, args []string, dismissDialogs bool) (int, error) {
	command := exec.Command(exePath, args...)
	command.Dir = filepath.Dir(exePath)
	// DETACHED_PROCESS ist Pflicht, nicht Geschmackssache: Wird dieses Programm
	// aus einer Konsole heraus gestartet, würde das Kind sie erben. Sein
	// eigenes AllocConsole (wincon.go) schlüge dann fehl — und ohne eigene
	// Konsole gibt es keinen sauberen Abbruch eines Laufs. Dieselbe Begründung
	// steht in selfupdate.go bei startSuccessor.
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	if err := command.Start(); err != nil {
		return 0, fmt.Errorf("watchdog.go: startWindow (Start): %w", err)
	}

	stopWatching := make(chan struct{})
	if dismissDialogs {
		go dismissCrashDialogs(uint32(command.Process.Pid), stopWatching)
	}

	err := command.Wait()
	close(stopWatching)

	// Ein Ende mit Rückgabewert ungleich 0 meldet Go als Fehler. Genau dieser
	// Fall ist hier aber der interessante und kein Fehler des Wartens.
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return 0, fmt.Errorf("watchdog.go: startWindow (Wait): %w", err)
	}
	return command.ProcessState.ExitCode(), nil
}

const (
	// messageBoxClass ist die Fensterklasse jeder Windows-Dialogbox.
	messageBoxClass = "#32770"

	// messageBoxErrorTitle ist der Titel, den Wails der Meldung gibt: Sie läuft
	// über winc.Errorf, und das ruft MessageBox mit dem festen Titel "Error"
	// auf (winc/commondlgs.go).
	messageBoxErrorTitle = "Error"

	// messageBoxTextID ist die feste Kennung des Textfeldes in einer MessageBox.
	messageBoxTextID = 0xFFFF

	// wmClose ist die Nachricht "mach zu". Gemessen am 2026-08-22 an einer
	// echten MessageBox (watchdog_live_test.go): Sie schließt die Meldung
	// sofort. Die naheliegenden Wege tun das NICHT — WM_COMMAND mit IDOK
	// verpufft, und der OK-Knopf ist über GetDlgItem gar nicht zu greifen.
	wmClose = 0x0010

	// maxWindowTextLength deckt den Meldungstext mit reichlich Luft ab.
	maxWindowTextLength = 512

	// crashDialogPoll ist der Takt, in dem nach der Meldung gesehen wird. Der
	// Blick über die Fensterliste ist billig, 0,4 s fallen niemandem auf.
	crashDialogPoll = 400 * time.Millisecond
)

var (
	procGetClassName   = user32.NewProc("GetClassNameW")
	procGetWindowText  = user32.NewProc("GetWindowTextW")
	procGetDlgItemText = user32.NewProc("GetDlgItemTextW")
	procPostMessage    = user32.NewProc("PostMessageW")
)

// dismissCrashDialogs klickt die Absturzmeldung des Fensters weg, damit der
// Neustart ohne Zutun durchläuft.
//
// Weggeklickt wird ausschließlich ein Fenster, auf das ALLE vier Merkmale
// passen: Es gehört dem eigenen Kindprozess, es ist eine Dialogbox, sein Titel
// ist "Error", und sein Text enthält die eigene Absturzmeldung. Diese
// Genauigkeit ist der Sinn der Übung — auch die normalen Rückfragen des
// Fensters sind Dialogboxen, und die darf hier nichts anfassen.
func dismissCrashDialogs(windowPID uint32, stop <-chan struct{}) {
	ticker := time.NewTicker(crashDialogPoll)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if dialog := findCrashDialog(windowPID); dialog != 0 {
				dismissDialog(dialog)
			}
		}
	}
}

// crashDialogPID und crashDialogFound reichen die Werte in den Rückruf hinein
// und wieder heraus. Das ist sicher, weil die Suche nur aus dem EINEN Faden von
// dismissCrashDialogs läuft und der Wächter immer nur ein Kind hat.
var (
	crashDialogPID   uint32
	crashDialogFound windows.HWND
)

// crashDialogSearch wird EINMAL erzeugt: windows.NewCallback belegt dauerhaft
// einen Platz in einer begrenzten Tabelle. In der Schleife erzeugt, wäre das
// ein Leck, das irgendwann das Programm anhält (dieselbe Falle wie in
// taskbar.go).
var crashDialogSearch = windows.NewCallback(func(window windows.HWND, _ uintptr) uintptr {
	const keepLooking, stop = 1, 0

	var processID uint32
	_, _, _ = procGetWindowThreadProcessID.Call(uintptr(window), uintptr(unsafe.Pointer(&processID)))
	if processID != crashDialogPID {
		return keepLooking
	}
	if !isCrashDialog(window) {
		return keepLooking
	}
	crashDialogFound = window
	return stop
})

// findCrashDialog sucht die Absturzmeldung des angegebenen Prozesses.
// 0 heißt: keine da — der Normalfall.
func findCrashDialog(windowPID uint32) windows.HWND {
	crashDialogPID = windowPID
	crashDialogFound = 0
	// Der Fehler von EnumWindows wird bewusst nicht gemeldet: Bricht die
	// Aufzählung ab, weil der Rückruf "gefunden" gesagt hat, meldet Windows
	// genau das als Fehler.
	_ = windows.EnumWindows(crashDialogSearch, nil)
	return crashDialogFound
}

// isCrashDialog prüft die drei Merkmale, die am Fenster selbst hängen.
func isCrashDialog(window windows.HWND) bool {
	if windowStringField(procGetClassName, window) != messageBoxClass {
		return false
	}
	if windowStringField(procGetWindowText, window) != messageBoxErrorTitle {
		return false
	}
	return strings.Contains(dialogText(window), webViewCrashMarker)
}

// dismissDialog schließt die Meldung.
//
// Es wird gepostet, nicht gesendet: Der Dialog gehört einem anderen Prozess,
// und ein sendendes Warten würde diesen Faden anhalten, falls dort etwas hängt.
// Eine MessageBox mit nur einem Knopf wertet das Schließen wie ein "OK" —
// dahinter läuft im Fenster ohnehin nur noch das Beenden.
func dismissDialog(window windows.HWND) {
	_, _, _ = procPostMessage.Call(uintptr(window), wmClose, 0, 0)
}

// windowStringField liest Klassenname oder Fenstertitel. Beide Windows-Aufrufe
// haben dieselbe Form, deshalb reicht eine Stelle für beide.
func windowStringField(procedure *windows.LazyProc, window windows.HWND) string {
	buffer := make([]uint16, maxWindowTextLength)
	length, _, _ := procedure.Call(
		uintptr(window),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if length == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:length])
}

// dialogText liest den Meldungstext aus dem Textfeld der MessageBox.
func dialogText(window windows.HWND) string {
	buffer := make([]uint16, maxWindowTextLength)
	length, _, _ := procGetDlgItemText.Call(
		uintptr(window),
		messageBoxTextID,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if length == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:length])
}
