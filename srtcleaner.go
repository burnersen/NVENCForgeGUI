// srtcleaner.go — die Phrasenliste des Untertitel-Reinigers lesen und schreiben.
//
// Der Konverter entfernt beim Zerlegen für DaVinci Werbe- und Störzeilen aus
// jedem .srt. WELCHE Zeilen das sind, steht in "SRTCleaner_config.txt" neben
// der Programmdatei — bisher nur mit einem Texteditor erreichbar.
//
// Wichtig: Diese Liste wirkt AUSSCHLIESSLICH im DaVinci-Modus. -split und
// -join fassen Untertitel absichtlich nicht an, und eine gewöhnliche
// Umwandlung reinigt ebenfalls nicht (geprüft am Konverter-Quelltext: cleanSRT
// wird nur aus extractSubs und runMerge gerufen). Die Oberfläche sagt das
// dazu, damit niemand hier eine Wirkung erwartet, die es nicht gibt.
//
// Aufbau der Datei: eine Phrase je Zeile, Groß-/Kleinschreibung egal. Ein
// vorangestelltes "=" heißt "nur bei genauer Übereinstimmung", ohne "=" reicht
// das Vorkommen irgendwo im Block. Zeilen mit "#" sind Kommentare.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// srtCleanerFileName ist die Datei, um die es hier geht.
const srtCleanerFileName = "SRTCleaner_config.txt"

// exactPrefix markiert eine Phrase, die genau passen muss.
const exactPrefix = "="

// SRTPhrase ist ein Eintrag der Liste.
type SRTPhrase struct {
	Text string `json:"text"`
	// Exact = nur löschen, wenn der Text EXAKT übereinstimmt (Vorsilbe "=").
	Exact bool `json:"exact"`
}

// SRTCleanerView ist, was die Oberfläche zum Anzeigen braucht.
type SRTCleanerView struct {
	Found   bool        `json:"found"`
	Path    string      `json:"path"`
	Note    string      `json:"note"`
	Phrases []SRTPhrase `json:"phrases"`
}

