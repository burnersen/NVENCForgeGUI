package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleCleanerFile = `# SRTCleaner Configuration
# ------------------------
# Enter phrases here (case is ignored).
#
untertitel
funk, 2017
=2017
# stillgelegt
whisper
`

func TestParseSRTPhrases(t *testing.T) {
	phrases := parseSRTPhrases(sampleCleanerFile)

	want := []SRTPhrase{
		{Text: "untertitel"},
		{Text: "funk, 2017"},
		{Text: "2017", Exact: true},
		{Text: "whisper"},
	}
	if len(phrases) != len(want) {
		t.Fatalf("%d Phrasen gelesen, erwartet %d: %+v", len(phrases), len(want), phrases)
	}
	for i := range want {
		if phrases[i] != want[i] {
			t.Errorf("Phrase %d = %+v, erwartet %+v", i, phrases[i], want[i])
		}
	}
}

func TestParseSRTPhrasesIgnoresBlanksAndComments(t *testing.T) {
	phrases := parseSRTPhrases("# nur Kommentar\n\n   \n")
	if len(phrases) != 0 {
		t.Errorf("erwartet keine Phrasen, bekommen: %+v", phrases)
	}
}

func TestCleanSRTPhrasesTrimsAndDeduplicates(t *testing.T) {
	cleaned, err := cleanSRTPhrases([]SRTPhrase{
		{Text: "  werbung  "},
		{Text: "WERBUNG"},
		{Text: ""},
		{Text: "   "},
		{Text: "werbung", Exact: true},
	})
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	// "werbung" doppelt (Groß/Klein egal) → einmal; die exakte Fassung ist ein
	// eigener Eintrag, weil sie anders wirkt.
	want := []SRTPhrase{{Text: "werbung"}, {Text: "werbung", Exact: true}}
	if len(cleaned) != len(want) {
		t.Fatalf("%d Phrasen übrig, erwartet %d: %+v", len(cleaned), len(want), cleaned)
	}
	for i := range want {
		if cleaned[i] != want[i] {
			t.Errorf("Phrase %d = %+v, erwartet %+v", i, cleaned[i], want[i])
		}
	}
}

// Ein getipptes "=" vorne gehört der Ankreuzbox — sonst stünde am Ende "==2017"
// in der Datei und die Phrase liefe ins Leere.
func TestCleanSRTPhrasesMovesTypedEqualsIntoTheFlag(t *testing.T) {
	cleaned, err := cleanSRTPhrases([]SRTPhrase{{Text: "=2017"}})
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if len(cleaned) != 1 || cleaned[0].Text != "2017" || !cleaned[0].Exact {
		t.Errorf("erwartet {2017 exakt}, bekommen: %+v", cleaned)
	}
}

func TestCleanSRTPhrasesRejectsComment(t *testing.T) {
	if _, err := cleanSRTPhrases([]SRTPhrase{{Text: "# keine Phrase"}}); err == nil {
		t.Error("eine Phrase mit \"#\" muss abgelehnt werden — sie würde zum Kommentar")
	}
}

func TestCleanSRTPhrasesRejectsLineBreak(t *testing.T) {
	if _, err := cleanSRTPhrases([]SRTPhrase{{Text: "erste\nzweite"}}); err == nil {
		t.Error("eine Phrase mit Zeilenumbruch muss abgelehnt werden")
	}
}

func TestBuildSRTCleanerFileKeepsComments(t *testing.T) {
	comments := commentLines(sampleCleanerFile)
	if len(comments) != 5 {
		t.Fatalf("%d Kommentarzeilen gefunden, erwartet 5: %q", len(comments), comments)
	}
	out := buildSRTCleanerFile(comments, []SRTPhrase{{Text: "werbung"}, {Text: "2017", Exact: true}})

	if !strings.Contains(out, "# SRTCleaner Configuration") {
		t.Error("der erklärende Kopf fehlt in der neuen Datei")
	}
	if !strings.Contains(out, "# stillgelegt") {
		t.Error("eine stillgelegte Phrase wurde weggeworfen")
	}
	if !strings.Contains(out, "\nwerbung\n") {
		t.Error("die Phrase fehlt")
	}
	if !strings.Contains(out, "\n=2017\n") {
		t.Error("die exakte Phrase braucht ihr \"=\"")
	}
}

// Runde: schreiben, zurücklesen, muss gleich sein.
func TestWriteSRTCleanerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, srtCleanerFileName)
	if err := os.WriteFile(path, []byte(sampleCleanerFile), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	want := []SRTPhrase{{Text: "werbung"}, {Text: "2017", Exact: true}}
	result, err := writeSRTCleanerTo(path, want)
	if err != nil {
		t.Fatalf("writeSRTCleanerTo: %v", err)
	}
	if result.Written != 2 {
		t.Errorf("Written = %d, erwartet 2", result.Written)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("zurücklesen: %v", err)
	}
	got := parseSRTPhrases(string(raw))
	if len(got) != len(want) {
		t.Fatalf("%d Phrasen zurückgelesen, erwartet %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Phrase %d = %+v, erwartet %+v", i, got[i], want[i])
		}
	}
}

func TestWriteSRTCleanerKeepsBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, srtCleanerFileName)
	if err := os.WriteFile(path, []byte(sampleCleanerFile), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := writeSRTCleanerTo(path, []SRTPhrase{{Text: "neu"}}); err != nil {
		t.Fatalf("writeSRTCleanerTo: %v", err)
	}

	backup, err := os.ReadFile(path + backupSuffix)
	if err != nil {
		t.Fatalf("die Sicherungskopie fehlt: %v", err)
	}
	if string(backup) != sampleCleanerFile {
		t.Error("die Sicherungskopie enthält nicht den Stand von vorher")
	}
}

// Eine abgelehnte Phrase darf die Datei nicht anfassen — sonst stünde am Ende
// eine halb gespeicherte Liste da.
func TestWriteSRTCleanerLeavesFileAloneOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, srtCleanerFileName)
	if err := os.WriteFile(path, []byte(sampleCleanerFile), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := writeSRTCleanerTo(path, []SRTPhrase{{Text: "# kaputt"}}); err == nil {
		t.Fatal("erwartet wurde ein Fehler")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lesen: %v", err)
	}
	if string(raw) != sampleCleanerFile {
		t.Error("die Datei wurde trotz Fehler verändert")
	}
	if _, err := os.Stat(path + backupSuffix); err == nil {
		t.Error("es wurde eine Sicherungskopie angelegt, obwohl gar nichts geschrieben wurde")
	}
}
