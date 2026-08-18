// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ein Ausschnitt im echten Aufbau: Überschriften, Kommentarblöcke, CRLF.
const sampleFile = "# NVENCForge Configuration\r\n" +
	"# =====================================================================\r\n" +
	"\r\n" +
	"# =====================================================================\r\n" +
	"#  PART 1  -  the handful of settings people actually change\r\n" +
	"# =====================================================================\r\n" +
	"\r\n" +
	"# Videos larger than this are scaled down (short edge, in pixels).\r\n" +
	"# 1080 is Full HD. Set 2160 to keep 4K material at 4K.\r\n" +
	"# Allowed: 720, 1080, 1440, 2160   |   Default: 1080\r\n" +
	"maxResolution=1080\r\n" +
	"\r\n" +
	"# =====================================================================\r\n" +
	"#  PART 2  -  expert settings\r\n" +
	"# =====================================================================\r\n" +
	"\r\n" +
	"# --- Speed ---\r\n" +
	"\r\n" +
	"# Unpack the source video on the graphics card.\r\n" +
	"# Allowed: true, false   |   Default: true\r\n" +
	"gpuDecode=true\r\n" +
	"\r\n" +
	"# Characters that survive file name cleaning.\r\n" +
	"# Allowed: any characters, or empty   |   Default: \r\n" +
	"extraFilenameChars=\r\n"

func settingWithKey(t *testing.T, settings []SettingEntry, key string) SettingEntry {
	t.Helper()
	for _, setting := range settings {
		if setting.Key == key {
			return setting
		}
	}
	t.Fatalf("setting %q was not parsed", key)
	return SettingEntry{}
}

func TestParseSettingsKeepsTheOrderOfTheFile(t *testing.T) {
	settings := parseSettings(sampleFile)
	want := []string{"maxResolution", "gpuDecode", "extraFilenameChars"}
	if len(settings) != len(want) {
		t.Fatalf("parsed %d settings, want %d", len(settings), len(want))
	}
	for index, key := range want {
		if settings[index].Key != key {
			t.Errorf("position %d is %q, want %q", index, settings[index].Key, key)
		}
	}
}

func TestParseSettingsReadsEverythingAroundAValue(t *testing.T) {
	settings := parseSettings(sampleFile)

	resolution := settingWithKey(t, settings, "maxResolution")
	if resolution.Value != "1080" || resolution.Default != "1080" {
		t.Errorf("value/default = %q/%q, want 1080/1080", resolution.Value, resolution.Default)
	}
	if resolution.Allowed != "720, 1080, 1440, 2160" {
		t.Errorf("allowed = %q", resolution.Allowed)
	}
	if resolution.Group != groupCommon {
		t.Errorf("group = %q, want %q", resolution.Group, groupCommon)
	}
	if !strings.HasPrefix(resolution.Description, "Videos larger than this") ||
		!strings.Contains(resolution.Description, "1080 is Full HD") {
		t.Errorf("description lost lines: %q", resolution.Description)
	}

	decode := settingWithKey(t, settings, "gpuDecode")
	if decode.Group != groupExpert {
		t.Errorf("gpuDecode group = %q, want %q", decode.Group, groupExpert)
	}
	if decode.Section != "Speed" {
		t.Errorf("gpuDecode section = %q, want Speed", decode.Section)
	}
	if decode.Allowed != "true, false" || decode.Default != "true" {
		t.Errorf("gpuDecode allowed/default = %q/%q", decode.Allowed, decode.Default)
	}
}

// Ein leerer Standardwert ist eine echte Angabe (extraFilenameChars hat keinen).
// Er darf nicht aus Versehen den Wert des Nachbarn erben.
func TestEmptyDefaultStaysEmpty(t *testing.T) {
	setting := settingWithKey(t, parseSettings(sampleFile), "extraFilenameChars")
	if setting.Default != "" {
		t.Errorf("default = %q, want empty", setting.Default)
	}
	if setting.Value != "" {
		t.Errorf("value = %q, want empty", setting.Value)
	}
}

