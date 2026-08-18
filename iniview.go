// iniview.go — die INI des Konverters lesen, um sie ANZUZEIGEN.
//
// Geschrieben wird hier nichts. Das Fenster soll nur sagen können, was gerade
// gilt: Welcher Bitraten-Deckel greift, auf welche Höhe wird verkleinert, ist
// die automatische Qualitätssuche an. Ohne diese Werte müsste die Oberfläche
// Zahlen behaupten, die vielleicht gar nicht stimmen — und kein Bedienelement
// darf lügen.
//
// Fehlt ein Wert in der INI, bleibt er hier auf null bzw. "unbekannt". Die
// eingebauten Standardwerte des Konverters werden BEWUSST nicht nachgebaut:
// Eine Kopie davon würde beim nächsten Konverter-Release still veralten, und
// eine falsche Zahl ist schlimmer als gar keine.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// configFileName ist die INI, die der Konverter neben sich selbst führt.
const configFileName = "NVENCForge_Config.ini"

// ConfigView sind die Werte, die die Oberfläche anzeigt. Alles, was sie nicht
// anzeigt, steht hier auch nicht drin — die vollständige Bearbeitung aller
// Einstellungen kommt in einem eigenen Bereich.
type ConfigView struct {
	Found bool   `json:"found"`
	Path  string `json:"path"`
	Note  string `json:"note"`

	MaxResolution int `json:"maxResolution"`

	// Die vier Bitraten-Deckel. Welcher gilt, entscheidet der Konverter allein
	// aus Codec und Auflösungs-Modus (main.go, parseArgs): verkleinert oder in
	// Originalauflösung.
	MaxBitrate1080p       int `json:"maxBitrate1080p"`
	MaxBitrateOriginal    int `json:"maxBitrateOriginal"`
	AV1MaxBitrate1080p    int `json:"av1MaxBitrate1080p"`
	AV1MaxBitrateOriginal int `json:"av1MaxBitrateOriginal"`

	TargetCQ         int `json:"targetCQ"`
	AV1TargetCQ      int `json:"av1TargetCQ"`
	AutoCQTargetVMAF int `json:"autoCQTargetVMAF"`

	// AutoCQKnown trennt "steht auf false" von "steht gar nicht in der Datei".
	// Ohne diese Unterscheidung würde eine fehlende Zeile wie ein bewusstes
	// Abschalten aussehen.
	AutoCQ      bool `json:"autoCQ"`
	AutoCQKnown bool `json:"autoCQKnown"`

	// RetireMode entscheidet, wohin ein Original nach erfolgreicher Umwandlung
	// wandert: "folder" (Unterordner "originals") oder "recyclebin". Die
	// Oberfläche darf das nicht raten — beide Einstellungen sind üblich.
	RetireMode string `json:"retireMode"`
}

// locateConfig sucht die INI dort, wo auch die Programmdatei liegt.
func locateConfig() (string, bool) {
	if exePath, found := locateConverter(); found {
		candidate := filepath.Join(filepath.Dir(exePath), configFileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	for _, dir := range toolsDirCandidates() {
		candidate := filepath.Join(dir, configFileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// readConfigView liest die INI und stellt die Anzeigewerte zusammen.
//
// Ein Fehler wird nicht nach oben gereicht: Für eine reine Anzeige ist eine
// fehlende INI kein Grund, irgendetwas abzubrechen. Der Grund steht in Note,
// damit das Fenster ihn nennen kann, statt still leer zu bleiben.
func readConfigView() ConfigView {
	path, found := locateConfig()
	if !found {
		return ConfigView{Note: "NVENCForge_Config.ini is not there yet — it appears the first time NVENCForge runs."}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ConfigView{Path: path, Note: fmt.Sprintf("NVENCForge_Config.ini could not be read: %v", err)}
	}

	entries := settingsByKey(parseSettings(string(content)))
	view := ConfigView{Found: true, Path: path}
	view.MaxResolution = intEntry(entries, "maxResolution")
	view.MaxBitrate1080p = intEntry(entries, "maxBitrate1080p")
	view.MaxBitrateOriginal = intEntry(entries, "maxBitrateOriginal")
	view.AV1MaxBitrate1080p = intEntry(entries, "av1MaxBitrate1080p")
	view.AV1MaxBitrateOriginal = intEntry(entries, "av1MaxBitrateOriginal")
	view.TargetCQ = intEntry(entries, "targetCQ")
	view.AV1TargetCQ = intEntry(entries, "av1TargetCQ")
	view.AutoCQTargetVMAF = intEntry(entries, "autoCQTargetVMAF")
	view.AutoCQ, view.AutoCQKnown = boolEntry(entries, "autoCQ")
	view.RetireMode = strings.ToLower(strings.TrimSpace(entries["retireMode"]))
	return view
}

// intEntry liefert eine Zahl oder 0, wenn der Schlüssel fehlt oder keine ist.
// 0 heißt für alle hier gelesenen Werte "unbekannt": Weder eine Auflösung noch
// ein Bitraten-Deckel noch ein CQ darf null sein.
func intEntry(entries map[string]string, key string) int {
	number, err := strconv.Atoi(entries[key])
	if err != nil {
		return 0
	}
	return number
}

// boolEntry liefert den Wahrheitswert und ob er überhaupt dastand.
func boolEntry(entries map[string]string, key string) (value bool, known bool) {
	raw, present := entries[key]
	if !present {
		return false, false
	}
	parsed, err := strconv.ParseBool(strings.ToLower(strings.TrimSpace(raw)))
	if err != nil {
		return false, false
	}
	return parsed, true
}
