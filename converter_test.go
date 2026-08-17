package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileContainsMarkerFindsAndMisses(t *testing.T) {
	dir := t.TempDir()
	hit := filepath.Join(dir, "hit.bin")
	miss := filepath.Join(dir, "miss.bin")

	if err := os.WriteFile(hit, []byte("nonsense-json-nonsense"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(miss, []byte("nothing to see here"), 0o644); err != nil {
		t.Fatal(err)
	}

	if found, err := fileContainsMarker(hit, jsonFlagMarker); err != nil || !found {
		t.Errorf("marker should be found: found=%v err=%v", found, err)
	}
	if found, err := fileContainsMarker(miss, jsonFlagMarker); err != nil || found {
		t.Errorf("marker should be missing: found=%v err=%v", found, err)
	}
}

func TestFileContainsMarkerAcrossBlockBoundary(t *testing.T) {
	// Der eigentliche Grund für den Übertrag im Puffer: Liegt der Treffer genau
	// auf der Grenze zwischen zwei gelesenen Blöcken, würde er ohne Übertrag
	// übersehen — und die Oberfläche meldete fälschlich "kein Datenkanal".
	dir := t.TempDir()
	path := filepath.Join(dir, "boundary.bin")

	const blockSize = 1 << 20
	marker := []byte(jsonFlagMarker)
	content := bytes.Repeat([]byte("x"), blockSize-len(marker)/2)
	content = append(content, marker...)
	content = append(content, bytes.Repeat([]byte("x"), 4096)...)

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := fileContainsMarker(path, jsonFlagMarker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("a marker on the block boundary must still be found")
	}
}

func TestPickConverterAsset(t *testing.T) {
	release := releaseInfo{
		TagName: "v1.16.0",
		Assets: []releaseAsset{
			{Name: "embedded_source.zip"},
			{Name: "NVENCForge.exe", URL: "https://example.invalid/x"},
		},
	}
	asset, err := pickConverterAsset(release)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if asset.Name != converterExeName {
		t.Errorf("wrong asset picked: %s", asset.Name)
	}

	if _, err := pickConverterAsset(releaseInfo{TagName: "v1", Assets: nil}); err == nil {
		t.Error("a release without the exe must be reported as an error")
	}
}

func TestReleaseJSONIsParsedAsGitHubSendsIt(t *testing.T) {
	raw := `{"tag_name":"v1.16.0","assets":[{"name":"NVENCForge.exe","size":8857088,` +
		`"browser_download_url":"https://example.invalid/NVENCForge.exe"}]}`
	var release releaseInfo
	if err := json.Unmarshal([]byte(raw), &release); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.TagName != "v1.16.0" || len(release.Assets) != 1 {
		t.Fatalf("release not read correctly: %+v", release)
	}
	if release.Assets[0].Size != 8857088 || release.Assets[0].URL == "" {
		t.Errorf("asset fields not read correctly: %+v", release.Assets[0])
	}
}

func TestParseGPUQuery(t *testing.T) {
	info := parseGPUQuery("NVIDIA GeForce RTX 5070 Ti, 610.74, 16303\n")
	if !info.Detected {
		t.Fatal("a GPU line must count as detected")
	}
	if info.Name != "NVIDIA GeForce RTX 5070 Ti" || info.Driver != "610.74" || info.MemoryMB != 16303 {
		t.Errorf("parsed wrongly: %+v", info)
	}

	empty := parseGPUQuery("\n\n")
	if empty.Detected || empty.Note == "" {
		t.Errorf("an empty answer must be reported honestly: %+v", empty)
	}
}
