// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// converter.go — die mitgebrachte NVENCForge.exe finden, prüfen und notfalls
// von GitHub laden.
//
// Die Kette ist bewusst so gebaut: Der Nutzer lädt nur dieses Fenster, das
// Fenster holt sich NVENCForge.exe, und NVENCForge.exe holt sich beim ersten
// Start selbst FFmpeg. Jede Stufe kümmert sich um genau das, was sie kennt.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	converterExeName = "NVENCForge.exe"
	toolsDirName     = "tools"

	// versionFileName merkt sich, welche Ausgabe geladen wurde. Die exe selbst
	// nach ihrer Version zu fragen hieße, sie zu starten — dafür ist eine reine
	// Anzeige kein guter Grund.
	versionFileName = "NVENCForge.version"

	// latestReleaseURL liefert die zuletzt veröffentlichte Ausgabe.
	latestReleaseURL = "https://api.github.com/repos/burnersen/NVENCForge/releases/latest"

	// jsonFlagMarker steht als Zeichenkette in jeder exe, die den Datenkanal
	// kennt (consumeJSONFlag vergleicht genau darauf). Danach in der Datei zu
	// suchen ist der billigste ehrliche Weg, die Fähigkeit festzustellen, ohne
	// das Programm zu starten.
	jsonFlagMarker = "-json"

	downloadTimeout = 10 * time.Minute
	apiTimeout      = 30 * time.Second
)

// ConverterStatus ist alles, was die Oberfläche über die Programmdatei weiß.
type ConverterStatus struct {
	Found         bool   `json:"found"`
	Path          string `json:"path"`
	SizeBytes     int64  `json:"sizeBytes"`
	Version       string `json:"version"`
	EventChannel  bool   `json:"eventChannel"`
	ToolsDir      string `json:"toolsDir"`
	FFmpegPresent bool   `json:"ffmpegPresent"`
	Note          string `json:"note"`
}

// toolsDirCandidates nennt die Orte, an denen die Programmdatei liegen kann.
//
// Zuerst neben der eigenen exe — so ist das fertige Programm aufgebaut. Danach
// im Arbeitsverzeichnis, denn beim Entwickeln mit "wails dev" läuft das Fenster
// aus einem Bau-Ordner, der bei jedem Start neu entsteht.
func toolsDirCandidates() []string {
	var dirs []string
	add := func(dir string) {
		for _, existing := range dirs {
			if strings.EqualFold(existing, dir) {
				return
			}
		}
		dirs = append(dirs, dir)
	}
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), toolsDirName))
	}
	if wd, err := os.Getwd(); err == nil {
		add(filepath.Join(wd, toolsDirName))
	}
	return dirs
}