// Überschriften und Trennlinien dürfen nicht in der Erklärung des nächsten
// Wertes landen — sonst stünde in jeder zweiten Sprechblase "PART 2".
func TestHeadingsDoNotLeakIntoDescriptions(t *testing.T) {
	for _, setting := range parseSettings(sampleFile) {
		if strings.Contains(setting.Description, "PART") ||
			strings.Contains(setting.Description, "===") ||
			strings.Contains(setting.Description, "---") {
			t.Errorf("%s carries a heading in its description: %q", setting.Key, setting.Description)
		}
	}
}

// Die ECHTE INI: Format und Schlüsselnamen ändern sich nur dort, und die
// Vorlage oben würde davon nichts merken.
func TestParseSettingsAgainstTheRealFile(t *testing.T) {
	file := readSettingsFile()
	if !file.Found {
		t.Skip("no NVENCForge_Config.ini here yet: " + file.Note)
	}
	if len(file.Settings) < 25 {
		t.Errorf("only %d settings parsed from %s — the converter documents around 29",
			len(file.Settings), file.Path)
	}
	var withoutHelp, withoutDefault []string
	for _, setting := range file.Settings {
		if setting.Description == "" || setting.Allowed == "" {
			withoutHelp = append(withoutHelp, setting.Key)
		}
		// extraFilenameChars hat als einziger bewusst keinen Standardwert.
		if setting.Default == "" && setting.Key != "extraFilenameChars" {
			withoutDefault = append(withoutDefault, setting.Key)
		}
	}
	if len(withoutHelp) > 0 {
		t.Errorf("no description or allowed range for: %s", strings.Join(withoutHelp, ", "))
	}
	if len(withoutDefault) > 0 {
		t.Errorf("no default value for: %s", strings.Join(withoutDefault, ", "))
	}
}

func TestReplaceValuesTouchesOnlyTheValue(t *testing.T) {
	updated, written, missing := replaceValues(sampleFile, map[string]string{"maxResolution": "2160"})
	if written != 1 || len(missing) != 0 {
		t.Fatalf("written %d, missing %v, want 1 and none", written, missing)
	}
	if !strings.Contains(updated, "maxResolution=2160\r\n") {
		t.Error("the new value is not in the file")
	}
	// Alles andere muss Zeichen für Zeichen stehen bleiben: Kommentare, die
	// Reihenfolge, die anderen Werte und die Zeilenenden.
	if !strings.Contains(updated, "# 1080 is Full HD. Set 2160 to keep 4K material at 4K.\r\n") {
		t.Error("a comment line was changed")
	}
	if !strings.Contains(updated, "gpuDecode=true\r\n") {
		t.Error("another setting was changed")
	}
	if strings.Contains(updated, "maxResolution=1080") {
		t.Error("the old value is still there")
	}
	if before, after := strings.Count(sampleFile, "\r\n"), strings.Count(updated, "\r\n"); before != after {
		t.Errorf("line endings changed: %d CRLF before, %d after", before, after)
	}
}

// Eine Kommentarzeile, die zufällig wie der gesuchte Schlüssel aussieht, darf
// nicht beschrieben werden.
func TestReplaceValuesIgnoresComments(t *testing.T) {
	content := "# maxResolution=720\r\nmaxResolution=1080\r\n"
	updated, written, _ := replaceValues(content, map[string]string{"maxResolution": "1440"})
	if written != 1 {
		t.Errorf("written = %d, want 1", written)
	}
	if !strings.Contains(updated, "# maxResolution=720\r\n") {
		t.Error("the comment was modified")
	}
	if !strings.Contains(updated, "maxResolution=1440\r\n") {
		t.Error("the value was not written")
	}
}

// Ein Schlüssel, den es in der Datei nicht (mehr) gibt, wird gemeldet statt
// still verschluckt oder ans Ende geschrieben.
func TestReplaceValuesReportsUnknownKeys(t *testing.T) {
	_, written, missing := replaceValues(sampleFile, map[string]string{"doesNotExist": "1"})
	if written != 0 {
		t.Errorf("written = %d, want 0", written)
	}
	if len(missing) != 1 || missing[0] != "doesNotExist" {
		t.Errorf("missing = %v, want [doesNotExist]", missing)
	}
}

