// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// watchdog_live_test.go — der Nachweis am echten Windows.
//
// Zwei Dinge lassen sich nur messen, nicht behaupten:
//
//  1. Kommt bei einem harten Ende (os.Exit(-1) — genau das tut Wails beim
//     Aussetzer der Webansicht) beim Wächter ein Rückgabewert ungleich 0 an?
//     Daran hängt die ganze Erkennung.
//  2. Findet der Wächter die echte Meldung und klickt sie weg — und lässt er
//     eine FREMDE Dialogbox in Ruhe? Das ist die Sicherheitsfrage.
//
// Beides braucht einen laufenden Windows-Desktop, deshalb läuft es nur mit
// NVENCFORGEGUI_LIVE=1 (wie die Prüfungen am echten Video). Für Nummer 1 wird
// ein winziges Wegwerf-Programm gebaut; Go muss dafür im Suchpfad liegen.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// procMessageBox zeigt im Test eine echte Windows-Meldung — dieselbe Art
// Fenster, die Wails über winc.Errorf aufmacht.
var procMessageBox = user32.NewProc("MessageBoxW")

const (
	mbOK        = 0x00000000
	mbIconError = 0x00000010

	// liveDialogWait ist die Geduld beim Suchen und beim Schließen.
	liveDialogWait = 5 * time.Second

	// strangeMessage ist der Text der fremden Meldung, die stehen bleiben muss.
	strangeMessage = "Die Datei konnte nicht gelesen werden."
)

// hardExitSource ist das Wegwerf-Programm für Messung 1.
const hardExitSource = `package main

import "os"

func main() { os.Exit(-1) }
`

// TestLiveHardExitArrivesAsFailure misst, was startWindow bei einem os.Exit(-1)
// zurückgibt.
func TestLiveHardExitArrivesAsFailure(t *testing.T) {
	requireLiveRun(t)

	folder := t.TempDir()
	source := filepath.Join(folder, "hardexit.go")
	if err := os.WriteFile(source, []byte(hardExitSource), 0o644); err != nil {
		t.Fatalf("Quelltext schreiben: %v", err)
	}
	program := filepath.Join(folder, "hardexit.exe")
	if output, err := exec.Command("go", "build", "-o", program, source).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v — %s", err, output)
	}

	// Ohne Meldungs-Wächter: Das Wegwerf-Programm hat gar kein Fenster.
	exitCode, err := startWindow(program, nil, false)
	if err != nil {
		t.Fatalf("startWindow: %v", err)
	}
	t.Logf("gemessener Rückgabewert nach os.Exit(-1): %d", exitCode)

	if exitCode == 0 {
		t.Fatal("ein hartes Ende kam als 0 an — der Wächter würde einen Absturz für ein sauberes Schließen halten")
	}
	if !restartWanted(exitCode, 1) {
		t.Errorf("restartWanted(%d, 1) ist false — nach einem Absturz käme kein Neustart", exitCode)
	}
}

// TestLiveRealMessageGetsDismissed zeigt eine echte Meldung mit dem echten Text
// und lässt den Wächter sie schließen.
func TestLiveRealMessageGetsDismissed(t *testing.T) {
	requireLiveRun(t)

	closed := showMessageBox(t, messageBoxErrorTitle, webViewCrashMessage)

	dialog := waitForCrashDialog(windows.GetCurrentProcessId())
	if dialog == 0 {
		t.Fatal("der Wächter hat die echte Meldung nicht gefunden")
	}
	dismissDialog(dialog)

	select {
	case <-closed:
	case <-time.After(liveDialogWait):
		// Nicht weggeklickt: Die Meldung steht noch und würde den Testlauf
		// blockieren. Selbst schließen, damit hier nichts hängen bleibt.
		dismissDialog(dialog)
		<-closed
		t.Fatal("die Meldung ging nicht zu — das automatische Wegklicken wirkt nicht")
	}
}