// locateConverter sucht die Programmdatei an allen bekannten Orten.
func locateConverter() (string, bool) {
	for _, dir := range toolsDirCandidates() {
		candidate := filepath.Join(dir, converterExeName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

// downloadDir bestimmt, wohin eine geladene Programmdatei gehört: in einen
// bereits vorhandenen tools-Ordner, sonst in den ersten möglichen.
func downloadDir() (string, error) {
	candidates := toolsDirCandidates()
	if len(candidates) == 0 {
		return "", fmt.Errorf("converter.go: downloadDir: no writable location found")
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return candidates[0], nil
}

// converterStatus stellt zusammen, was die Oberfläche anzeigen soll.
func converterStatus() ConverterStatus {
	dir, _ := downloadDir()
	status := ConverterStatus{ToolsDir: dir}

	path, found := locateConverter()
	if !found {
		status.Note = "NVENCForge.exe is not here yet — it can be downloaded from GitHub."
		return status
	}

	status.Found = true
	status.Path = path
	status.ToolsDir = filepath.Dir(path)
	if info, err := os.Stat(path); err == nil {
		status.SizeBytes = info.Size()
	}
	status.Version = readVersionFile(filepath.Dir(path))
	status.FFmpegPresent = fileExists(filepath.Join(filepath.Dir(path), "ffmpeg.exe"))

	hasChannel, err := fileContainsMarker(path, jsonFlagMarker)
	switch {
	case err != nil:
		status.Note = "Could not read NVENCForge.exe: " + err.Error()
	case hasChannel:
		status.EventChannel = true
	default:
		status.Note = "This NVENCForge.exe has no event channel (-json). " +
			"The run still works, but progress can only be read from the log."
	}
	return status
}

// readVersionFile liefert die gemerkte Ausgabe oder einen ehrlichen Ersatz.
func readVersionFile(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, versionFileName))
	if err != nil {
		return "local build"
	}
	version := strings.TrimSpace(string(raw))
	if version == "" {
		return "local build"
	}
	return version
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// fileContainsMarker sucht eine Zeichenkette in einer Datei, ohne sie komplett
// in den Speicher zu holen.
//
// Der Übertrag am Pufferende ist der Grund für den Umweg: Ohne ihn würde ein
// Treffer übersehen, der genau auf einer Blockgrenze liegt.
func fileContainsMarker(path, marker string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("converter.go: fileContainsMarker (Open): %w", err)
	}
	defer file.Close()

	needle := []byte(marker)
	overlap := len(needle) - 1
	const blockSize = 1 << 20

	buffer := make([]byte, blockSize+overlap)
	carried := 0
	for {
		read, readErr := file.Read(buffer[carried:])
		if read > 0 {
			window := buffer[:carried+read]
			if bytes.Contains(window, needle) {
				return true, nil
			}
			carried = overlap
			if carried > len(window) {
				carried = len(window)
			}
			copy(buffer, window[len(window)-carried:])
		}
		if readErr == io.EOF {
			return false, nil
		}
		if readErr != nil {
			return false, fmt.Errorf("converter.go: fileContainsMarker (Read): %w", readErr)
		}
	}
}

// ----------------------------------------------------------------------------
// Herunterladen von GitHub
// ----------------------------------------------------------------------------

type releaseAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// fetchLatestRelease fragt GitHub nach der neuesten Veröffentlichung.
func fetchLatestRelease(ctx context.Context) (releaseInfo, error) {
	var release releaseInfo

	requestCtx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, latestReleaseURL, nil)
	if err != nil {
		return release, fmt.Errorf("converter.go: fetchLatestRelease (NewRequest): %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "NVENCForgeGUI")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return release, fmt.Errorf("converter.go: fetchLatestRelease (Do): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return release, fmt.Errorf("converter.go: fetchLatestRelease: GitHub answered %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, fmt.Errorf("converter.go: fetchLatestRelease (Decode): %w", err)
	}
	return release, nil
}

// pickConverterAsset sucht die Programmdatei unter den angehängten Dateien.
func pickConverterAsset(release releaseInfo) (releaseAsset, error) {
	for _, asset := range release.Assets {
		if strings.EqualFold(asset.Name, converterExeName) {
			return asset, nil
		}
	}
	return releaseAsset{}, fmt.Errorf(
		"converter.go: pickConverterAsset: release %s has no %s attached", release.TagName, converterExeName)
}

// progressWriter meldet den Fortschritt weiter, während geschrieben wird.
type progressWriter struct {
	total    int64
	done     int64
	lastSent time.Time
	report   func(done, total int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.done += int64(len(p))
	// Gedrosselt, sonst erzeugt ein 9-MB-Download tausende Meldungen an die
	// Oberfläche — dieselbe Überlegung wie beim Fortschritt des Konverters.
	if w.report != nil && time.Since(w.lastSent) >= 100*time.Millisecond {
		w.lastSent = time.Now()
		w.report(w.done, w.total)
	}
	return len(p), nil
}

// DownloadResult sagt der Oberfläche, was der Download bewirkt hat.
type DownloadResult struct {
	Status   ConverterStatus `json:"status"`
	Replaced bool            `json:"replaced"`
	Tag      string          `json:"tag"`
	Message  string          `json:"message"`
}

// downloadConverter lädt die neueste NVENCForge.exe in den tools-Ordner.
//
// Geschrieben wird zuerst in eine Teildatei und erst nach vollständigem
// Download umbenannt. Sonst bliebe nach einem Abbruch eine halbe exe liegen,
// die beim nächsten Start als vorhanden gälte.
//
// force=false schützt vor einem Rückschritt: Kann die vorhandene Programmdatei
// den Datenkanal und die geladene nicht, wird NICHT ersetzt. Sonst würde ein
// Klick auf "update" die Fortschrittsanzeige stillschweigend abschalten —
// genau das ist am 2026-08-17 passiert.
func downloadConverter(ctx context.Context, force bool, report func(done, total int64)) (DownloadResult, error) {
	before := converterStatus()
	result := DownloadResult{Status: before}

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return result, err
	}
	result.Tag = release.TagName

	asset, err := pickConverterAsset(release)
	if err != nil {
		return result, err
	}

	dir, err := downloadDir()
	if err != nil {
		return result, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result, fmt.Errorf("converter.go: downloadConverter (MkdirAll): %w", err)
	}

	targetPath := filepath.Join(dir, converterExeName)
	partPath := targetPath + ".part"

	downloadCtx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return result, fmt.Errorf("converter.go: downloadConverter (NewRequest): %w", err)
	}
	req.Header.Set("User-Agent", "NVENCForgeGUI")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("converter.go: downloadConverter (Do): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf(
			"converter.go: downloadConverter: download answered %s", resp.Status)
	}

	part, err := os.Create(partPath)
	if err != nil {
		return result, fmt.Errorf("converter.go: downloadConverter (Create): %w", err)
	}

	counter := &progressWriter{total: asset.Size, report: report}
	_, copyErr := io.Copy(io.MultiWriter(part, counter), resp.Body)
	closeErr := part.Close()

	if copyErr != nil {
		_ = os.Remove(partPath)
		return result, fmt.Errorf("converter.go: downloadConverter (Copy): %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(partPath)
		return result, fmt.Errorf("converter.go: downloadConverter (Close): %w", closeErr)
	}

	if !force && wouldLoseEventChannel(before, partPath) {
		_ = os.Remove(partPath)
		result.Message = "Release " + release.TagName + " has no event channel (-json), " +
			"but the converter installed here has one. Nothing was replaced — " +
			"the progress display would have stopped working."
		return result, nil
	}

	// Eine laufende alte exe kann nicht überschrieben werden; die vorher zu
	// entfernen ist der verlässlichere Weg als sich auf Rename zu verlassen.
	_ = os.Remove(targetPath)
	if err := os.Rename(partPath, targetPath); err != nil {
		return result, fmt.Errorf("converter.go: downloadConverter (Rename): %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, versionFileName), []byte(release.TagName), 0o644); err != nil {
		// Die Programmdatei liegt richtig — nur die Versionsnotiz fehlt. Das ist
		// kein Grund, den Download als gescheitert zu melden.
		_ = err
	}

	if report != nil {
		report(asset.Size, asset.Size)
	}
	result.Replaced = true
	result.Status = converterStatus()
	result.Message = "NVENCForge " + release.TagName + " installed."
	return result, nil
}

// wouldLoseEventChannel prüft, ob das Einspielen ein Rückschritt wäre.
//
// Lässt sich die geladene Datei nicht lesen, gilt sie als "kein Rückschritt" —
// die Entscheidung darüber trifft dann die Prüfung nach dem Einspielen, und
// die Oberfläche sagt es ohnehin an.
func wouldLoseEventChannel(before ConverterStatus, downloadedPath string) bool {
	if !before.Found || !before.EventChannel {
		return false
	}
	hasChannel, err := fileContainsMarker(downloadedPath, jsonFlagMarker)
	if err != nil {
		return false
	}
	return !hasChannel
}
