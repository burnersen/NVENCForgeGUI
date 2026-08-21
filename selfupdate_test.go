// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// selfupdate_test.go — Prüfungen für das Selbst-Update.
//
// Geprüft wird alles, was ohne Netz und ohne laufendes Fenster prüfbar ist:
// der Versionsvergleich, das Lesen der Startargumente, der Platz der Sicherung
// und vor allem der Dateitausch samt seinem Rückweg. Genau dort könnte ein
// Fehler die Programmdatei des Nutzers kosten.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsNewerVersion prüft den Vergleich, an dem das ganze Update hängt.
func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
		why     string
	}{
		{"1.0.2", "v1.0.3", true, "eine Stelle höher"},
		{"1.0.2", "v1.0.2", false, "gleich ist nicht neuer"},
		{"1.0.2", "v1.0.1", false, "älter ist nicht neuer"},
		{"1.9.0", "v1.10.0", true, "10 ist größer als 9 — buchstabenweise wäre es andersherum"},
		{"1.10.0", "v1.9.0", false, "und in die andere Richtung genauso"},
		{"1.0.2", "v2.0.0", true, "erste Stelle schlägt alles"},
		{"1.1", "v1.1.0", false, "fehlende Stelle zählt als 0"},
		{"1.1.0", "v1.1", false, "und umgekehrt auch"},
		{"1.0.2", "1.0.3", true, "das v davor ist nicht Pflicht"},
		{"1.0.2", " v1.0.3 ", true, "Leerzeichen drumherum stören nicht"},
		{"1.0.2", "v1.0.3-beta", true, "Zusatz hinter der Nummer wird ignoriert"},
		{"1.0.2", "", false, "leere Antwort löst kein Update aus"},
		{"1.0.2", "latest", false, "unlesbare Antwort löst kein Update aus"},
	}
	for _, testCase := range cases {
		got := isNewerVersion(testCase.current, testCase.latest)
		if got != testCase.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, erwartet %v (%s)",
				testCase.current, testCase.latest, got, testCase.want, testCase.why)
		}
	}
}

// TestPredecessorPID liest die Prozessnummer aus den Startargumenten.
func TestPredecessorPID(t *testing.T) {
	cases := []struct {
		args []string
		want int
		why  string
	}{
		{[]string{"NVENCForgeGUI.exe"}, 0, "normaler Start"},
		{[]string{"NVENCForgeGUI.exe", afterUpdateFlag, "1234"}, 1234, "Update-Start"},
		{[]string{"NVENCForgeGUI.exe", afterUpdateFlag}, 0, "Nummer fehlt"},
		{[]string{"NVENCForgeGUI.exe", afterUpdateFlag, "keine-zahl"}, 0, "keine Zahl"},
		{[]string{"NVENCForgeGUI.exe", afterUpdateFlag, "0"}, 0, "0 ist keine Prozessnummer"},
		{[]string{"NVENCForgeGUI.exe", afterUpdateFlag, "-7"}, 0, "negativ ist keine Prozessnummer"},
	}
	for _, testCase := range cases {
		if got := predecessorPID(testCase.args); got != testCase.want {
			t.Errorf("predecessorPID(%v) = %d, erwartet %d (%s)",
				testCase.args, got, testCase.want, testCase.why)
		}
	}
}

// TestBackupPathFor stellt sicher, dass die Sicherung im tools-Ordner landet
// und nicht neben der Programmdatei — genau so hat der Nutzer es verlangt.
func TestBackupPathFor(t *testing.T) {
	exe := filepath.Join("C:\\", "Programme", "NVENCForgeGUI", "NVENCForgeGUI.exe")
	got := backupPathFor(exe, "1.0.2")

	wantDir := filepath.Join("C:\\", "Programme", "NVENCForgeGUI", toolsDirName)
	if filepath.Dir(got) != wantDir {
		t.Errorf("Sicherung liegt in %q, erwartet %q", filepath.Dir(got), wantDir)
	}
	if name := filepath.Base(got); name != "NVENCForgeGUI-1.0.2.exe.bak" {
		t.Errorf("Sicherung heißt %q, erwartet %q", name, "NVENCForgeGUI-1.0.2.exe.bak")
	}
	// Doppelklick-Schutz: Die Sicherung darf nicht auf .exe enden.
	if strings.HasSuffix(strings.ToLower(got), ".exe") {
		t.Errorf("Sicherung endet auf .exe: %q", got)
	}
}

// TestSwapInNewExe spielt den Tausch mit echten Dateien durch.
func TestSwapInNewExe(t *testing.T) {
	home := t.TempDir()
	currentExe := filepath.Join(home, "NVENCForgeGUI.exe")
	newFile := currentExe + ".part"
	backupPath := backupPathFor(currentExe, "1.0.2")

	writeProbeFile(t, currentExe, "alte-ausgabe")
	writeProbeFile(t, newFile, "neue-ausgabe")

	if err := swapInNewExe(currentExe, newFile, backupPath); err != nil {
		t.Fatalf("swapInNewExe: %v", err)
	}
	if content := readProbeFile(t, currentExe); content != "neue-ausgabe" {
		t.Errorf("am Platz der exe steht %q, erwartet %q", content, "neue-ausgabe")
	}
	if content := readProbeFile(t, backupPath); content != "alte-ausgabe" {
		t.Errorf("in der Sicherung steht %q, erwartet %q", content, "alte-ausgabe")
	}
	if _, err := os.Stat(newFile); err == nil {
		t.Error("die Teildatei liegt noch da — sie sollte an den Platz der exe gewandert sein")
	}
}

