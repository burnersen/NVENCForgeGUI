// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// configfile.go — die INI des Konverters lesen und wertgenau zurückschreiben.
//
// Die Einstellungsseite wird NICHT im Fenster nachgebaut, sondern aus der INI
// selbst erzeugt: Reihenfolge, Gruppen, Erklärungen, erlaubte Werte und
// Standardwerte stehen dort bereits, geschrieben vom Konverter. Damit kann die
// Oberfläche nicht veralten — und bringt eine neue Konverter-Ausgabe weitere
// Einstellungen mit, erscheinen sie ohne eine Zeile Arbeit.
//
// Beim Schreiben wird ausschließlich die rechte Seite einer Wertzeile ersetzt.
// Kommentare, Reihenfolge, Einrückung und Zeilenenden bleiben Zeichen für
// Zeichen erhalten; genau so hält es der Konverter selbst, wenn er eine
// ungültige Zeile zurücksetzt (Config.go, resetInvalidConfigLines).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// backupSuffix hängt an der Sicherungskopie, die vor jedem Schreiben entsteht.
// Sie wird jedes Mal überschrieben und hält deshalb immer den Stand von VOR
// der letzten Änderung. Daneben entsteht einmal je Programmstart eine zweite,
// unangetastete Kopie mit Zeitstempel (siehe writeSessionBackup) — seit die
// Konvertieren-Seite jede Änderung sofort zurückschreibt, wäre eine einzige
// Sicherung nach ein paar Klicks wertlos.
const backupSuffix = ".bak"

// Gruppen der INI. Die Datei teilt sich selbst in einen kurzen ersten Teil und
// einen ausführlichen zweiten; die Oberfläche zeigt den zweiten eingeklappt.
const (
	groupCommon = "common"
	groupExpert = "expert"
)

// SettingEntry ist eine Einstellung mit allem, was die Oberfläche über sie
// wissen muss — alles aus der INI gelesen, nichts davon hier festgelegt.
type SettingEntry struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Default     string `json:"default"`     // aus "… | Default: X"
	Allowed     string `json:"allowed"`     // aus "Allowed: X | …"
	Description string `json:"description"` // die übrigen Kommentarzeilen darüber
	Group       string `json:"group"`       // common oder expert
	Section     string `json:"section"`     // Zwischenüberschrift, z. B. "Speed"
}

// SettingsFile ist der Stand der ganzen Datei.
type SettingsFile struct {
	Found    bool           `json:"found"`
	Path     string         `json:"path"`
	Note     string         `json:"note"`
	Settings []SettingEntry `json:"settings"`
}

// SaveResult meldet zurück, was beim Speichern passiert ist. Der Pfad der
// Sicherungskopie gehört dazu, damit die Oberfläche ihn nennen kann statt nur
// zu behaupten, es gäbe eine.
type SaveResult struct {
	Written    int    `json:"written"`
	BackupPath string `json:"backupPath"`
	// SessionBackupPath nennt die Kopie vom Programmstart. Sie steht im
	// Fenster, damit der Weg zurück bekannt ist, bevor man ihn braucht.
	SessionBackupPath string `json:"sessionBackupPath"`
	Note              string `json:"note"`
}

// readSettingsFile liest die INI und zerlegt sie in Einstellungen.
func readSettingsFile() SettingsFile {
	path, found := locateConfig()
	if !found {
		return SettingsFile{Note: "NVENCForge_Config.ini is not there yet — it appears the first time NVENCForge runs."}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return SettingsFile{Path: path, Note: fmt.Sprintf("NVENCForge_Config.ini could not be read: %v", err)}
	}
	return SettingsFile{Found: true, Path: path, Settings: parseSettings(string(content))}
}

// parseSettings liest die Datei so, wie ein Mensch sie liest: Der Kommentarblock
// direkt über einer Wertzeile gehört zu ihr.
//
// Eine Leerzeile beendet einen Block. Ohne diese Regel würde die Überschrift
// eines Abschnitts an der ersten Einstellung darunter kleben.
func parseSettings(content string) []SettingEntry {
	var settings []SettingEntry
	var description []string
	group, section, allowed, defaultValue := groupCommon, "", "", ""

	clear := func() {
		description = nil
		allowed, defaultValue = "", ""
	}

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))

		if line == "" {
			clear()
			continue
		}
		if strings.HasPrefix(line, "#") {
			comment := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			switch {
			case comment == "" || strings.HasPrefix(comment, "==="):
				// Trennlinie, kein Inhalt.
			case strings.HasPrefix(comment, "PART "):
				group = groupCommon
				if strings.Contains(strings.ToLower(comment), "expert") {
					group = groupExpert
				}
				section = ""
				clear()
			case strings.HasPrefix(comment, "---"):
				section = strings.TrimSpace(strings.Trim(comment, "- "))
				clear()
			case strings.HasPrefix(comment, "Allowed:"):
				allowed, defaultValue = splitAllowedAndDefault(comment)
			default:
				description = append(description, comment)
			}
			continue
		}

		key, value, separated := strings.Cut(line, "=")
		if !separated {
			continue
		}
		settings = append(settings, SettingEntry{
			Key:         strings.TrimSpace(key),
			Value:       strings.TrimSpace(value),
			Default:     defaultValue,
			Allowed:     allowed,
			Description: strings.Join(description, " "),
			Group:       group,
			Section:     section,
		})
		clear()
	}
	return settings
}

