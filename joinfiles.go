// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// joinfiles.go — die Ablage des Bereichs "Join" sortiert sich selbst.
//
// Der Nutzer zieht Bild-, Ton- und Untertiteldateien in EINE Ablage; welche
// Datei was ist, entscheidet hier die Endung — nach denselben Regeln, die der
// Konverter in Streams.go (categorizeArgs) anwendet. Zwei getrennte Programme
// können sich keine Liste teilen; ändert er seine, gehört sie hier nachgezogen.
//
// Bewusst NICHT die Warteschlange aus queue.go: Die kennt elf Video-Endungen,
// als Bild-Grundlage nimmt der Konverter aber nur vier. Eine .ts-Datei würde
// dort anstandslos landen und der Lauf bräche erst beim Konverter ab.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Endungen, wie der Konverter sie beim Zusammenfügen einordnet.
var (
	joinVideoExtensions = []string{".mkv", ".mp4", ".m4v", ".mov"}

	joinAudioExtensions = []string{
		".m4a", ".aac", ".mp3", ".wav", ".ac3", ".eac3", ".ec3",
		".dts", ".flac", ".opus", ".ogg", ".mka", ".thd",
	}

	joinSubtitleExtensions = []string{".srt", ".sup", ".idx", ".ass", ".ssa", ".vtt"}
)

// Die Einordnungen, die die Oberfläche anzeigt. Als Konstanten, damit ein
// Tippfehler beim Vergleich auffällt, statt eine Gruppe leer zu lassen.
const (
	joinKindVideo     = "video"
	joinKindAudio     = "audio"
	joinKindSubtitle  = "subtitle"
	joinKindCompanion = "companion" // .sub, die zu einer abgelegten .idx gehört
	joinKindUnusable  = "unusable"
)

// JoinFile ist eine Zeile in der Join-Ablage.
type JoinFile struct {
	Path    string  `json:"path"`
	Name    string  `json:"name"`
	Folder  string  `json:"folder"`
	Kind    string  `json:"kind"`
	Note    string  `json:"note"` // nur gefüllt, wenn es etwas zu erklären gibt
	SizeMB  float64 `json:"sizeMB"`
	Missing bool    `json:"missing"`
}

// classifyJoinFiles ordnet die gesamte Ablage neu ein.
//
// Absichtlich zustandslos und immer über ALLE Pfade: Ob eine .sub verwendbar
// ist, hängt davon ab, ob die gleichnamige .idx ebenfalls in der Ablage liegt.
// Käme jede neu abgelegte Datei für sich, bliebe eine zuerst abgelegte .sub
// für immer als unbrauchbar stehen, obwohl die .idx später dazukam.
func classifyJoinFiles(paths []string) []JoinFile {
	files := expandJoinPaths(paths)
	idxStems := indexStems(files)

	result := make([]JoinFile, 0, len(files))
	for _, path := range files {
		kind, note := joinKindOf(path, idxStems)
		result = append(result, newJoinFile(path, kind, note))
	}

	sortJoinFiles(result)
	return result
}

// expandJoinPaths macht aus dem Abgelegten eine flache Dateiliste ohne
// Doppelte.
//
// Ordner werden nur EINE Ebene tief gelesen: Die Teile eines zerlegten Films
// liegen immer nebeneinander im selben Ordner. Ein rekursiver Durchlauf würde
// aus einem Filmarchiv hunderte Dateien einsammeln, von denen der Nutzer
// anschließend eine einzige Bild-Grundlage heraussuchen müsste.
func expandJoinPaths(paths []string) []string {
	var files []string
	seen := make(map[string]bool)

	add := func(path string) {
		absolute, err := filepath.Abs(path)
		if err != nil {
			absolute = path
		}
		key := strings.ToLower(absolute)
		if seen[key] {
			return
		}
		seen[key] = true
		files = append(files, absolute)
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			// Verschwundene Datei trotzdem aufnehmen: Sie wird unten als
			// fehlend markiert und ist damit sichtbar, statt spurlos zu
			// verschwinden.
			add(path)
			continue
		}
		if !info.IsDir() {
			add(path)
			continue
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			add(filepath.Join(path, entry.Name()))
		}
	}
	return files
}

// indexStems merkt sich die Namen aller abgelegten .idx-Dateien. Nur zu ihnen
// darf eine .sub gehören.
func indexStems(paths []string) map[string]bool {
	stems := make(map[string]bool)
	for _, path := range paths {
		if strings.ToLower(filepath.Ext(path)) != ".idx" {
			continue
		}
		name := strings.ToLower(filepath.Base(path))
		stems[strings.TrimSuffix(name, ".idx")] = true
	}
	return stems
}