// TestLiveStrangeMessageStaysOpen ist die eigentliche Sicherheitsprüfung: Eine
// andere Dialogbox desselben Programms darf der Wächter NICHT anfassen.
func TestLiveStrangeMessageStaysOpen(t *testing.T) {
	requireLiveRun(t)

	closed := showMessageBox(t, messageBoxErrorTitle, strangeMessage)
	defer func() {
		// Aufräumen: Diese Meldung soll der Wächter ja gerade NICHT schließen.
		if dialog := findDialogByText(strangeMessage); dialog != 0 {
			dismissDialog(dialog)
		}
		<-closed
	}()

	// Warten, bis die Meldung sicher steht — sonst prüft der Test ins Leere.
	if findDialogByText(strangeMessage) == 0 {
		time.Sleep(time.Second)
	}

	if dialog := findCrashDialog(windows.GetCurrentProcessId()); dialog != 0 {
		t.Fatal("der Wächter hält eine fremde Meldung für die Absturzmeldung — er würde fremde Rückfragen wegklicken")
	}
}

// requireLiveRun hält die Prüfungen an, solange NVENCFORGEGUI_LIVE nicht gesetzt
// ist: Sie öffnen echte Fenster auf dem Bildschirm.
func requireLiveRun(t *testing.T) {
	t.Helper()

	if os.Getenv("NVENCFORGEGUI_LIVE") != "1" {
		t.Skip("Live-Prüfung übersprungen (NVENCFORGEGUI_LIVE=1 setzt sie in Gang)")
	}
}

// showMessageBox öffnet eine echte MessageBox in einem eigenen Faden. Der
// zurückgegebene Kanal schließt sich, sobald die Meldung wieder zu ist.
func showMessageBox(t *testing.T, title, text string) chan struct{} {
	t.Helper()

	closed := make(chan struct{})
	go func() {
		// Eine MessageBox gehört dem Faden, der sie öffnet — er darf während
		// der Anzeige nicht gewechselt werden.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(closed)

		titleText, _ := windows.UTF16PtrFromString(title)
		bodyText, _ := windows.UTF16PtrFromString(text)
		_, _, _ = procMessageBox.Call(
			0,
			uintptr(unsafe.Pointer(bodyText)),
			uintptr(unsafe.Pointer(titleText)),
			mbOK|mbIconError,
		)
	}()
	return closed
}

// waitForCrashDialog sucht die Absturzmeldung, bis sie da ist.
func waitForCrashDialog(pid uint32) windows.HWND {
	deadline := time.Now().Add(liveDialogWait)
	for time.Now().Before(deadline) {
		if dialog := findCrashDialog(pid); dialog != 0 {
			return dialog
		}
		time.Sleep(100 * time.Millisecond)
	}
	return 0
}

// searchedText und foundDialog reichen die Werte in den Rückruf hinein und
// wieder heraus — dieselbe Bauweise wie im Wächter selbst.
var (
	searchedText string
	foundDialog  windows.HWND
)

// dialogByTextSearch wird EINMAL erzeugt: windows.NewCallback belegt dauerhaft
// einen Platz in einer begrenzten Tabelle.
var dialogByTextSearch = windows.NewCallback(func(window windows.HWND, _ uintptr) uintptr {
	const keepLooking, stop = 1, 0

	var processID uint32
	_, _, _ = procGetWindowThreadProcessID.Call(uintptr(window), uintptr(unsafe.Pointer(&processID)))
	if processID != windows.GetCurrentProcessId() {
		return keepLooking
	}
	if windowStringField(procGetClassName, window) != messageBoxClass {
		return keepLooking
	}
	if dialogText(window) != searchedText {
		return keepLooking
	}
	foundDialog = window
	return stop
})

// findDialogByText sucht eine eigene Dialogbox über ihren Text. Gebraucht wird
// das nur zum Aufräumen der Test-Meldungen.
func findDialogByText(text string) windows.HWND {
	searchedText = text
	foundDialog = 0
	_ = windows.EnumWindows(dialogByTextSearch, nil)
	return foundDialog
}