// splitAllowedAndDefault zerlegt "Allowed: 1 to 51   |   Default: 26".
//
// Fehlt der Default-Teil (bei extraFilenameChars ist er leer), bleibt er leer —
// die Oberfläche zeigt dann keinen Rückstell-Pfeil an, statt einen erfundenen
// Wert anzubieten.
func splitAllowedAndDefault(comment string) (allowed, defaultValue string) {
	body := strings.TrimSpace(strings.TrimPrefix(comment, "Allowed:"))
	allowedPart, defaultPart, hasDefault := strings.Cut(body, "|")
	allowed = strings.TrimSpace(allowedPart)
	if hasDefault {
		defaultValue = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(defaultPart), "Default:"))
	}
	return allowed, defaultValue
}

// settingsByKey macht aus den Einträgen die schlichte Nachschlagetabelle, die
// die Konvertieren-Seite braucht.
func settingsByKey(settings []SettingEntry) map[string]string {
	values := make(map[string]string, len(settings))
	for _, setting := range settings {
		values[setting.Key] = setting.Value
	}
	return values
}

// writeSettings schreibt geänderte Werte in die INI.
//
// Ablauf mit Absicht in dieser Reihenfolge: erst die Sicherungskopie, dann in
// eine Nebendatei schreiben, dann umbenennen. Bricht irgendetwas ab, ist die
// alte INI entweder unversehrt oder als .bak vorhanden — halb geschriebene
// Einstellungen kann es nicht geben.
func writeSettings(values map[string]string) (SaveResult, error) {
	if len(values) == 0 {
		return SaveResult{Note: "nothing to save"}, nil
	}
	path, found := locateConfig()
	if !found {
		return SaveResult{}, fmt.Errorf("configfile.go: writeSettings: NVENCForge_Config.ini not found")
	}
	return writeSettingsTo(path, values)
}

// writeSettingsTo erledigt die eigentliche Arbeit an einer benannten Datei.
//
// Getrennt vom Suchen der INI, damit die Tests an einer Wegwerf-Kopie arbeiten
// können: Ein Test, der die echte Einstellungsdatei des Nutzers beschreibt,
// wäre selbst dann ein Fehler, wenn er funktioniert.
func writeSettingsTo(path string, values map[string]string) (SaveResult, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return SaveResult{}, fmt.Errorf("configfile.go: writeSettings (read): %w", err)
	}

	updated, written, missing := replaceValues(string(original), values)
	if len(missing) > 0 {
		return SaveResult{}, fmt.Errorf(
			"configfile.go: writeSettings: these settings are no longer in the file: %s",
			strings.Join(missing, ", "))
	}

	sessionBackup, err := writeSessionBackup(path, original)
	if err != nil {
		return SaveResult{}, err
	}
	backupPath := path + backupSuffix
	if err := os.WriteFile(backupPath, original, 0o644); err != nil {
		return SaveResult{}, fmt.Errorf("configfile.go: writeSettings (backup): %w", err)
	}

	tempPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".new")
	if err := os.WriteFile(tempPath, []byte(updated), 0o644); err != nil {
		return SaveResult{}, fmt.Errorf("configfile.go: writeSettings (temp file): %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return SaveResult{}, fmt.Errorf("configfile.go: writeSettings (replace): %w", err)
	}
	return SaveResult{Written: written, BackupPath: backupPath, SessionBackupPath: sessionBackup}, nil
}