// joinKindOf entscheidet über eine einzelne Datei.
//
// Die Reihenfolge der Fälle entspricht der des Konverters. Wichtig ist der
// .sub-Fall: Mit passender .idx wird sie vom Konverter still übersprungen —
// FFmpeg holt sie von selbst neben der .idx. Ohne .idx dagegen bricht der
// ganze Lauf ab ("Unknown file types"), deshalb muss sie hier auffallen.
func joinKindOf(path string, idxStems map[string]bool) (kind, note string) {
	extension := strings.ToLower(filepath.Ext(path))

	switch {
	case hasExtension(joinVideoExtensions, extension):
		return joinKindVideo, ""
	case hasExtension(joinSubtitleExtensions, extension):
		return joinKindSubtitle, ""
	case extension == ".sub":
		stem := strings.TrimSuffix(strings.ToLower(filepath.Base(path)), ".sub")
		if idxStems[stem] {
			return joinKindCompanion, "goes along with the .idx of the same name"
		}
		return joinKindUnusable, "a .sub only works together with its .idx file"
	case hasExtension(joinAudioExtensions, extension):
		return joinKindAudio, ""
	case extension == "":
		return joinKindUnusable, "no file extension — the converter sorts by extension only"
	default:
		return joinKindUnusable, extension + " is not a video, audio or subtitle file"
	}
}

// newJoinFile liest die Angaben, die in der Zeile stehen sollen.
func newJoinFile(path, kind, note string) JoinFile {
	file := JoinFile{
		Path:   path,
		Name:   filepath.Base(path),
		Folder: filepath.Dir(path),
		Kind:   kind,
		Note:   note,
	}
	info, err := os.Stat(path)
	if err != nil {
		file.Missing = true
		return file
	}
	file.SizeMB = float64(info.Size()) / 1024 / 1024
	return file
}

// sortJoinFiles bringt die Ablage in eine feste Ordnung: erst das Bild, dann
// Ton, dann Untertitel, zuletzt was nicht mitkommt. Innerhalb einer Gruppe
// nach Namen — so springt beim Hinzufügen nichts durcheinander.
func sortJoinFiles(files []JoinFile) {
	rank := map[string]int{
		joinKindVideo:     0,
		joinKindAudio:     1,
		joinKindSubtitle:  2,
		joinKindCompanion: 3,
		joinKindUnusable:  4,
	}
	sort.SliceStable(files, func(a, b int) bool {
		if rank[files[a].Kind] != rank[files[b].Kind] {
			return rank[files[a].Kind] < rank[files[b].Kind]
		}
		return strings.ToLower(files[a].Name) < strings.ToLower(files[b].Name)
	})
}

// hasExtension sagt, ob die Endung in der Liste steht.
func hasExtension(list []string, wanted string) bool {
	for _, entry := range list {
		if entry == wanted {
			return true
		}
	}
	return false
}

// joinArgOrder bringt die Dateien in die Reihenfolge, die der Konverter
// erwartet: erst die Bild-Grundlage, dann Ton, dann Untertitel.
//
// Die Prüfung ist die zweite Sicherung hinter der gesperrten Schaltfläche und
// nötig, weil der Konverter eine falsche Kombination NICHT als Fehler meldet:
// Gemessen am 2026-08-18 schickt er im -json-Kanal nur sein "run"-Ereignis und
// endet dann wortlos mit Rückgabewert 0 — das Fenster stünde da und wartete auf
// eine Zusammenfassung, die nie kommt. Lieber vorher eine klare Meldung.
func joinArgOrder(paths []string) ([]string, error) {
	var videos, audios, subtitles []string
	for _, file := range classifyJoinFiles(paths) {
		switch file.Kind {
		case joinKindVideo:
			videos = append(videos, file.Path)
		case joinKindAudio:
			audios = append(audios, file.Path)
		case joinKindSubtitle:
			subtitles = append(subtitles, file.Path)
		case joinKindCompanion:
			// Die .sub wird nicht übergeben: Der Konverter überspringt sie
			// ohnehin, FFmpeg liest sie neben ihrer .idx.
			continue
		default:
			return nil, fmt.Errorf("joinfiles.go: joinArgOrder: %s cannot be joined", file.Name)
		}
	}

	switch {
	case len(videos) == 0:
		return nil, errors.New("joinfiles.go: joinArgOrder: pick one video file to build on")
	case len(videos) > 1:
		return nil, errors.New("joinfiles.go: joinArgOrder: only one video file can be the base")
	case len(audios) == 0 && len(subtitles) == 0:
		return nil, errors.New("joinfiles.go: joinArgOrder: add at least one audio or subtitle file")
	}

	ordered := append([]string{}, videos...)
	ordered = append(ordered, audios...)
	return append(ordered, subtitles...), nil
}

// joinFilterPatterns baut die Muster für den Dateidialog.
func joinFilterPatterns() (video, audio, subtitle string) {
	pattern := func(extensions []string) string {
		parts := make([]string, 0, len(extensions))
		for _, extension := range extensions {
			parts = append(parts, "*"+extension)
		}
		return strings.Join(parts, ";")
	}
	// Die .sub gehört in den Dialog, obwohl sie nie allein mitgeht: Wer sie
	// nicht auswählen kann, kann sie auch nicht neben ihre .idx legen.
	return pattern(joinVideoExtensions),
		pattern(joinAudioExtensions),
		pattern(append(append([]string{}, joinSubtitleExtensions...), ".sub"))
}