func TestWriteSettingsBacksUpTheOriginal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, []byte(sampleFile), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := writeSettingsTo(path, map[string]string{"maxResolution": "720", "gpuDecode": "false"})
	if err != nil {
		t.Fatalf("writeSettingsTo: %v", err)
	}
	if result.Written != 2 {
		t.Errorf("written = %d, want 2", result.Written)
	}

	backup, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("no backup at %s: %v", result.BackupPath, err)
	}
	if string(backup) != sampleFile {
		t.Error("the backup does not match the original file")
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "maxResolution=720\r\n") ||
		!strings.Contains(string(written), "gpuDecode=false\r\n") {
		t.Errorf("values were not written: %q", string(written))
	}
	// Kein Rest der Nebendatei darf liegen bleiben.
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".*"))
	if len(leftovers) > 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

// Die schärfste Probe: die ECHTE INI, alle Werte auf ihren eigenen Wert
// geschrieben. Danach muss die Datei Byte für Byte dieselbe sein — jede
// verlorene Kommentarzeile, jedes umgewandelte Zeilenende und jeder verrutschte
// Wert fällt hier auf. Gearbeitet wird auf einer Kopie im Wegwerf-Ordner; die
// Datei des Nutzers wird nicht angefasst.
func TestRewritingTheRealFileChangesNothing(t *testing.T) {
	file := readSettingsFile()
	if !file.Found {
		t.Skip("no NVENCForge_Config.ini here yet: " + file.Note)
	}
	original, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), configFileName)
	if err := os.WriteFile(copyPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	values := make(map[string]string, len(file.Settings))
	for _, setting := range file.Settings {
		values[setting.Key] = setting.Value
	}
	result, err := writeSettingsTo(copyPath, values)
	if err != nil {
		t.Fatalf("writeSettingsTo: %v", err)
	}
	if result.Written != len(values) {
		t.Errorf("wrote %d of %d settings — some lines were not found",
			result.Written, len(values))
	}
	after, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, after) {
		t.Errorf("writing every value unchanged altered the file (%d bytes before, %d after)",
			len(original), len(after))
	}
}

// Und die Gegenprobe: Wird EIN Wert geändert, darf sich auch nur EINE Zeile
// unterscheiden.
func TestChangingOneValueChangesOneLine(t *testing.T) {
	file := readSettingsFile()
	if !file.Found {
		t.Skip("no NVENCForge_Config.ini here yet")
	}
	original, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(t.TempDir(), configFileName)
	if err := os.WriteFile(copyPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeSettingsTo(copyPath, map[string]string{"maxResolution": "1440"}); err != nil {
		t.Fatalf("writeSettingsTo: %v", err)
	}
	after, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}

	before, now := strings.Split(string(original), "\n"), strings.Split(string(after), "\n")
	if len(before) != len(now) {
		t.Fatalf("line count changed: %d -> %d", len(before), len(now))
	}
	differing := 0
	for index := range before {
		if before[index] != now[index] {
			differing++
			if !strings.HasPrefix(strings.TrimSpace(now[index]), "maxResolution=") {
				t.Errorf("line %d changed but is not the one we asked for: %q", index+1, now[index])
			}
		}
	}
	if differing != 1 {
		t.Errorf("%d lines changed, want exactly 1", differing)
	}
}

// Schlägt das Schreiben fehl, muss die INI unverändert bleiben. Geprüft am
// realistischsten Fall: einer der Schlüssel existiert nicht mehr.
func TestWriteSettingsLeavesTheFileAloneOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, []byte(sampleFile), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := writeSettingsTo(path, map[string]string{"maxResolution": "720", "gone": "1"}); err == nil {
		t.Fatal("expected an error for a key that is not in the file")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != sampleFile {
		t.Error("the file was changed even though saving failed")
	}
	if _, err := os.Stat(path + backupSuffix); err == nil {
		t.Error("a backup was written even though nothing was saved")
	}
}
