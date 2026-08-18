// setup.go — die Erstausstattung des Konverters anstoßen.
//
// Frisch heruntergeladen ist NVENCForge.exe allein noch nicht arbeitsfähig:
// Seine INI, die SRTCleaner-Einstellungen und sein eigenes FFmpeg entstehen
// erst, wenn er einmal gelaufen ist. Solange fehlt der Einstellungsseite ihre
// Grundlage — sie zeigt eine Datei an, die es noch gar nicht gibt.
//
// Gemessen am 2026-08-18 in einem leeren Ordner:
//
//	-help                    → nur NVENCForge_Help.txt, KEINE INI
//	<nicht vorhandene Datei> → NVENCForge_Config.ini, SRTCleaner_config.txt
//	                           UND das passende FFmpeg wird geladen
//
// Deshalb dieser Weg: ein Aufruf mit einem Dateinamen, den es sicher nicht
// gibt. Der Konverter richtet sich vollständig ein und meldet dann nur, dass
// die Datei fehlt.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// setupProbeName ist der Dateiname, mit dem der Konverter geweckt wird. Er
// nennt seinen Zweck, damit er im Protokoll nicht wie ein Fehler des Nutzers
// aussieht — und ist so unwahrscheinlich, dass er nie wirklich existiert.
const setupProbeName = "nvencforge-gui-setup-probe.mkv"

// needsSetup sagt, ob die Erstausstattung noch fehlt.
func needsSetup(status ConverterStatus) bool {
	if !status.Found {
		return false
	}
	if _, err := os.Stat(filepath.Join(status.ToolsDir, configFileName)); err != nil {
		return true
	}
	return !status.FFmpegPresent
}

// runSetup lässt den Konverter einmal laufen und reicht seine Ausgabe zeilenweise
// weiter. Der Aufruf blockiert, bis er fertig ist — der FFmpeg-Download kann
// eine Weile dauern, deshalb ruft die Oberfläche ihn nebenläufig auf und sieht
// derweil im Protokoll zu.
func runSetup(status ConverterStatus, line func(text string)) error {
	if !status.Found {
		return fmt.Errorf("setup.go: runSetup: NVENCForge.exe was not found")
	}
	probe := filepath.Join(status.ToolsDir, setupProbeName)
	if _, err := os.Stat(probe); err == nil {
		return fmt.Errorf("setup.go: runSetup: %s exists — remove it and try again", setupProbeName)
	}

	command := exec.Command(status.Path, probe)
	command.Dir = status.ToolsDir
	// Kein eigenes Fenster: Die Einrichtung läuft im Hintergrund, ihre Ausgabe
	// steht im Protokoll (die Begründung zu den Flags steht in wincon.go).
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}

	output, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("setup.go: runSetup (StdoutPipe): %w", err)
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		return fmt.Errorf("setup.go: runSetup (Start): %w", err)
	}

	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventLine)
	scanner.Split(splitLinesAndReturns)
	for scanner.Scan() {
		if text, ok := toLogLine(scanner.Text()); ok {
			line(text.Text)
		}
	}

	// Der Konverter endet mit einem Fehler, weil die Datei fehlt — das ist der
	// erwartete Ausgang und kein Grund zur Klage. Ob die Einrichtung geklappt
	// hat, entscheidet allein, was jetzt im Ordner liegt.
	_ = command.Wait()

	if _, err := os.Stat(filepath.Join(status.ToolsDir, configFileName)); err != nil {
		return fmt.Errorf("setup.go: runSetup: %s was still not created", configFileName)
	}
	return nil
}
