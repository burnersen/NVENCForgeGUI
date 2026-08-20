// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// profiles.go — gespeicherte Optionssätze ("Profile").
//
// Wer immer dasselbe einstellt, soll es einmal einstellen. Ein Profil merkt
// sich die Einstellungen der Konvertieren-Seite unter einem Namen; die Seite
// mit dem beobachteten Ordner kann dieselben Profile laden.
//
// Bewusst NICHT im Profil: die abzuarbeitenden Dateien (die gehören zum Lauf,
// nicht zur Einstellung) und "Rechner danach ausschalten". Ein Schalter, der
// den Rechner herunterfährt, darf sich nicht unbemerkt über einen Profilnamen
// wieder einschalten — das ist eine Entscheidung für genau einen Lauf.
//
// Abgelegt wird neben der exe, wie der Fensterplatz und das Sparbuch. Das hält
// die Zusage aus dem README: nichts wird installiert, nichts in die
// Registrierung geschrieben — Ordner löschen und es ist weg.
//
// Anders als beim Sparbuch wird ein Schreibfehler hier NICHT verschluckt: Wer
// "Speichern" drückt, muss erfahren, wenn nichts gespeichert wurde.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	// profilesFileName ist die Profildatei neben der exe.
	profilesFileName = "NVENCForgeGUI.profiles"

	// maxProfiles hält die Liste bedienbar — und eine beschädigte Datei davon
	// ab, das Auswahlfeld unendlich lang zu machen.
	maxProfiles = 50

	// maxProfileNameLength: Der Name steht in einem Auswahlfeld. Was dort nicht
	// mehr lesbar ist, hilft niemandem beim Wiederfinden.
	maxProfileNameLength = 40

	// Grenzen für Zahlen aus der Datei. Sie fangen einen von Hand verdorbenen
	// Eintrag ab, bevor er als unsinnige Vorgabe im Fenster steht.
	maxProfileCQ      = 63
	maxProfileBitrate = 200000
	maxProfileRuns    = 3
)

// Profile ist ein Satz Einstellungen unter einem Namen. Die Feldnamen sind
// dieselben wie in RunRequest — die Fensterseite kann sie eins zu eins in ihre
// Bedienelemente schreiben, ohne eine zweite Übersetzungstabelle zu pflegen.
type Profile struct {
	Name       string `json:"name"`
	Codec      string `json:"codec"`
	Encoder    string `json:"encoder"`
	Container  string `json:"container"`
	Resolution string `json:"resolution"`
	Audio      string `json:"audio"`
	BitDepth   string `json:"bitDepth"`
	Quality    string `json:"quality"`
	FixedCQ    int    `json:"fixedCQ"`
	MaxBitrate int    `json:"maxBitrate"`
	KeepSource bool   `json:"keepSource"`
	Parallel   int    `json:"parallel"`
}

// profileStore hält die Profile und die Datei, in der sie stehen.
type profileStore struct {
	mu       sync.Mutex
	path     string
	profiles []Profile
}

// newProfileStore liest die vorhandenen Profile ein. Fehlt die Datei, fängt es
// mit einer leeren Liste an — das ist der Normalfall beim ersten Start.
func newProfileStore() *profileStore {
	store := &profileStore{}
	path, err := profilesPath()
	if err != nil {
		// Ohne Pfad bleibt es bei einer Liste im Speicher. Speichern meldet
		// dann einen Fehler, statt so zu tun, als wäre etwas abgelegt.
		return store
	}
	store.path = path
	store.profiles = loadProfiles(path)
	return store
}

func profilesPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("profiles.go: profilesPath: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), profilesFileName), nil
}

// loadProfiles liest die Datei und wirft weg, was unbrauchbar ist.
//
// Eine beschädigte Datei darf das Fenster nicht aufhalten: Profile sind
// Bequemlichkeit, kein Programmteil, ohne den nichts geht.
func loadProfiles(path string) []Profile {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var stored []Profile
	if json.Unmarshal(raw, &stored) != nil {
		return nil
	}
	cleaned := make([]Profile, 0, len(stored))
	seen := map[string]bool{}
	for _, profile := range stored {
		profile = sanitiseProfile(profile)
		if profile.Name == "" || seen[strings.ToLower(profile.Name)] {
			continue
		}
		seen[strings.ToLower(profile.Name)] = true
		cleaned = append(cleaned, profile)
		if len(cleaned) == maxProfiles {
			break
		}
	}
	sortProfiles(cleaned)
	return cleaned
}

