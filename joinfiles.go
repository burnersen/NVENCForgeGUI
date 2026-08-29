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
	"strconv"
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
	Path   string `json:"path"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
	Kind   string `json:"kind"`
	Note   string `json:"note"` // nur gefüllt, wenn es etwas zu erklären gibt

	// Group ist der Namensstamm der Bild-Grundlage, zu der diese Datei gehört
	// — leer, wenn keine passt. Aus ihr entsteht je ein eigener Auftrag, damit
	// ein ganzer Stapel zerlegter Filme in einem Rutsch wieder zusammengesetzt
	// werden kann, statt einer nach dem anderen von Hand.
	Group string `json:"group"`

	// Ready sagt, ob der Auftrag dieser Datei wirklich losläuft: ein Bild,
	// mindestens eine Beigabe, keine verschwundene Datei. Die Auskunft kommt
	// von hier und nicht aus dem Fenster, damit Anzeige und Argumentbau
	// dieselbe Regel benutzen und nicht auseinanderlaufen.
	Ready bool `json:"ready"`

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

	assignJoinGroups(result)
	markRunnableJoinGroups(result)
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

// sortJoinFiles bringt die Ablage in eine feste Ordnung: die Aufträge nach
// ihrem Namensstamm, innerhalb eines Auftrags erst das Bild, dann Ton, dann
// Untertitel, zuletzt was nicht mitkommt. Was zu keiner Bild-Grundlage passt,
// steht ganz unten — dort stört es die Aufträge nicht und bleibt trotzdem
// sichtbar.
//
// Nach Auftrag statt nach Art, weil ein Stapel sonst unlesbar wäre: Bei zehn
// zerlegten Filmen stünden erst zehn Bilddateien, dann dreißig Tonspuren, und
// welche zu welchem Film gehört, müsste der Nutzer am Namen abzählen.
func sortJoinFiles(files []JoinFile) {
	rank := map[string]int{
		joinKindVideo:     0,
		joinKindAudio:     1,
		joinKindSubtitle:  2,
		joinKindCompanion: 3,
		joinKindUnusable:  4,
	}
	sort.SliceStable(files, func(a, b int) bool {
		first, second := files[a], files[b]
		if (first.Group == "") != (second.Group == "") {
			return second.Group == ""
		}
		if !strings.EqualFold(first.Group, second.Group) {
			return strings.ToLower(first.Group) < strings.ToLower(second.Group)
		}
		if rank[first.Kind] != rank[second.Kind] {
			return rank[first.Kind] < rank[second.Kind]
		}
		return strings.ToLower(first.Name) < strings.ToLower(second.Name)
	})
}

// ----------------------------------------------------------------------------
// Die Zuordnung: welche Teile gehören zu welchem Film
//
// Das Zerlegen schreibt seine Ergebnisse nach einem festen Muster neben die
// Quelle: "Film.Name.NoSound.mkv", "Film.Name.ger.eac3", "Film.Name.ger.srt".
// Der gemeinsame Stamm "Film.Name" ist damit die einzige Angabe, die man
// braucht, um die Teile wieder zusammenzufinden — genau das macht dieser
// Abschnitt, damit ein ganzer Stapel in einem Rutsch zurückgebaut werden kann.
// ----------------------------------------------------------------------------

// joinStemSuffixes sind die Namensteile, die der Konverter beim Zerlegen selbst
// abschneidet (trimToolSuffixes in seiner Streams.go). Sie gehören nicht zum
// Namen des Films: "Film.h265.mkv" wird zu "Film.NoSound.mkv" plus
// "Film.ger.eac3". Ändert er seine Liste, gehört sie hier nachgezogen — zwei
// getrennte Programme können sich keine teilen.
var joinStemSuffixes = map[string]bool{
	"sub": true, "subbed": true, "h265": true, "h264": true, "av1": true,
	"remux": true, "video": true, "nosound": true, "joined": true, "preview": true,
}

// joinStem lässt vom Dateinamen den Stamm stehen, an dem die Teile eines Films
// einander erkennen.
//
// Nur von hinten und nur ganze Punktabschnitte: Ein Film, der wirklich
// "Preview" heißt, verlöre sonst mitten im Namen ein Wort.
func joinStem(fileName string) string {
	stem := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	for {
		dot := strings.LastIndex(stem, ".")
		if dot <= 0 {
			return stem
		}
		token := strings.ToLower(stem[dot+1:])
		if !joinStemSuffixes[token] && !isNumberedSubSuffix(token) {
			return stem
		}
		stem = stem[:dot]
	}
}

// isNumberedSubSuffix erkennt die durchnummerierten Untertitel-Endungen
// (".sub2", ".sub3"), die der Konverter vergibt, wenn ein Name schon belegt war.
func isNumberedSubSuffix(token string) bool {
	rest, found := strings.CutPrefix(token, "sub")
	if !found || rest == "" {
		return false
	}
	_, err := strconv.Atoi(rest)
	return err == nil
}

// isSilentSplitVideo erkennt das stumme Bild aus dem Zerlegen.
func isSilentSplitVideo(fileName string) bool {
	name := strings.ToLower(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	return strings.HasSuffix(name, ".nosound")
}

// belongsToStem sagt, ob eine Datei zu einem Stamm gehört: entweder heißt sie
// genau so, oder der Stamm steht davor und danach folgt ein Punkt und das, was
// das Zerlegen angehängt hat (Sprache, "forced", "sdh", eine laufende Nummer).
//
// Der Punkt ist die ganze Sicherung: Ohne ihn zöge der Stamm "Film" auch
// "Filmmusik.ger.mp3" an sich.
func belongsToStem(fileName, stem string) bool {
	name := strings.ToLower(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	wanted := strings.ToLower(stem)
	return name == wanted || strings.HasPrefix(name, wanted+".")
}

// joinBase ist eine gefundene Bild-Grundlage.
type joinBase struct {
	folder string
	stem   string
}

// assignJoinGroups ordnet jede abgelegte Datei einer Bild-Grundlage zu.
//
// Zwei Durchgänge, weil erst die Videodateien die Gruppen aufspannen: Zuerst
// bekommt jedes Video seinen Stamm, dann sucht sich jede Beigabe das Video,
// dessen Stamm am längsten auf ihren Namen passt. Der längste gewinnt, damit
// "Film.2.ger.srt" bei "Film.2" landet und nicht bei "Film".
//
// Die Zuordnung endet am Ordner: Zwei gleichnamige Folgen in getrennten Ordnern
// sind zwei Filme, und eine Tonspur von nebenan wäre geraten, nicht erkannt.
func assignJoinGroups(files []JoinFile) {
	bases := make([]joinBase, 0, len(files))
	taken := make(map[string]int) // Ordner+Stamm -> Zeile der Bild-Grundlage

	for i := range files {
		if files[i].Kind != joinKindVideo {
			continue
		}
		stem := joinStem(files[i].Name)
		key := joinGroupKey(files[i].Folder, stem)
		previous, exists := taken[key]
		if !exists {
			taken[key] = i
			bases = append(bases, joinBase{folder: files[i].Folder, stem: stem})
			files[i].Group = stem
			continue
		}
		// Zwei Videos mit demselben Stamm: Eines ist die Grundlage, das andere
		// kann es nicht zusätzlich sein. Das stumme Bild aus dem Zerlegen hat
		// Vorrang — genau darauf soll der Ton zurück.
		loser := i
		if isSilentSplitVideo(files[i].Name) && !isSilentSplitVideo(files[previous].Name) {
			loser = previous
			taken[key] = i
			files[i].Group = stem
		}
		files[loser].Group = ""
		files[loser].Kind = joinKindUnusable
		files[loser].Note = "another video of the same name is the base here"
	}

	for i := range files {
		if files[i].Kind == joinKindVideo || files[i].Kind == joinKindUnusable {
			continue
		}
		files[i].Group = bestJoinGroup(files[i], bases)
		if files[i].Group == "" && files[i].Note == "" {
			files[i].Note = "no video of this name in the list"
		}
	}
}

// bestJoinGroup sucht den längsten Stamm im selben Ordner, der auf die Datei
// passt.
func bestJoinGroup(file JoinFile, bases []joinBase) string {
	best := ""
	for _, candidate := range bases {
		if !strings.EqualFold(candidate.folder, file.Folder) {
			continue
		}
		if !belongsToStem(file.Name, candidate.stem) {
			continue
		}
		if len(candidate.stem) > len(best) {
			best = candidate.stem
		}
	}
	return best
}

// joinGroupKey ist der Schlüssel einer Gruppe: Ordner plus Stamm, beides klein
// geschrieben. Der senkrechte Strich kann in keinem der beiden vorkommen und
// hält sie deshalb sauber auseinander.
func joinGroupKey(folder, stem string) string {
	return strings.ToLower(folder) + "|" + strings.ToLower(stem)
}

// markRunnableJoinGroups setzt Ready und erklärt, was einer Gruppe fehlt.
//
// Eine Gruppe ist vollständig, wenn sie ein Bild und mindestens eine Ton- oder
// Untertiteldatei enthält. Bewusst NUR an den Namen entschieden und ohne Blick
// auf die Festplatte: Ob eine Datei gerade da ist, steht als Missing in jeder
// Zeile und wird dort behandelt, wo es hingehört — beim Start im Fenster. So
// bleibt die Zuordnung eine reine Namensfrage und liefert immer dasselbe
// Ergebnis, egal wann sie gestellt wird.
func markRunnableJoinGroups(files []JoinFile) {
	hasVideo := make(map[string]bool)
	hasExtra := make(map[string]bool)

	for _, file := range files {
		if file.Group == "" {
			continue
		}
		key := joinGroupKey(file.Folder, file.Group)
		switch file.Kind {
		case joinKindVideo:
			hasVideo[key] = true
		case joinKindAudio, joinKindSubtitle:
			hasExtra[key] = true
		}
	}

	for i := range files {
		if files[i].Group == "" {
			continue
		}
		key := joinGroupKey(files[i].Folder, files[i].Group)
		files[i].Ready = hasVideo[key] && hasExtra[key]
		if files[i].Ready || files[i].Note != "" {
			continue
		}
		if hasVideo[key] {
			files[i].Note = "no audio or subtitle for this video yet"
		} else {
			files[i].Note = "no video of this name in the list"
		}
	}
}

// joinGroupPaths teilt die Ablage in die einzelnen Zusammenfüge-Aufträge auf:
// je Bild-Grundlage einen, mit allen Dateien, die zu ihr gehören.
//
// Was nicht losläuft, fällt still weg — es steht in der Liste des Fensters
// bereits mit dem Grund daneben, und ein Stapel darf nicht an einer einzelnen
// unvollständigen Gruppe scheitern. Bleibt gar nichts übrig, ist das ein
// Fehler: Dann hätte der Lauf nichts zu tun.
func joinGroupPaths(paths []string) ([][]string, error) {
	files := classifyJoinFiles(paths)

	order := make([]string, 0, len(files))
	members := make(map[string][]string)
	for _, file := range files {
		if !file.Ready {
			continue
		}
		key := joinGroupKey(file.Folder, file.Group)
		if _, known := members[key]; !known {
			order = append(order, key)
		}
		members[key] = append(members[key], file.Path)
	}

	if len(order) == 0 {
		return nil, errors.New("joinfiles.go: joinGroupPaths: nothing complete to join — every job needs one video plus at least one audio or subtitle file")
	}

	groups := make([][]string, 0, len(order))
	for _, key := range order {
		groups = append(groups, members[key])
	}
	return groups, nil
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
