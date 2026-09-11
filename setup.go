// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// setup.go — den Konverter anstoßen, ohne ihn arbeiten zu lassen.
//
// Zwei Anlässe brauchen das, und beide gehen seit 1.12.0 denselben Weg
// (runConverterIdle):
//
//   - Die Erstausstattung: Frisch heruntergeladen ist NVENCForge.exe allein
//     noch nicht arbeitsfähig. Seine INI, die SRTCleaner-Einstellungen und
//     sein eigenes FFmpeg entstehen erst, wenn er einmal gelaufen ist.
//   - Jedes spätere Update: Neue Einstellungen trägt der Konverter beim START
//     in seine INI nach, nicht beim Einspielen. Ohne Anstoß erschiene ein
//     neuer Schlüssel erst nach der nächsten Konvertierung.
//
// In beiden Fällen zeigt die Einstellungsseite sonst eine Datei an, die es
// noch gar nicht gibt oder die noch nicht alles enthält.
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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// setupProbeName ist der Dateiname, mit dem der Konverter geweckt wird. Er
// nennt seinen Zweck, damit er im Protokoll nicht wie ein Fehler des Nutzers
// aussieht. Seit 1.12.0 liegt er in einem eigens angelegten leeren Ordner und
// kann deshalb nie mit einer echten Datei zusammenfallen — die frühere
// Prüfung "existiert der Name schon?" entfällt damit.
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

// runConverterIdle lässt den Konverter einmal leer laufen — der eine Weg, auf
// dem dieses Fenster ihn anstößt, ohne dass er dabei Arbeit verrichten kann.
//
// Zwei Vorkehrungen machen den Lauf harmlos:
//
//   - Das Arbeitsverzeichnis ist ein frisch angelegter, leerer Ordner. Dort
//     sucht der Konverter nach Videos, wenn ihm keine brauchbare Datei genannt
//     wird — und findet zwangsläufig keine. Selbst ein versehentlich im
//     tools-Ordner liegendes Video bleibt so unangetastet. Seine eigenen
//     Dateien legt er trotzdem richtig ab: INI, SRTCleaner-Einstellungen und
//     FFmpeg sucht er immer neben seiner Programmdatei.
//   - Übergeben wird setupProbeName, ein Name, den es in diesem leeren Ordner
//     sicher nicht gibt (siehe die Messung im Dateikopf).
//
// line darf nil sein; dann läuft der Anstoß still und es wird nichts gelesen.
// Sonst geht jede Ausgabezeile durch, damit ein langer FFmpeg-Download im
// Protokoll sichtbar bleibt.
//
// Dass der Konverter am Ende einen Fehler meldet, ist der erwartete Ausgang:
// Die genannte Datei gibt es ja nicht. Ob der Anstoß etwas gebracht hat,
// entscheidet allein, was danach im Ordner liegt — das prüft der Aufrufer.
func runConverterIdle(ctx context.Context, exePath string, line func(text string)) error {
	workDir, err := os.MkdirTemp("", "NVENCForgeGUI_idle_")
	if err != nil {
		return fmt.Errorf("setup.go: runConverterIdle (MkdirTemp): %w", err)
	}
	defer os.RemoveAll(workDir)

	command := exec.CommandContext(ctx, exePath, filepath.Join(workDir, setupProbeName))
	command.Dir = workDir
	// Kein eigenes Fenster: Der Lauf gehört in den Hintergrund, seine Ausgabe
	// ins Protokoll (die Begründung zu den Flags steht in wincon.go).
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}

	if line == nil {
		_ = command.Run()
		return nil
	}

	output, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("setup.go: runConverterIdle (StdoutPipe): %w", err)
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		return fmt.Errorf("setup.go: runConverterIdle (Start): %w", err)
	}

	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventLine)
	scanner.Split(splitLinesAndReturns)
	for scanner.Scan() {
		if text, ok := toLogLine(scanner.Text()); ok {
			line(text.Text)
		}
	}
	_ = command.Wait()
	return nil
}

// runSetup stößt die Erstausstattung an und reicht die Ausgabe zeilenweise
// weiter. Der Aufruf blockiert, bis der Konverter fertig ist — der
// FFmpeg-Download kann eine Weile dauern, deshalb ruft die Oberfläche ihn
// nebenläufig auf und sieht derweil im Protokoll zu.
//
// Bewusst ohne Zeitgrenze: Ein langsamer Download ist kein Fehler, und ein
// abgebrochener Lauf hinterließe eine halb eingerichtete Installation.
func runSetup(status ConverterStatus, line func(text string)) error {
	if !status.Found {
		return fmt.Errorf("setup.go: runSetup: NVENCForge.exe was not found")
	}
	if err := runConverterIdle(context.Background(), status.Path, line); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(status.ToolsDir, configFileName)); err != nil {
		return fmt.Errorf("setup.go: runSetup: %s was still not created", configFileName)
	}
	return nil
}
