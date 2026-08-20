// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// storeInTemp baut einen Profilspeicher auf eine Datei, die es noch nicht gibt.
func storeInTemp(t *testing.T) *profileStore {
	t.Helper()
	return &profileStore{path: filepath.Join(t.TempDir(), profilesFileName)}
}

func TestSaveAndReadBack(t *testing.T) {
	store := storeInTemp(t)
	saved, err := store.Save(Profile{Name: "  Serien  ", Codec: "av1", Quality: "fixed", FixedCQ: 30, Parallel: 2})
	if err != nil {
		t.Fatalf("Speichern fehlgeschlagen: %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("erwartet 1 Profil, bekommen %d", len(saved))
	}
	// Leerzeichen am Rand kommen beim Tippen mit und wären im Auswahlfeld nicht
	// zu sehen — zwei Profile, die gleich aussehen, wären die Folge.
	if saved[0].Name != "Serien" {
		t.Errorf("Name nicht beschnitten: %q", saved[0].Name)
	}

	// Ein neu gestartetes Fenster muss dasselbe lesen.
	reopened := &profileStore{path: store.path, profiles: loadProfiles(store.path)}
	again := reopened.List()
	if len(again) != 1 || again[0].Codec != "av1" || again[0].FixedCQ != 30 {
		t.Errorf("nach dem Neulesen anders: %+v", again)
	}
}

func TestSaveReplacesTheSameName(t *testing.T) {
	store := storeInTemp(t)
	if _, err := store.Save(Profile{Name: "Serien", Codec: "av1"}); err != nil {
		t.Fatalf("erstes Speichern: %v", err)
	}
	// Anderer Groß-/Kleinbuchstabe, gleiches Profil: Zwei Einträge wären im
	// Auswahlfeld nicht auseinanderzuhalten.
	list, err := store.Save(Profile{Name: "serien", Codec: ""})
	if err != nil {
		t.Fatalf("zweites Speichern: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("erwartet 1 Profil, bekommen %d", len(list))
	}
	if list[0].Codec != "" {
		t.Errorf("der neue Wert hat den alten nicht ersetzt: %+v", list[0])
	}
}

func TestSaveNeedsAName(t *testing.T) {
	store := storeInTemp(t)
	if _, err := store.Save(Profile{Name: "   "}); err == nil {
		t.Error("ein Profil ohne Namen wurde angenommen")
	}
	if _, err := os.Stat(store.path); !os.IsNotExist(err) {
		t.Error("es wurde trotzdem eine Datei geschrieben")
	}
}

func TestDeleteRemovesTheFileWhenNothingIsLeft(t *testing.T) {
	store := storeInTemp(t)
	if _, err := store.Save(Profile{Name: "Serien"}); err != nil {
		t.Fatalf("Speichern: %v", err)
	}
	if _, err := store.Delete("Serien"); err != nil {
		t.Fatalf("Löschen: %v", err)
	}
	if _, err := os.Stat(store.path); !os.IsNotExist(err) {
		t.Error("die leere Profildatei liegt noch neben der exe")
	}
	if _, err := store.Delete("Serien"); err == nil {
		t.Error("das Löschen eines unbekannten Profils wurde still hingenommen")
	}
}

func TestListIsSortedByName(t *testing.T) {
	store := storeInTemp(t)
	for _, name := range []string{"zuletzt", "Anfang", "mitte"} {
		if _, err := store.Save(Profile{Name: name}); err != nil {
			t.Fatalf("Speichern von %q: %v", name, err)
		}
	}
	list := store.List()
	want := []string{"Anfang", "mitte", "zuletzt"}
	for index, name := range want {
		if list[index].Name != name {
			t.Errorf("Platz %d: %q, erwartet %q", index, list[index].Name, name)
		}
	}
}

func TestListIsCapped(t *testing.T) {
	store := storeInTemp(t)
	for index := 0; index < maxProfiles; index++ {
		if _, err := store.Save(Profile{Name: string(rune('a'+index%26)) + strings.Repeat("x", index%5+1)}); err != nil {
			t.Fatalf("Speichern Nr. %d: %v", index, err)
		}
	}
	if _, err := store.Save(Profile{Name: "einer zu viel"}); err == nil {
		t.Error("über die Höchstzahl hinaus wurde gespeichert")
	}
}

// Eine von Hand verdorbene Datei darf das Fenster nicht aufhalten und keine
// unsinnigen Vorgaben in die Bedienelemente schreiben.
func TestLoadSurvivesADamagedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, profilesFileName)

	if err := os.WriteFile(path, []byte("kein JSON"), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	if got := loadProfiles(path); got != nil {
		t.Errorf("aus Unsinn wurden Profile: %+v", got)
	}

	broken := `[{"name":"","codec":"av1"},
	            {"name":"gut","fixedCQ":900,"maxBitrate":-5,"parallel":9},
	            {"name":"GUT","codec":"doppelt"}]`
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatalf("schreiben: %v", err)
	}
	list := loadProfiles(path)
	if len(list) != 1 {
		t.Fatalf("erwartet 1 brauchbares Profil, bekommen %d: %+v", len(list), list)
	}
	got := list[0]
	if got.FixedCQ != maxProfileCQ || got.MaxBitrate != 0 || got.Parallel != maxProfileRuns {
		t.Errorf("Zahlen nicht in die Grenzen geholt: %+v", got)
	}
}

// Ohne Ablageort darf nicht so getan werden, als wäre etwas gespeichert.
func TestSaveWithoutAPathReportsIt(t *testing.T) {
	store := &profileStore{}
	if _, err := store.Save(Profile{Name: "Serien"}); err == nil {
		t.Error("ohne Pfad wurde stillschweigend Erfolg gemeldet")
	}
}
