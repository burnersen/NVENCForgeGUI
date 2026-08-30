//go:build windows && amd64

// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"strings"
	"testing"
)

// Windows-Pfade stehen hier als Roh-Zeichenketten in schrägen Anführungszeichen.
// Das erspart die doppelten Schrägstriche und macht sie lesbar.
const (
	testFileA          = `C:\filme\a.mkv`
	testFileB          = `C:\filme\b.mkv`
	testFileAOtherCase = `c:\FILME\A.mkv`
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
		Files:   []string{testFileA},
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
			RunRequest{Files: []string{testFileA}, Quality: qualityCheck}, "-cqcheck"},
		{"nur Balken prüfen",
			RunRequest{Files: []string{testFileA}, Crop: cropCheck}, "-cropcheck"},
		{"Qualität prüfen, Balken wirklich schneiden",
			RunRequest{Files: []string{testFileA}, Quality: qualityCheck, Crop: cropOn}, "-cqcheck"},
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

// ----------------------------------------------------------------------------
// Die Wiederverwendung: ein schon gemessener CQ soll die zweite Suche sparen —
// aber nur dort, wo er wirklich gilt.
// ----------------------------------------------------------------------------

func TestMeasuredCQBecomesFixedCQ(t *testing.T) {
	jobs, err := buildJobs(RunRequest{
		Files:      []string{testFileA},
		Quality:    qualityAuto,
		MeasuredCQ: map[string]int{testFileA: 28},
	}, true)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("ein Auftrag erwartet, bekam %d", len(jobs))
	}
	joined := strings.Join(jobs[0].args, " ")
	if !strings.Contains(joined, "-cq 28") {
		t.Errorf("der gemessene CQ fehlt: %q", joined)
	}
	// Beides zusammen wäre widersprüchlich — "-cq" schlägt die Suche, und die
	// Suche noch einmal anzuwerfen ist genau das, was gespart werden soll.
	if strings.Contains(joined, "-autocq") {
		t.Errorf("die Suche läuft trotzdem noch: %q", joined)
	}
}

func TestMeasuredCQOnlyForItsOwnFile(t *testing.T) {
	jobs, err := buildJobs(RunRequest{
		Files:      []string{testFileA, testFileB},
		Quality:    qualityAuto,
		MeasuredCQ: map[string]int{testFileA: 28},
	}, true)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("zwei Aufträge erwartet, bekam %d", len(jobs))
	}
	if got := strings.Join(jobs[0].args, " "); !strings.Contains(got, "-cq 28") {
		t.Errorf("die gemessene Datei bekam ihren Wert nicht: %q", got)
	}
	// Die zweite Datei wurde nie gemessen und muss ganz normal gesucht werden.
	if got := strings.Join(jobs[1].args, " "); !strings.Contains(got, "-autocq") {
		t.Errorf("die ungemessene Datei sucht nicht: %q", got)
	}
}

func TestMeasuredCQRespectsTheUser(t *testing.T) {
	cases := []struct {
		name    string
		quality string
		wantCQ  bool
	}{
		{"auto nimmt den Messwert", qualityAuto, true},
		{"leer heißt INI, und die stand auf Auto", "", true},
		{"ein fester CQ ist die Wahl des Nutzers", qualityFixed, false},
		{"abgeschaltet heißt abgeschaltet", qualityOff, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			request := RunRequest{
				Files:      []string{testFileA},
				Quality:    c.quality,
				FixedCQ:    22,
				MeasuredCQ: map[string]int{testFileA: 28},
			}
			_, ok := measuredCQFor(request, testFileA)
			if ok != c.wantCQ {
				t.Errorf("Quality %q: Messwert benutzt = %v, erwartet %v", c.quality, ok, c.wantCQ)
			}
		})
	}
}

// TestMeasuredCQOutsideTheScaleIsRefused: ein Wert, der die Skala verlässt,
// darf nicht durchgereicht werden — der Konverter lehnte den Lauf sonst erst
// ab, nachdem er schon gestartet ist.
func TestMeasuredCQOutsideTheScaleIsRefused(t *testing.T) {
	for _, cq := range []int{0, -5, maxCQH265 + 1} {
		request := RunRequest{
			Files:      []string{testFileA},
			Quality:    qualityAuto,
			MeasuredCQ: map[string]int{testFileA: cq},
		}
		if _, ok := measuredCQFor(request, testFileA); ok {
			t.Errorf("CQ %d liegt außerhalb der H.265-Skala und wurde trotzdem benutzt", cq)
		}
	}
	// Auf der AV1-Skala ist derselbe Wert dagegen gültig.
	request := RunRequest{
		Files:      []string{testFileA},
		Codec:      codecAV1,
		Quality:    qualityAuto,
		MeasuredCQ: map[string]int{testFileA: maxCQH265 + 1},
	}
	if _, ok := measuredCQFor(request, testFileA); !ok {
		t.Error("auf der AV1-Skala hätte der Wert gelten müssen")
	}
}

func TestMeasuredCQIgnoresPathCase(t *testing.T) {
	// Windows unterscheidet Groß- und Kleinschreibung nicht; die Oberfläche
	// bekommt Pfade aus Ablage, Dialog und Ordnerüberwachung und schreibt sie
	// nicht überall gleich.
	request := RunRequest{
		Files:      []string{testFileA},
		Quality:    qualityAuto,
		MeasuredCQ: map[string]int{testFileAOtherCase: 28},
	}
	cq, ok := measuredCQFor(request, testFileA)
	if !ok || cq != 28 {
		t.Errorf("Pfad mit anderer Schreibweise nicht erkannt: cq=%d ok=%v", cq, ok)
	}
}
