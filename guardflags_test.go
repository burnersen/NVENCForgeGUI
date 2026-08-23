//go:build windows && amd64

// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"strings"
	"testing"
)

// hasArg sagt, ob ein Schalter in der Befehlszeile steht.
func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// TestCropOffSendsNoCrop: ein leeres Kästchen muss beim Konverter ankommen.
// Ohne "-nocrop" gewönne autoCrop=true aus der INI, und es würde geschnitten,
// obwohl im Fenster nichts danach aussieht.
func TestCropOffSendsNoCrop(t *testing.T) {
	args, err := buildConverterArgs(RunRequest{
		Files: []string{"film.mkv"},
		Crop:  cropOff,
	}, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if !hasArg(args, "-nocrop") {
		t.Errorf("-nocrop fehlt in %v", args)
	}
	if hasArg(args, "-crop") {
		t.Errorf("-crop darf hier nicht dabei sein: %v", args)
	}
}

// TestCropUnsetSendsNothing: solange nicht feststeht, dass die Programmdatei
// den Gegenschalter kennt, darf er nicht mitgeschickt werden — eine ältere
// Ausgabe würde ihn bei jedem Lauf als unbekannte Option anmeckern.
func TestCropUnsetSendsNothing(t *testing.T) {
	args, err := buildConverterArgs(RunRequest{Files: []string{"film.mkv"}}, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	for _, flag := range []string{"-crop", "-nocrop", "-cropcheck"} {
		if hasArg(args, flag) {
			t.Errorf("%s wurde ungefragt mitgeschickt: %v", flag, args)
		}
	}
}

// TestCropOnAndCheckStayExclusive: beide Schalter zusammen wären
// widersprüchlich — "-cropcheck" konvertiert nichts, "-crop" schon.
func TestCropOnAndCheckStayExclusive(t *testing.T) {
	for _, mode := range []string{cropOn, cropCheck} {
		args, err := buildConverterArgs(RunRequest{
			Files: []string{"film.mkv"},
			Crop:  mode,
		}, false)
		if err != nil {
			t.Fatalf("unerwarteter Fehler: %v", err)
		}
		count := 0
		for _, flag := range []string{"-crop", "-nocrop", "-cropcheck"} {
			if hasArg(args, flag) {
				count++
			}
		}
		if count != 1 {
			t.Errorf("bei Crop=%q stehen %d Crop-Schalter in %v", mode, count, args)
		}
	}
}

// TestCodecH265SendsCounterSwitch: wählt das Fenster ausdrücklich H.265, muss
// "-h265" mitgehen. Der Schalter überstimmt ein früheres "-av1" auf derselben
// Befehlszeile; gegen die Konfigurationsdatei richtet er nichts aus, denn die
// hat gar keinen Schlüssel, der AV1 einschaltet.
func TestCodecH265SendsCounterSwitch(t *testing.T) {
	args, err := buildConverterArgs(RunRequest{
		Files: []string{"film.mkv"},
		Codec: codecH265,
	}, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if !hasArg(args, "-h265") {
		t.Errorf("-h265 fehlt in %v", args)
	}
	if hasArg(args, "-av1") {
		t.Errorf("-av1 darf hier nicht dabei sein: %v", args)
	}
}

// TestSuppressShutdownSendsNoShutdown ist der Schutz gegen den Schaden, der bis
// v1.0.1 im Fenster steckte, hier über die Konfigurationsdatei ausgelöst: mit
// autoShutdown=true würde der Konverter abschalten, sobald SEINE eine Datei
// fertig ist, und den Rest des Stapels mitreißen.
func TestSuppressShutdownSendsNoShutdown(t *testing.T) {
	args, err := buildConverterArgs(RunRequest{
		Files:                     []string{"film.mkv"},
		SuppressConverterShutdown: true,
	}, false)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if !hasArg(args, "-noshutdown") {
		t.Errorf("-noshutdown fehlt in %v", args)
	}
	// Und "-shutdown" bleibt weiterhin tabu: abgeschaltet wird von shutdown.go,
	// erst wenn wirklich alles leer ist.
	if hasArg(args, "-shutdown") {
		t.Errorf("-shutdown darf NIE an den Konverter gehen: %v", args)
	}
}

// TestToolModesStayClean: Zerlegen, Zusammenfügen und der DaVinci-Weg vertragen
// keine zusätzlichen Schalter — der Konverter hielte sie für Dateinamen. Sie
// brauchen den Abschalt-Schutz auch nicht: diese Modi kehren zurück, bevor im
// Konverter überhaupt abgeschaltet werden könnte.
func TestToolModesStayClean(t *testing.T) {
	for mode := range modeFlags {
		// Zusammenfügen braucht zu einem Video mindestens eine Beigabe, sonst
		// gäbe es nichts zusammenzufügen und der Argumentbau lehnt zu Recht ab.
		// needsJoinOrder statt einer eigenen Liste: die bliebe sonst zurück,
		// sobald ein weiterer Zusammenfüge-Weg dazukommt.
		queue := []string{"film.mkv"}
		if needsJoinOrder(mode) {
			queue = append(queue, "ton.mka")
		}
		args, err := buildConverterArgs(RunRequest{
			Files:                     queue,
			Mode:                      mode,
			Crop:                      cropOff,
			Codec:                     codecH265,
			SuppressConverterShutdown: true,
		}, false)
		if err != nil {
			t.Fatalf("Modus %q: unerwarteter Fehler: %v", mode, err)
		}
		for _, flag := range []string{"-nocrop", "-h265", "-noshutdown"} {
			if hasArg(args, flag) {
				t.Errorf("Modus %q bekam %s mit: %v", mode, flag, strings.Join(args, " "))
			}
		}
	}
}
