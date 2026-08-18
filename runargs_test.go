package main

import (
	"strings"
	"testing"
)

// joined macht aus der Argumentliste eine vergleichbare Zeichenkette.
func joined(args []string) string { return strings.Join(args, " ") }

func TestDefaultsPassNoOptions(t *testing.T) {
	// Wichtigster Fall: Steht alles auf "Standard", darf NICHTS außer dem
	// Datenkanal und den Dateien übergeben werden — sonst würde die Oberfläche
	// still die INI des Nutzers übersteuern.
	args, err := buildConverterArgs(RunRequest{Files: []string{"a.mkv"}}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := joined(args); got != "-json a.mkv" {
		t.Errorf("expected only the event channel and the file, got %q", got)
	}
}

func TestEventChannelIsOmittedForOlderConverters(t *testing.T) {
	args, err := buildConverterArgs(RunRequest{Files: []string{"a.mkv"}}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(joined(args), "-json") {
		t.Errorf("-json must not be passed when the converter cannot handle it: %q", joined(args))
	}
}

func TestEveryOptionReachesTheCommandLine(t *testing.T) {
	request := RunRequest{
		Files:      []string{"one.mkv", "two.mkv"},
		Codec:      codecAV1,
		Encoder:    encoderCPU,
		Container:  containerMP4,
		Resolution: resolutionOriginal,
		Audio:      audioCopy,
		BitDepth:   bitDepth8,
		Quality:    qualityFixed,
		FixedCQ:    40,
		MaxBitrate: 9000,
		KeepSource: true,
		Shutdown:   true,
	}
	args, err := buildConverterArgs(request, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := joined(args)
	for _, expected := range []string{
		"-json", "-av1", "-cpu", "-mp4", "-original", "-copyaudio", "-8bit",
		"-cq 40", "-9000", "-keep", "-shutdown", "one.mkv two.mkv",
	} {
		if !strings.Contains(got, expected) {
			t.Errorf("missing %q in %q", expected, got)
		}
	}
	// Die Dateien müssen ganz hinten stehen: Alles davor ist eine Option.
	if !strings.HasSuffix(got, "one.mkv two.mkv") {
		t.Errorf("files must come last, got %q", got)
	}
}

func TestQualityModes(t *testing.T) {
	cases := []struct {
		quality string
		want    string
	}{
		{qualityAuto, "-autocq"},
		{qualityOff, "-noautocq"},
		{"", ""},
	}
	for _, testCase := range cases {
		args, err := buildConverterArgs(
			RunRequest{Files: []string{"a.mkv"}, Quality: testCase.quality}, false)
		if err != nil {
			t.Fatalf("quality %q: unexpected error: %v", testCase.quality, err)
		}
		got := joined(args)
		if testCase.want == "" {
			if got != "a.mkv" {
				t.Errorf("quality %q should add nothing, got %q", testCase.quality, got)
			}
			continue
		}
		if !strings.Contains(got, testCase.want) {
			t.Errorf("quality %q should add %q, got %q", testCase.quality, testCase.want, got)
		}
	}
}

func TestFixedCQRangeDependsOnCodec(t *testing.T) {
	// AV1 reicht bis 63, H.265 nur bis 51. Wird die Grenze verwechselt, startet
	// der Lauf und scheitert erst im Konverter — genau das soll hier auffallen.
	if _, err := buildConverterArgs(
		RunRequest{Files: []string{"a.mkv"}, Quality: qualityFixed, FixedCQ: 60}, false); err == nil {
		t.Error("CQ 60 must be refused for H.265")
	}
	if _, err := buildConverterArgs(
		RunRequest{Files: []string{"a.mkv"}, Codec: codecAV1, Quality: qualityFixed, FixedCQ: 60}, false); err != nil {
		t.Errorf("CQ 60 is valid for AV1: %v", err)
	}
	if _, err := buildConverterArgs(
		RunRequest{Files: []string{"a.mkv"}, Quality: qualityFixed, FixedCQ: 0}, false); err == nil {
		t.Error("CQ 0 must be refused")
	}
}

func TestBitrateRange(t *testing.T) {
	if _, err := buildConverterArgs(
		RunRequest{Files: []string{"a.mkv"}, MaxBitrate: 5}, false); err == nil {
		t.Error("a bitrate of 5 kbit/s must be refused")
	}
	if _, err := buildConverterArgs(
		RunRequest{Files: []string{"a.mkv"}, MaxBitrate: 999999}, false); err == nil {
		t.Error("a bitrate of 999999 kbit/s must be refused")
	}
}

func TestEmptyQueueIsRefused(t *testing.T) {
	if _, err := buildConverterArgs(RunRequest{}, true); err == nil {
		t.Error("an empty queue must not start a run")
	}
}

// ----------------------------------------------------------------------------
// Die Werkzeug-Modi
// ----------------------------------------------------------------------------

// TestModeFlagComesFirst sichert die einzige Regel ab, an der ein Modus-Lauf
// scheitern kann: Der Konverter erkennt seinen Modus AUSSCHLIESSLICH am ersten
// Argument. Stünde -json davor oder eine Option dahinter, liefe still eine
// normale Konvertierung an.
func TestModeFlagComesFirst(t *testing.T) {
	args, err := buildConverterArgs(
		RunRequest{Mode: "davinci", Files: []string{`C:\a.mkv`}}, true)
	if err != nil {
		t.Fatalf("buildConverterArgs: %v", err)
	}
	if len(args) != 3 || args[0] != "-json" || args[1] != "-davinci" || args[2] != `C:\a.mkv` {
		t.Fatalf("erwartet [-json -davinci Datei], bekommen %v", args)
	}

	// Ohne Datenkanal muss der Modus ganz vorn stehen.
	args, err = buildConverterArgs(
		RunRequest{Mode: "davinci", Files: []string{`C:\a.mkv`}}, false)
	if err != nil {
		t.Fatalf("buildConverterArgs: %v", err)
	}
	if args[0] != "-davinci" {
		t.Fatalf("der Modus muss das erste Argument sein, bekommen %v", args)
	}
}

// TestModeRunCarriesNoConversionOptions: Die Werkzeug-Modi kopieren nur. Eine
// mitgeschickte Option würde vom Konverter für einen Dateinamen gehalten.
func TestModeRunCarriesNoConversionOptions(t *testing.T) {
	args, err := buildConverterArgs(RunRequest{
		Mode:       "davinci",
		Files:      []string{`C:\a.mkv`},
		Codec:      codecAV1,
		Encoder:    encoderCPU,
		Container:  containerMP4,
		Quality:    qualityFixed,
		FixedCQ:    28,
		MaxBitrate: 8000,
		KeepSource: true,
	}, false)
	if err != nil {
		t.Fatalf("buildConverterArgs: %v", err)
	}
	if len(args) != 2 {
		t.Fatalf("erwartet nur Modus + Datei, bekommen %v", args)
	}
}

func TestUnknownModeIsRefused(t *testing.T) {
	if _, err := buildConverterArgs(
		RunRequest{Mode: "nonsense", Files: []string{`C:\a.mkv`}}, false); err == nil {
		t.Error("ein unbekannter Modus muss abgelehnt werden, statt still zu konvertieren")
	}
}
