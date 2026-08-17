package main

import "testing"

// Die Vorlage entspricht dem echten Format: CRLF-Zeilenenden, Kommentarblöcke
// vor jedem Wert, Leerzeilen dazwischen.
const sampleConfig = "# NVENCForge Configuration\r\n" +
	"# Allowed: 720, 1080, 1440, 2160   |   Default: 1080\r\n" +
	"maxResolution=1080\r\n" +
	"\r\n" +
	"maxBitrate1080p=8000\r\n" +
	"maxBitrateOriginal=22000\r\n" +
	"av1MaxBitrate1080p=6000\r\n" +
	"av1MaxBitrateOriginal=13000\r\n" +
	"targetCQ=26\r\n" +
	"av1TargetCQ=32\r\n" +
	"autoCQ=true\r\n"

func TestParseConfigReadsTheValuesWeShow(t *testing.T) {
	entries := parseConfigEntries(sampleConfig)

	cases := []struct {
		key  string
		want int
	}{
		{"maxResolution", 1080},
		{"maxBitrate1080p", 8000},
		{"maxBitrateOriginal", 22000},
		{"av1MaxBitrate1080p", 6000},
		{"av1MaxBitrateOriginal", 13000},
		{"targetCQ", 26},
		{"av1TargetCQ", 32},
	}
	for _, tc := range cases {
		if got := intEntry(entries, tc.key); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.key, got, tc.want)
		}
	}
	if value, known := boolEntry(entries, "autoCQ"); !known || !value {
		t.Errorf("autoCQ = %v (known %v), want true/true", value, known)
	}
}

// Ein auskommentierter Schlüssel darf NICHT gelten. Die INI erklärt jeden Wert
// in Kommentarzeilen darüber; würde eine davon mitgelesen, zeigte das Fenster
// eine Einstellung an, die gar nicht aktiv ist.
func TestCommentedKeysAreIgnored(t *testing.T) {
	entries := parseConfigEntries("# maxResolution=720\r\nmaxResolution=1440\r\n")
	if got := intEntry(entries, "maxResolution"); got != 1440 {
		t.Errorf("maxResolution = %d, want 1440 (the comment must not win)", got)
	}
}

// Fehlt ein Wert, wird nichts geraten: 0 bedeutet für die Oberfläche
// "unbekannt", und sie zeigt dann gar keine Zahl statt einer falschen.
func TestMissingValuesStayUnknown(t *testing.T) {
	entries := parseConfigEntries("maxResolution=1080\r\n")
	if got := intEntry(entries, "maxBitrate1080p"); got != 0 {
		t.Errorf("missing key returned %d, want 0", got)
	}
	if got := intEntry(entries, "targetCQ"); got != 0 {
		t.Errorf("missing key returned %d, want 0", got)
	}
	if _, known := boolEntry(entries, "autoCQ"); known {
		t.Error("a missing autoCQ must not count as known")
	}
}

// Der wichtige Unterschied: "steht auf false" ist eine Aussage des Nutzers,
// "steht nicht da" ist keine.
func TestAutoCQFalseDiffersFromAbsent(t *testing.T) {
	value, known := boolEntry(parseConfigEntries("autoCQ=false\r\n"), "autoCQ")
	if value || !known {
		t.Errorf("autoCQ=false gave %v/%v, want false/true", value, known)
	}
	if _, known := boolEntry(parseConfigEntries("\r\n"), "autoCQ"); known {
		t.Error("an empty file must not report autoCQ as known")
	}
}

// Kaputte Werte sind wie fehlende. Der Konverter setzt eine ungültige Zeile
// beim nächsten Start selbst zurück — bis dahin darf das Fenster sie nicht als
// gültige Einstellung ausgeben.
func TestBrokenValuesAreTreatedAsMissing(t *testing.T) {
	entries := parseConfigEntries("maxResolution=hd\r\nautoCQ=vielleicht\r\ntargetCQ=\r\n")
	if got := intEntry(entries, "maxResolution"); got != 0 {
		t.Errorf("maxResolution = %d, want 0 for a broken value", got)
	}
	if got := intEntry(entries, "targetCQ"); got != 0 {
		t.Errorf("targetCQ = %d, want 0 for an empty value", got)
	}
	if _, known := boolEntry(entries, "autoCQ"); known {
		t.Error("a broken autoCQ must not count as known")
	}
}

// Der wichtigste Test: die ECHTE INI im tools-Ordner. Vorlagen im Test können
// jederzeit von der Wirklichkeit abweichen — ein geändertes Format oder ein
// umbenannter Schlüssel fällt nur hier auf. Ist keine INI da (frisches
// Verzeichnis, der Konverter lief noch nie), wird übersprungen statt zu
// scheitern.
func TestConfigViewReadsTheRealFile(t *testing.T) {
	view := readConfigView()
	if !view.Found {
		t.Skip("no NVENCForge_Config.ini here yet: " + view.Note)
	}

	values := map[string]int{
		"maxResolution":         view.MaxResolution,
		"maxBitrate1080p":       view.MaxBitrate1080p,
		"maxBitrateOriginal":    view.MaxBitrateOriginal,
		"av1MaxBitrate1080p":    view.AV1MaxBitrate1080p,
		"av1MaxBitrateOriginal": view.AV1MaxBitrateOriginal,
		"targetCQ":              view.TargetCQ,
		"av1TargetCQ":           view.AV1TargetCQ,
		"autoCQTargetVMAF":      view.AutoCQTargetVMAF,
	}
	for key, value := range values {
		if value <= 0 {
			t.Errorf("%s was not read from %s (got %d)", key, view.Path, value)
		}
	}
	if !view.AutoCQKnown {
		t.Errorf("autoCQ was not read from %s", view.Path)
	}
	// Die Deckel für Originalauflösung müssen höher liegen als die verkleinerten
	// — darauf beruht der Hinweis im Fenster.
	if view.MaxBitrateOriginal <= view.MaxBitrate1080p {
		t.Errorf("the original-size cap (%d) should be above the downscaled one (%d)",
			view.MaxBitrateOriginal, view.MaxBitrate1080p)
	}
}

// Leerzeichen um Schlüssel und Wert kommen vor, wenn jemand die Datei von Hand
// bearbeitet hat.
func TestSpacesAroundKeyAndValueAreTrimmed(t *testing.T) {
	entries := parseConfigEntries("  maxResolution = 720  \r\n")
	if got := intEntry(entries, "maxResolution"); got != 720 {
		t.Errorf("maxResolution = %d, want 720", got)
	}
}
