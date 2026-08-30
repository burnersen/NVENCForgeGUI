//go:build windows && amd64

// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"strings"
	"testing"
)

// Sichert den Prüflauf der Qualitätssuche ab: das richtige Flag, keine
// doppelten Schalter, und keine zwei Prüfläufe in einem Durchgang.

func TestQualityCheckSendsCQCheck(t *testing.T) {
	args, err := buildQualityArgs(RunRequest{Quality: qualityCheck})
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	joined := strings.Join(args, " ")
	if joined != "-cqcheck" {
		t.Errorf("erwartet wurde genau \"-cqcheck\", bekam %q", joined)
	}
	// "-cqcheck" schaltet die Suche im Konverter selbst ein. Käme "-autocq"
	// dazu, stünde derselbe Wunsch zweimal in der Zeile.
	if strings.Contains(joined, "-autocq") {
		t.Error("-autocq ist neben -cqcheck überflüssig")
	}
}

func TestQualityModesStayApart(t *testing.T) {
	cases := []struct {
		quality string
		want    string
	}{
		{qualityAuto, "-autocq"},
		{qualityOff, "-noautocq"},
		{qualityCheck, "-cqcheck"},
	}
	for _, c := range cases {
		t.Run(c.quality, func(t *testing.T) {
			args, err := buildQualityArgs(RunRequest{Quality: c.quality})
			if err != nil {
				t.Fatalf("unerwarteter Fehler: %v", err)
			}
			if got := strings.Join(args, " "); got != c.want {
				t.Errorf("Quality %q ergab %q, erwartet %q", c.quality, got, c.want)
			}
		})
	}
}

// TestTwoCheckRunsRefused deckt den Fall ab, den der Konverter stillschweigend
// falsch entscheiden würde: der Balken-Prüflauf steigt dort früher aus, die
// Qualitätssuche käme nie dran. Das Fenster muss vorher nein sagen.
func TestTwoCheckRunsRefused(t *testing.T) {
	_, err := buildConverterArgs(RunRequest{
		Files:   []string{"C:\\filme\\a.mkv"},
		Quality: qualityCheck,
		Crop:    cropCheck,
	}, true)
	if err == nil {
		t.Fatal("zwei Prüfläufe zusammen müssen abgelehnt werden")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("die Meldung sollte den Grund nennen, war: %v", err)
	}
}

// Die Gegenprobe: jeder Prüflauf für sich muss durchgehen — auch zusammen mit
// dem NORMALEN Balkenschnitt, der ja nur eine Filtereinstellung ist.
func TestSingleCheckRunsAllowed(t *testing.T) {
	cases := []struct {
		name    string
		request RunRequest
		want    string
	}{
		{"nur Qualität prüfen",
			RunRequest{Files: []string{"a.mkv"}, Quality: qualityCheck}, "-cqcheck"},
		{"nur Balken prüfen",
			RunRequest{Files: []string{"a.mkv"}, Crop: cropCheck}, "-cropcheck"},
		{"Qualität prüfen, Balken wirklich schneiden",
			RunRequest{Files: []string{"a.mkv"}, Quality: qualityCheck, Crop: cropOn}, "-cqcheck"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args, err := buildConverterArgs(c.request, true)
			if err != nil {
				t.Fatalf("unerwartet abgelehnt: %v", err)
			}
			if !strings.Contains(strings.Join(args, " "), c.want) {
				t.Errorf("%q fehlt in %q", c.want, strings.Join(args, " "))
			}
		})
	}
}

// TestCQCheckCapabilityMarker hält die Fähigkeitsprüfung an das Flag gebunden.
// Ohne sie böte das Fenster den Knopf auch neben einer älteren Programmdatei
// an, die ihn als unbekannte Option verwirft.
func TestCQCheckCapabilityMarker(t *testing.T) {
	if cqCheckFlagMarker != "-cqcheck" {
		t.Errorf("der Marker muss der Schalter selbst sein, ist %q", cqCheckFlagMarker)
	}
	// Der Marker darf in keinem anderen Schalter als Teilwort stecken, sonst
	// meldet eine ältere exe fälschlich, sie könne es.
	for _, other := range []string{jsonFlagMarker, cropFlagMarker, guardFlagMarker, baseSettingsMarker} {
		if strings.Contains(other, cqCheckFlagMarker) {
			t.Errorf("%q enthält den Marker %q", other, cqCheckFlagMarker)
		}
	}
}