// sanitiseProfile schneidet zurecht, was aus der Datei kommt.
func sanitiseProfile(profile Profile) Profile {
	profile.Name = strings.TrimSpace(profile.Name)
	if len(profile.Name) > maxProfileNameLength {
		profile.Name = strings.TrimSpace(profile.Name[:maxProfileNameLength])
	}
	profile.FixedCQ = clampInt(profile.FixedCQ, 0, maxProfileCQ)
	profile.MaxBitrate = clampInt(profile.MaxBitrate, 0, maxProfileBitrate)
	// 0 heißt hier nicht "null Läufe", sondern "stand nicht in der Datei".
	if profile.Parallel == 0 {
		profile.Parallel = 1
	}
	profile.Parallel = clampInt(profile.Parallel, 1, maxProfileRuns)
	return profile
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// sortProfiles ordnet nach Namen, ohne auf Groß- und Kleinschreibung zu achten
// — im Auswahlfeld sucht man mit den Augen, nicht nach ASCII-Werten.
func sortProfiles(profiles []Profile) {
	sort.SliceStable(profiles, func(a, b int) bool {
		return strings.ToLower(profiles[a].Name) < strings.ToLower(profiles[b].Name)
	})
}

// List liefert die Profile in der Reihenfolge, in der sie angezeigt werden.
func (s *profileStore) List() []Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Profile(nil), s.profiles...)
}

// Save legt ein Profil ab. Ein vorhandener Name wird überschrieben — genau das
// ist beim Nachbessern gemeint, und ein zweiter Eintrag gleichen Namens wäre
// im Auswahlfeld nicht auseinanderzuhalten.
func (s *profileStore) Save(profile Profile) ([]Profile, error) {
	profile = sanitiseProfile(profile)
	if profile.Name == "" {
		return s.List(), fmt.Errorf("the profile needs a name")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	replaced := false
	for index, existing := range s.profiles {
		if strings.EqualFold(existing.Name, profile.Name) {
			s.profiles[index] = profile
			replaced = true
			break
		}
	}
	if !replaced {
		if len(s.profiles) >= maxProfiles {
			return append([]Profile(nil), s.profiles...),
				fmt.Errorf("there is room for %d profiles — delete one first", maxProfiles)
		}
		s.profiles = append(s.profiles, profile)
	}
	sortProfiles(s.profiles)
	if err := s.write(); err != nil {
		return append([]Profile(nil), s.profiles...), err
	}
	return append([]Profile(nil), s.profiles...), nil
}

// Delete nimmt ein Profil wieder heraus.
func (s *profileStore) Delete(name string) ([]Profile, error) {
	name = strings.TrimSpace(name)

	s.mu.Lock()
	defer s.mu.Unlock()

	kept := make([]Profile, 0, len(s.profiles))
	found := false
	for _, existing := range s.profiles {
		if strings.EqualFold(existing.Name, name) {
			found = true
			continue
		}
		kept = append(kept, existing)
	}
	if !found {
		return append([]Profile(nil), s.profiles...), fmt.Errorf("no profile called %q", name)
	}
	s.profiles = kept
	if err := s.write(); err != nil {
		return append([]Profile(nil), s.profiles...), err
	}
	return append([]Profile(nil), s.profiles...), nil
}

// write legt die Liste ab. Erwartet die Sperre.
//
// Eine leere Liste löscht die Datei: Eine Datei, die nur "[]" enthält, ist
// nichts, was neben der exe herumliegen muss.
func (s *profileStore) write() error {
	if s.path == "" {
		return fmt.Errorf("profiles.go: write: no place to save to (the program's own path is unknown)")
	}
	if len(s.profiles) == 0 {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("profiles.go: write (Remove): %w", err)
		}
		return nil
	}
	raw, err := json.MarshalIndent(s.profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("profiles.go: write (Marshal): %w", err)
	}
	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("profiles.go: write (WriteFile): %w", err)
	}
	return nil
}