// locateSRTCleaner sucht die Datei dort, wo auch die Programmdatei liegt —
// dieselbe Reihenfolge wie bei der INI, damit beide immer zusammenpassen.
func locateSRTCleaner() (string, bool) {
	if exePath, found := locateConverter(); found {
		candidate := filepath.Join(filepath.Dir(exePath), srtCleanerFileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	for _, dir := range toolsDirCandidates() {
		candidate := filepath.Join(dir, srtCleanerFileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// readSRTCleaner liest die Phrasenliste.
//
// Eine fehlende Datei ist kein Fehler, den jemand sehen müsste: Sie entsteht,
// sobald der Konverter das erste Mal läuft. Der Grund steht in Note.
func readSRTCleaner() SRTCleanerView {
	path, found := locateSRTCleaner()
	if !found {
		return SRTCleanerView{
			Note: srtCleanerFileName + " is not there yet — it appears the first time NVENCForge runs.",
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return SRTCleanerView{
			Path: path,
			Note: fmt.Sprintf("%s could not be read: %v", srtCleanerFileName, err),
		}
	}
	return SRTCleanerView{
		Found:   true,
		Path:    path,
		Phrases: parseSRTPhrases(string(content)),
	}
}

// parseSRTPhrases holt die Phrasen aus dem Dateiinhalt. Kommentare und leere
// Zeilen fallen weg — sie sind Erklärung, kein Filter.
func parseSRTPhrases(content string) []SRTPhrase {
	phrases := []SRTPhrase{}
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		phrase := SRTPhrase{Text: line}
		if strings.HasPrefix(line, exactPrefix) {
			phrase.Exact = true
			phrase.Text = strings.TrimSpace(strings.TrimPrefix(line, exactPrefix))
		}
		if phrase.Text == "" {
			continue
		}
		phrases = append(phrases, phrase)
	}
	return phrases
}

// commentLines sammelt die Kommentarzeilen einer Datei.
//
// Sie werden beim Schreiben wortgetreu wieder vorangestellt: Der Kopf erklärt
// das Dateiformat, und wer eine Phrase mit "#" stillgelegt hat, soll sie
// wiederfinden. Die Oberfläche darf fremde Notizen nicht wegwerfen.
func commentLines(content string) []string {
	var comments []string
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			comments = append(comments, line)
		}
	}
	return comments
}

// buildSRTCleanerFile setzt den neuen Dateiinhalt zusammen: erst die
// Kommentare von vorher, dann eine Leerzeile, dann die Phrasen.
func buildSRTCleanerFile(comments []string, phrases []SRTPhrase) string {
	var builder strings.Builder
	for _, comment := range comments {
		builder.WriteString(comment)
		builder.WriteString("\n")
	}
	if len(comments) > 0 {
		builder.WriteString("\n")
	}
	for _, phrase := range phrases {
		if phrase.Exact {
			builder.WriteString(exactPrefix)
		}
		builder.WriteString(phrase.Text)
		builder.WriteString("\n")
	}
	return builder.String()
}

// cleanSRTPhrases prüft und bereinigt, was die Oberfläche schickt.
//
// Leere Einträge fliegen raus, doppelte ebenso (sie hätten keine zusätzliche
// Wirkung, würden die Liste aber unübersichtlich machen). Eine Phrase, die mit
// "#" beginnt, wird abgelehnt statt stillschweigend umgedeutet: Sie würde als
// Kommentar in der Datei landen und damit wirkungslos sein — genau die Art
// stiller Überraschung, die dieses Fenster vermeiden soll.
func cleanSRTPhrases(phrases []SRTPhrase) ([]SRTPhrase, error) {
	cleaned := []SRTPhrase{}
	seen := map[string]bool{}
	for _, phrase := range phrases {
		text := strings.TrimSpace(phrase.Text)
		// Ein vorangestelltes "=" gehört der Ankreuzbox, nicht dem Text.
		for strings.HasPrefix(text, exactPrefix) {
			phrase.Exact = true
			text = strings.TrimSpace(strings.TrimPrefix(text, exactPrefix))
		}
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "#") {
			return nil, fmt.Errorf(
				"srtcleaner.go: a phrase must not start with \"#\" — that line would become a comment (%q)", text)
		}
		if strings.ContainsAny(text, "\r\n") {
			return nil, fmt.Errorf("srtcleaner.go: a phrase must stay on one line (%q)", text)
		}
		key := strings.ToLower(text)
		if phrase.Exact {
			key = exactPrefix + key
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, SRTPhrase{Text: text, Exact: phrase.Exact})
	}
	return cleaned, nil
}

// writeSRTCleaner schreibt die Liste zurück.
func writeSRTCleaner(phrases []SRTPhrase) (SaveResult, error) {
	path, found := locateSRTCleaner()
	if !found {
		return SaveResult{}, fmt.Errorf("srtcleaner.go: writeSRTCleaner: %s not found", srtCleanerFileName)
	}
	return writeSRTCleanerTo(path, phrases)
}

// writeSRTCleanerTo erledigt die Arbeit an einer benannten Datei — getrennt
// vom Suchen, damit Tests an einer Wegwerf-Kopie arbeiten können.
//
// Reihenfolge wie bei der INI: erst die Sicherungskopie, dann in eine
// Nebendatei, dann umbenennen. Bricht etwas ab, ist die alte Liste entweder
// unversehrt oder als .bak vorhanden.
func writeSRTCleanerTo(path string, phrases []SRTPhrase) (SaveResult, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return SaveResult{}, fmt.Errorf("srtcleaner.go: writeSRTCleaner (read): %w", err)
	}

	cleaned, err := cleanSRTPhrases(phrases)
	if err != nil {
		return SaveResult{}, err
	}
	updated := buildSRTCleanerFile(commentLines(string(original)), cleaned)

	backupPath := path + backupSuffix
	if err := os.WriteFile(backupPath, original, 0o644); err != nil {
		return SaveResult{}, fmt.Errorf("srtcleaner.go: writeSRTCleaner (backup): %w", err)
	}

	tempPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".new")
	if err := os.WriteFile(tempPath, []byte(updated), 0o644); err != nil {
		return SaveResult{}, fmt.Errorf("srtcleaner.go: writeSRTCleaner (temp file): %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath)
		return SaveResult{}, fmt.Errorf("srtcleaner.go: writeSRTCleaner (replace): %w", err)
	}
	return SaveResult{Written: len(cleaned), BackupPath: backupPath}, nil
}
