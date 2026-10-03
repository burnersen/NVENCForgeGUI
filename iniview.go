// NVENCForgeGUI (https://github.com/burnersen/NVENCForgeGUI)
// Copyright (C) 2026 burnersen
// SPDX-License-Identifier: GPL-3.0-only

// iniview.go — die INI des Konverters lesen, um sie ANZUZEIGEN.
//
// Geschrieben wird hier nichts. Das Fenster soll nur sagen können, was gerade
// gilt: auf welche Höhe wird verkleinert, ist die automatische Qualitätssuche
// an. Ohne diese Werte müsste die Oberfläche
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

	// Bitraten-Deckel gibt es seit NVENCForge 2.0.0 nicht mehr: die Qualität
	// regelt das VMAF-Ziel, die Größe minSavePercent.
	MaxResolution int `json:"maxResolution"`

	TargetCQ         int `json:"targetCQ"`
	AV1TargetCQ      int `json:"av1TargetCQ"`
	AutoCQTargetVMAF int `json:"autoCQTargetVMAF"`

	// Seit NVENCForge 2.2.0 misst Auto-CQ zusätzlich am 5-%-Perzentil der
	// Bildwerte; der Mittelwert oben bleibt als Sicherheitsnetz. 0 heißt
	// "aus" oder "diese NVENCForge-Fassung kennt es noch nicht" — beides
	// bedeutet für die Anzeige dasselbe: es zählt nur der Mittelwert.
	AutoCQVMAFPercentile       int     `json:"autoCQVMAFPercentile"`
	AutoCQTargetVMAFPercentile float64 `json:"autoCQTargetVMAFPercentile"`

	// AutoCQKnown trennt "steht auf false" von "steht gar nicht in der Datei".
	// Ohne diese Unterscheidung würde eine fehlende Zeile wie ein bewusstes
	// Abschalten aussehen.
	AutoCQ      bool `json:"autoCQ"`
	AutoCQKnown bool `json:"autoCQKnown"`

	// RetireMode entscheidet, wohin ein Original nach erfolgreicher Umwandlung
	// wandert: "folder" (Unterordner "originals") oder "recyclebin". Die
	// Oberfläche darf das nicht raten — beide Einstellungen sind üblich.
	RetireMode string `json:"retireMode"`
	// ------------------------------------------------------------------
	// Die Grundentscheidungen. Seit NVENCForge 1.23.0 stehen sie in der INI,
	// vorher gab es sie nur als Schalter der Befehlszeile.
	//
	// Jeder Wert hat ein "Known" daneben. Das trennt "steht so in der Datei"
	// von "steht gar nicht drin" — bei einer älteren INI fehlt der Schlüssel,
	// und dann darf das Fenster nichts behaupten und erst recht keinen
	// Gegenschalter schicken.
	// ------------------------------------------------------------------
	Codec      string `json:"codec"`
	CodecKnown bool   `json:"codecKnown"`

	Container      string `json:"container"`
	ContainerKnown bool   `json:"containerKnown"`

	AudioMode      string `json:"audioMode"`
	AudioModeKnown bool   `json:"audioModeKnown"`

	BitDepth      int  `json:"bitDepth"`
	BitDepthKnown bool `json:"bitDepthKnown"`

	KeepResolution      bool `json:"keepResolution"`
	KeepResolutionKnown bool `json:"keepResolutionKnown"`

	KeepSource      bool `json:"keepSource"`
	KeepSourceKnown bool `json:"keepSourceKnown"`

	// Encoder ist "nvidia" oder "cpu" und gibt es schon lange. Angezeigt wurde
	// er nie — das Fenster behauptete immer "GPU / NVENC (default)", auch wenn
	// in Wahrheit auf dem Prozessor gerechnet wurde.
	Encoder      string `json:"encoder"`
	EncoderKnown bool   `json:"encoderKnown"`

	AutoCrop      bool `json:"autoCrop"`
	AutoCropKnown bool `json:"autoCropKnown"`

	// AutoShutdown wird angezeigt, damit niemand von einem abschaltenden
	// Rechner überrascht wird: Steht es in der Datei, muss das Kästchen im
	// Fenster es zeigen und das Protokoll sagen, woher es kommt.
	AutoShutdown      bool `json:"autoShutdown"`
	AutoShutdownKnown bool `json:"autoShutdownKnown"`
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
	view.TargetCQ = intEntry(entries, "targetCQ")
	view.AV1TargetCQ = intEntry(entries, "av1TargetCQ")
	view.AutoCQTargetVMAF = intEntry(entries, "autoCQTargetVMAF")
	view.AutoCQVMAFPercentile = intEntry(entries, "autoCQVMAFPercentile")
	view.AutoCQTargetVMAFPercentile = floatEntry(entries, "autoCQTargetVMAFPercentile")
	view.AutoCQ, view.AutoCQKnown = boolEntry(entries, "autoCQ")
	view.RetireMode = strings.ToLower(strings.TrimSpace(entries["retireMode"]))
	view.Codec, view.CodecKnown = wordEntry(entries, "codec")
	view.Container, view.ContainerKnown = wordEntry(entries, "container")
	view.AudioMode, view.AudioModeKnown = wordEntry(entries, "audioMode")
	view.Encoder, view.EncoderKnown = wordEntry(entries, "encoder")
	view.BitDepth = intEntry(entries, "bitDepth")
	view.BitDepthKnown = view.BitDepth != 0
	view.KeepResolution, view.KeepResolutionKnown = boolEntry(entries, "keepResolution")
	view.KeepSource, view.KeepSourceKnown = boolEntry(entries, "keepSource")
	view.AutoCrop, view.AutoCropKnown = boolEntry(entries, "autoCrop")
	view.AutoShutdown, view.AutoShutdownKnown = boolEntry(entries, "autoShutdown")
	return view
}

// intEntry liefert eine Zahl oder 0, wenn der Schlüssel fehlt oder keine ist.
// 0 heißt für alle hier gelesenen Werte "unbekannt": Weder eine Auflösung noch
// ein CQ darf null sein.
func intEntry(entries map[string]string, key string) int {
	number, err := strconv.Atoi(entries[key])
	if err != nil {
		return 0
	}
	return number
}

// floatEntry ist intEntry für Werte mit Nachkommastelle (92.5): eine Zahl oder
// 0, wenn der Schlüssel fehlt oder keine ist.
func floatEntry(entries map[string]string, key string) float64 {
	number, err := strconv.ParseFloat(strings.TrimSpace(entries[key]), 64)
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

// wordEntry liefert einen Wert in Kleinbuchstaben und dazu, ob der Schlüssel
// überhaupt in der Datei steht.
//
// Die Unterscheidung ist wichtiger, als sie aussieht: Ein fehlender Schlüssel
// heißt "diese NVENCForge-Fassung kennt die Einstellung noch nicht". Das
// Fenster darf dann weder einen Wert anzeigen noch einen Gegenschalter
// schicken — eine ältere exe würde ihn als unbekannte Option anmeckern.
func wordEntry(entries map[string]string, key string) (value string, known bool) {
	raw, present := entries[key]
	if !present {
		return "", false
	}
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}
