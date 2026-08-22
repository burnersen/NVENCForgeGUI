// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// watchdog_test.go — Prüfungen für den Start-Wächter.
//
// Geprüft wird hier alles, was ohne echte Fenster auskommt: die Unterscheidung
// Wächter/Fenster, die weitergereichten Startargumente und die Frage, wann ein
// neuer Versuch folgt. Das Erkennen und Wegklicken der echten Meldung steht in
// watchdog_live_test.go, weil dafür wirklich ein Fenster aufgehen muss.
package main

import (
	"slices"
	"strings"
	"testing"
)

// TestRunsAsWindowSpotsTheChild prüft die Weiche des ganzen Aufbaus: Wird sie
// falsch beantwortet, startet sich das Programm endlos selbst.
func TestRunsAsWindowSpotsTheChild(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"ganz normaler Start", []string{"NVENCForgeGUI.exe"}, false},
		{"mit Datei darauf gezogen", []string{"NVENCForgeGUI.exe", `C:\Filme\test.mkv`}, false},
		{"nach dem Selbst-Update", []string{"NVENCForgeGUI.exe", afterUpdateFlag, "1234"}, false},
		{"das Fenster selbst", []string{"NVENCForgeGUI.exe", windowProcessFlag}, true},
		{"Fenster mit Datei", []string{"NVENCForgeGUI.exe", windowProcessFlag, `C:\Filme\test.mkv`}, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := runsAsWindow(test.args); got != test.want {
				t.Errorf("runsAsWindow(%v) = %v, erwartet %v", test.args, got, test.want)
			}
		})
	}
}

// TestWindowArgsForwardsEverything sichert, dass gezogene Dateien beim Umweg
// über den Wächter nicht verloren gehen — sonst würde ein Doppelklick auf ein
// Video plötzlich nur noch ein leeres Fenster öffnen.
func TestWindowArgsForwardsEverything(t *testing.T) {
	args := []string{`C:\Filme\test.mkv`, `C:\Filme\zweites.mp4`}

	got := windowArgs(args, true)

	if len(got) == 0 || got[0] != windowProcessFlag {
		t.Fatalf("das Kennzeichen des Fensters muss vorn stehen, bekommen: %v", got)
	}
	for _, file := range args {
		if !slices.Contains(got, file) {
			t.Errorf("die Datei %q wurde nicht weitergereicht: %v", file, got)
		}
	}
}

// TestWindowArgsKeepsUpdateFlagOnFirstStart: Nur der erste Start kommt wirklich
// aus einem Selbst-Update — dort MUSS das Warten auf den Vorgänger erhalten
// bleiben, sonst rennt das neue Fenster in die noch belegte Startsperre.
func TestWindowArgsKeepsUpdateFlagOnFirstStart(t *testing.T) {
	got := windowArgs([]string{afterUpdateFlag, "4321"}, true)

	if !slices.Contains(got, afterUpdateFlag) {
		t.Errorf("%s fehlt beim ersten Start: %v", afterUpdateFlag, got)
	}
	if !slices.Contains(got, "4321") {
		t.Errorf("die Prozessnummer fehlt beim ersten Start: %v", got)
	}
}

// TestWindowArgsDropsUpdateFlagOnRestart: Beim Wiederholungsstart ist der
// Vorgänger längst beendet. Bliebe das Argument stehen, wartete das Fenster nur
// noch sinnlos.
func TestWindowArgsDropsUpdateFlagOnRestart(t *testing.T) {
	got := windowArgs([]string{afterUpdateFlag, "4321", `C:\Filme\test.mkv`}, false)

	if slices.Contains(got, afterUpdateFlag) {
		t.Errorf("%s hätte beim Neustart wegfallen müssen: %v", afterUpdateFlag, got)
	}
	if slices.Contains(got, "4321") {
		t.Errorf("die Prozessnummer hängt noch dran: %v", got)
	}
	// Die Datei darf dabei NICHT mit verschwinden.
	if !slices.Contains(got, `C:\Filme\test.mkv`) {
		t.Errorf("die Datei ist beim Aufräumen mit weggefallen: %v", got)
	}
}

// TestWindowArgsSurvivesFlagWithoutNumber prüft den Randfall, dass hinter dem
// Kennzeichen nichts mehr steht.
func TestWindowArgsSurvivesFlagWithoutNumber(t *testing.T) {
	got := windowArgs([]string{afterUpdateFlag}, false)

	if len(got) != 1 || got[0] != windowProcessFlag {
		t.Errorf("erwartet nur das Kennzeichen des Fensters, bekommen: %v", got)
	}
}

// TestRestartWanted prüft die Abbruchbedingung. Wichtigster Fall: ein sauberes
// Ende (0) darf NIE einen Neustart auslösen — sonst ginge das Fenster nach dem
// Schließen wieder auf.
func TestRestartWanted(t *testing.T) {
	cases := []struct {
		name     string
		exitCode int
		start    int
		want     bool
	}{
		{"sauber geschlossen", 0, 1, false},
		{"zweites Fenster gibt ab (Startsperre)", 0, 2, false},
		{"Absturz beim ersten Versuch", -1, 1, true},
		{"Absturz beim zweiten Versuch", -1, 2, true},
		{"Absturz beim letzten Versuch", -1, maxWindowStarts, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := restartWanted(test.exitCode, test.start); got != test.want {
				t.Errorf("restartWanted(%d, %d) = %v, erwartet %v",
					test.exitCode, test.start, got, test.want)
			}
		})
	}
}

// TestCrashMarkerLivesInTheMessage ist die Regressionsbremse für das
// automatische Wegklicken: Wer den Meldungstext umformuliert und dabei den
// Marker verliert, schaltet den Wächter still ab — das Fenster käme dann nach
// einem Aussetzer nicht mehr von allein zurück, ohne dass irgendetwas meckert.
func TestCrashMarkerLivesInTheMessage(t *testing.T) {
	if !strings.Contains(webViewCrashMessage, webViewCrashMarker) {
		t.Fatalf("der Marker %q steckt nicht mehr in der Meldung %q",
			webViewCrashMarker, webViewCrashMessage)
	}
	if strings.TrimSpace(webViewCrashMarker) == "" {
		t.Fatal("ein leerer Marker würde auf JEDE Meldung passen")
	}
	if windowMessages().WebView2ProcessCrash != webViewCrashMessage {
		t.Error("das Fenster zeigt einen anderen Text, als der Wächter sucht")
	}
}