// replaceValues ersetzt in jeder betroffenen Zeile nur das, was rechts vom
// ersten "=" steht. Zurück kommen der neue Inhalt, die Zahl der geänderten
// Zeilen und die Schlüssel, für die keine Zeile existiert.
func replaceValues(content string, values map[string]string) (updated string, written int, missing []string) {
	remaining := make(map[string]string, len(values))
	for key, value := range values {
		remaining[key] = value
	}

	lines := strings.Split(content, "\n")
	for index, rawLine := range lines {
		lineEnd := ""
		line := rawLine
		if strings.HasSuffix(line, "\r") {
			lineEnd = "\r"
			line = strings.TrimSuffix(line, "\r")
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		keyPart, _, separated := strings.Cut(line, "=")
		if !separated {
			continue
		}
		key := strings.TrimSpace(keyPart)
		newValue, wanted := remaining[key]
		if !wanted {
			continue
		}
		delete(remaining, key)
		lines[index] = keyPart + "=" + newValue + lineEnd
		written++
	}

	for key := range remaining {
		missing = append(missing, key)
	}
	return strings.Join(lines, "\n"), written, missing
}

// ----------------------------------------------------------------------------
// Sicherung je Programmstart
// ----------------------------------------------------------------------------

// Warum es zwei Sicherungen gibt: Die ".bak" wird bei JEDEM Speichern neu
// geschrieben. Seit die Konvertieren-Seite ihre Änderungen sofort in die INI
// zurückschreibt, ist sie nach drei Klicks nur noch der Stand von vor dem
// dritten Klick — der Zustand vom Morgen wäre unwiederbringlich weg.
//
// Deshalb zusätzlich EINE unangetastete Kopie je Programmstart. Sie trägt den
// Zeitstempel des Starts im Namen, wird in dieser Sitzung nie wieder angefasst
// und niemals von selbst gelöscht.
var (
	sessionBackupMu sync.Mutex

	// Je DATEI einmal, nicht je Sitzung einmal: Der tools-Ordner kann während
	// des Laufs wechseln (die exe wird nachgeladen, ein Ordner wird ausgetauscht),
	// und dann bekäme die zweite Datei nie eine Sicherung — obwohl sie genauso
	// beschrieben wird.
	sessionBackupPaths = map[string]bool{}

	// sessionStamp steht fest, sobald das Fenster läuft — nicht erst beim
	// ersten Speichern. Sonst hieße die Datei nach dem Zeitpunkt der Änderung
	// und nicht nach dem Stand, den sie bewahrt.
	sessionStamp = time.Now().Format("2006-01-02_1504")
)

// writeSessionBackup legt die Kopie an, falls diese Datei in dieser Sitzung
// noch keine hat. Der Pfad kommt zurück, damit das Fenster ihn nennen kann.
func writeSessionBackup(path string, original []byte) (string, error) {
	sessionBackupMu.Lock()
	defer sessionBackupMu.Unlock()

	target := path + backupSuffix + "-" + sessionStamp
	if sessionBackupPaths[path] {
		return target, nil
	}
	if err := os.WriteFile(target, original, 0o644); err != nil {
		return "", fmt.Errorf("configfile.go: writeSessionBackup: %w", err)
	}
	sessionBackupPaths[path] = true
	return target, nil
}

// writeKnownSettings schreibt nur die Werte, für die es in der INI auch eine
// Zeile gibt.
//
// Gedacht für Profile: Ein gespeicherter Satz kann aus einer anderen
// NVENCForge-Ausgabe stammen und Schlüssel enthalten, die diese Datei (noch)
// nicht hat. writeSettings lehnt so etwas komplett ab — richtig für die
// Einstellungsseite, wo jeder Schlüssel gerade erst aus der Datei kam, aber
// falsch für ein Profil: Dann ließe sich ein älteres Profil gar nicht mehr
// laden, obwohl 29 seiner 30 Werte passen.
//
// Was übersprungen wurde, steht in der Rückmeldung — stillschweigend fehlende
// Einstellungen wären genau die Art Überraschung, die dieses Fenster vermeiden
// soll.
func writeKnownSettings(values map[string]string) (SaveResult, error) {
	if len(values) == 0 {
		return SaveResult{Note: "this profile has no settings of its own"}, nil
	}
	file := readSettingsFile()
	if !file.Found {
		return SaveResult{}, fmt.Errorf("configfile.go: writeKnownSettings: %s", file.Note)
	}

	known := make(map[string]bool, len(file.Settings))
	for _, entry := range file.Settings {
		known[entry.Key] = true
	}

	usable := make(map[string]string, len(values))
	var skipped []string
	for key, value := range values {
		if known[key] {
			usable[key] = value
			continue
		}
		skipped = append(skipped, key)
	}
	if len(usable) == 0 {
		return SaveResult{Note: "none of the profile's settings exist in this NVENCForge_Config.ini"}, nil
	}

	result, err := writeSettings(usable)
	if err != nil {
		return SaveResult{}, err
	}
	if len(skipped) > 0 {
		sort.Strings(skipped)
		result.Note = fmt.Sprintf("%d setting(s) of this profile are not in your NVENCForge_Config.ini and were left out: %s",
			len(skipped), strings.Join(skipped, ", "))
	}
	return result, nil
}