// TestSwapInNewExeKeepsTheProgramWhenTheSecondStepFails ist die wichtigste
// Prüfung dieser Datei: Scheitert der Tausch auf halbem Weg, muss die alte
// Programmdatei zurück an ihren Platz — sonst stünde der Nutzer ohne Programm da.
func TestSwapInNewExeKeepsTheProgramWhenTheSecondStepFails(t *testing.T) {
	home := t.TempDir()
	currentExe := filepath.Join(home, "NVENCForgeGUI.exe")
	missingFile := filepath.Join(home, "gibt-es-nicht.part")
	backupPath := backupPathFor(currentExe, "1.0.2")

	writeProbeFile(t, currentExe, "alte-ausgabe")

	if err := swapInNewExe(currentExe, missingFile, backupPath); err == nil {
		t.Fatal("swapInNewExe meldete Erfolg, obwohl die neue Datei fehlt")
	}
	if content := readProbeFile(t, currentExe); content != "alte-ausgabe" {
		t.Errorf("die alte Ausgabe steht nach dem Fehlschlag nicht mehr an ihrem Platz (%q)", content)
	}
	if _, err := os.Stat(backupPath); err == nil {
		t.Error("die Sicherung liegt noch im tools-Ordner — die Datei wurde nicht zurückgeholt")
	}
}

// TestSwapInNewExeReplacesAnOlderBackup prüft, dass ein zweiter Versuch mit
// derselben Ausgabe nicht an der Sicherung von vorhin scheitert.
func TestSwapInNewExeReplacesAnOlderBackup(t *testing.T) {
	home := t.TempDir()
	currentExe := filepath.Join(home, "NVENCForgeGUI.exe")
	newFile := currentExe + ".part"
	backupPath := backupPathFor(currentExe, "1.0.2")

	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatalf("tools-Ordner anlegen: %v", err)
	}
	writeProbeFile(t, backupPath, "sicherung-von-vorhin")
	writeProbeFile(t, currentExe, "alte-ausgabe")
	writeProbeFile(t, newFile, "neue-ausgabe")

	if err := swapInNewExe(currentExe, newFile, backupPath); err != nil {
		t.Fatalf("swapInNewExe: %v", err)
	}
	if content := readProbeFile(t, backupPath); content != "alte-ausgabe" {
		t.Errorf("in der Sicherung steht %q, erwartet %q", content, "alte-ausgabe")
	}
}

// TestVerifyDownloadedSize prüft die Notbremse gegen halbe Programmdateien.
func TestVerifyDownloadedSize(t *testing.T) {
	home := t.TempDir()

	full := filepath.Join(home, "voll.part")
	writeProbeFile(t, full, "12345")
	if err := verifyDownloadedSize(full, 5); err != nil {
		t.Errorf("vollständige Datei wurde abgelehnt: %v", err)
	}
	if err := verifyDownloadedSize(full, 0); err != nil {
		t.Errorf("ohne angekündigte Größe darf nur auf leer geprüft werden: %v", err)
	}
	if err := verifyDownloadedSize(full, 9); err == nil {
		t.Error("halbe Datei wurde durchgelassen")
	}

	empty := filepath.Join(home, "leer.part")
	writeProbeFile(t, empty, "")
	if err := verifyDownloadedSize(empty, 0); err == nil {
		t.Error("leere Datei wurde durchgelassen")
	}

	if err := verifyDownloadedSize(filepath.Join(home, "gibt-es-nicht.part"), 5); err == nil {
		t.Error("fehlende Datei wurde durchgelassen")
	}
}

// TestCanWriteInto prüft beide Antworten: schreibbarer Ordner und ein Ordner,
// den es gar nicht gibt.
func TestCanWriteInto(t *testing.T) {
	home := t.TempDir()
	if err := canWriteInto(home); err != nil {
		t.Errorf("schreibbarer Ordner wurde abgelehnt: %v", err)
	}
	if err := canWriteInto(filepath.Join(home, "gibt-es-nicht")); err == nil {
		t.Error("fehlender Ordner galt als schreibbar")
	}
}

// TestWaitForPredecessorReturnsWithoutAProcess stellt sicher, dass ein ganz
// normaler Start nicht hängenbleibt.
func TestWaitForPredecessorReturnsWithoutAProcess(t *testing.T) {
	// 0 heißt "kein Vorgänger" und muss sofort zurückkommen.
	waitForPredecessor(0, predecessorWait)
	// Eine Prozessnummer, die es sicher nicht gibt, ebenso: Das Fenster darf
	// niemals wegen einer Zahl aus den Startargumenten stehenbleiben.
	waitForPredecessor(0x7FFFFFF0, predecessorWait)
}

// writeFile legt eine Prüfdatei an und bricht bei Fehlern ab.
func writeProbeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("Datei %s schreiben: %v", path, err)
	}
}

// readFile liest eine Prüfdatei und bricht bei Fehlern ab.
func readProbeFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Datei %s lesen: %v", path, err)
	}
	return string(raw)
}
