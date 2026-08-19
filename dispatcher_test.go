// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestJobsPerFileWhenConverting: Beim Konvertieren muss jede Datei ihren
// eigenen Prozess bekommen — sonst kann nichts parallel laufen. Bekämen alle
// Instanzen dieselbe Liste, entstünden die Falschmeldungen aus der Messreihe
// vom 2026-08-18 ("No such file or directory", obwohl die Datei fertig ist).
func TestJobsPerFileWhenConverting(t *testing.T) {
	jobs, err := buildJobs(RunRequest{
		Files: []string{`C:\v\a.mkv`, `C:\v\b.mkv`, `C:\v\c.mkv`},
	}, true)
	if err != nil {
		t.Fatalf("buildJobs: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("erwartet 3 Aufträge, bekommen %d", len(jobs))
	}
	for index, expected := range []string{`C:\v\a.mkv`, `C:\v\b.mkv`, `C:\v\c.mkv`} {
		args := jobs[index].args
		if args[len(args)-1] != expected {
			t.Errorf("Auftrag %d: erwartet %s, bekommen %v", index+1, expected, args)
		}
		// Genau eine Datei je Auftrag: Zwei würden auf demselben Platz
		// nacheinander laufen und einen Platz blockieren.
		files := 0
		for _, arg := range args {
			if strings.HasPrefix(arg, `C:\v\`) {
				files++
			}
		}
		if files != 1 {
			t.Errorf("Auftrag %d trägt %d Dateien, erwartet genau 1", index+1, files)
		}
	}
}

// TestToolModesStayOneJob: Zerlegen, DaVinci und Zusammenfügen bleiben EIN
// Auftrag. Sie fragen nach Spuren — zwei Dialoge gleichzeitig für zwei Dateien
// wären eine Zumutung, und eine falsch zugeordnete Antwort zöge die falschen
// Spuren heraus. Zusammenfügen ist ohnehin von Natur aus ein einziger Auftrag.
func TestToolModesStayOneJob(t *testing.T) {
	for _, mode := range []string{"split", "davinci"} {
		jobs, err := buildJobs(RunRequest{
			Mode:  mode,
			Files: []string{`C:\v\a.mkv`, `C:\v\b.mkv`},
		}, true)
		if err != nil {
			t.Fatalf("%s: buildJobs: %v", mode, err)
		}
		if len(jobs) != 1 {
			t.Fatalf("%s: erwartet 1 Auftrag, bekommen %d", mode, len(jobs))
		}
	}

	jobs, err := buildJobs(RunRequest{
		Mode:  "join",
		Files: []string{`C:\v\film.mkv`, `C:\v\film.ger.m4a`},
	}, true)
	if err != nil {
		t.Fatalf("join: buildJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("join: erwartet 1 Auftrag, bekommen %d", len(jobs))
	}
}

// TestClampSlots: Ein leeres Auswahlfeld schickt 0 — das darf nicht bedeuten,
// dass gar nichts läuft. Und mehr als drei Plätze gibt es nicht (gemessen:
// oberhalb von zwei kein Gewinn mehr).
func TestClampSlots(t *testing.T) {
	cases := map[int]int{0: 1, -5: 1, 1: 1, 2: 2, 3: 3, 4: maxConvertSlots, 99: maxConvertSlots}
	for given, want := range cases {
		if got := clampSlots(given); got != want {
			t.Errorf("clampSlots(%d) = %d, erwartet %d", given, got, want)
		}
	}
}

// shortJob liefert Programm und Argumente für einen Auftrag, der etwa eine
// Sekunde dauert. Bewusst ein Bordmittel statt des Konverters: Geprüft wird
// hier der Verteiler, nicht das Konvertieren — und der Test soll ohne
// Grafikkarte und ohne Videodateien laufen.
func shortJob(t *testing.T) (string, []string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("nur unter Windows sinnvoll")
	}
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	if _, err := os.Stat(shell); err != nil {
		t.Skipf("kein cmd.exe gefunden: %v", err)
	}
	return shell, []string{"/c", "ping", "-n", "2", "127.0.0.1"}
}

// waitFor wartet, bis die Bedingung eintritt — höchstens aber so lange, dass
// ein hängender Test nicht die ganze Prüfung blockiert.
func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Zeitüberschreitung: %s", what)
}

// TestDispatcherRunsUpToTheLimit prüft die eigentliche Zusage: Es laufen nie
// mehr Konverter gleichzeitig als eingestellt, und alle Aufträge kommen dran.
func TestDispatcherRunsUpToTheLimit(t *testing.T) {
	exe, args := shortJob(t)

	var maxSeen int
	dispatcher := NewDispatcher(func(name string, data ...any) {})
	watch := func() {
		if active := dispatcher.QueueStatus().Areas[areaConvert].Active; active > maxSeen {
			maxSeen = active
		}
	}

	var jobs []job
	for _, label := range []string{"a", "b", "c", "d"} {
		jobs = append(jobs, job{label: label, args: args})
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 2, false, jobs); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	watch()

	waitFor(t, "alle Aufträge abgearbeitet", func() bool {
		watch()
		return !dispatcher.Busy()
	})

	if maxSeen > 2 {
		t.Errorf("es liefen %d Konverter gleichzeitig, erlaubt waren 2", maxSeen)
	}
	if status := dispatcher.QueueStatus(); status.Areas[areaConvert].Pending != 0 {
		t.Errorf("es warten noch %d Aufträge", status.Areas[areaConvert].Pending)
	}
}

// TestDispatcherStopClearsWhatIsWaiting: Der Abbruch-Knopf muss auch den
// Vorrat leeren. Sonst startete das Nachrücken sofort den nächsten wartenden
// Auftrag — ein Abbruch, der neue Arbeit anfängt.
func TestDispatcherStopClearsWhatIsWaiting(t *testing.T) {
	exe, args := shortJob(t)
	dispatcher := NewDispatcher(func(string, ...any) {})

	jobs := make([]job, 0, 20)
	for i := 0; i < 20; i++ {
		jobs = append(jobs, job{label: "j", args: args})
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 1, false, jobs); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := dispatcher.RequestStop(); err != nil {
		t.Logf("RequestStop meldete: %v", err) // ein bereits beendeter Prozess ist kein Fehler
	}
	if pending := dispatcher.QueueStatus().Areas[areaConvert].Pending; pending != 0 {
		t.Errorf("nach dem Abbruch warten noch %d Aufträge", pending)
	}
	waitFor(t, "nach dem Abbruch läuft nichts mehr", func() bool { return !dispatcher.Busy() })
}

// TestStopSlotLeavesTheRestAlone: Einen einzelnen Konverter anzuhalten darf
// weder die anderen treffen noch den Vorrat leeren — sonst verlöre man beim
// Abbrechen einer Datei den ganzen Stapel.
func TestStopSlotLeavesTheRestAlone(t *testing.T) {
	exe, args := shortJob(t)
	dispatcher := NewDispatcher(func(string, ...any) {})

	var jobs []job
	for i := 0; i < 6; i++ {
		jobs = append(jobs, job{label: "j", args: args})
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 2, false, jobs); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitFor(t, "beide Plätze laufen", func() bool { return dispatcher.QueueStatus().Areas[areaConvert].Active == 2 })

	before := dispatcher.QueueStatus().Areas[areaConvert].Pending
	if err := dispatcher.StopSlot(1); err != nil {
		t.Logf("StopSlot meldete: %v", err) // ein gerade beendeter Prozess ist kein Fehler
	}
	if after := dispatcher.QueueStatus().Areas[areaConvert].Pending; after != before {
		t.Errorf("der Vorrat wurde angetastet: %d statt %d", after, before)
	}
	// Der Stapel muss weiterlaufen und zu Ende kommen.
	waitFor(t, "der Rest läuft durch", func() bool { return !dispatcher.Busy() })
}

// TestStopSlotRefusesAnUnknownSlot: Eine Platznummer, die es nicht gibt, muss
// auffallen, statt still nichts zu tun.
func TestStopSlotRefusesAnUnknownSlot(t *testing.T) {
	dispatcher := NewDispatcher(func(string, ...any) {})
	if err := dispatcher.StopSlot(watchSlot + 1); err == nil {
		t.Error("eine unbekannte Platznummer muss abgelehnt werden")
	}
}

// TestAnswerNeedsAKnownSlot: Eine Antwort ohne gültige Platznummer darf nicht
// still irgendwo landen.
func TestAnswerNeedsAKnownSlot(t *testing.T) {
	dispatcher := NewDispatcher(func(string, ...any) {})
	if err := dispatcher.Answer(watchSlot+1, "1"); err == nil {
		t.Error("eine unbekannte Platznummer muss abgelehnt werden")
	}
	// Ein gültiger Platz, auf dem nichts läuft, meldet ebenfalls einen Fehler —
	// aber den des Läufers ("nothing is waiting for an answer").
	if err := dispatcher.Answer(1, "1"); err == nil {
		t.Error("ohne laufenden Konverter kann nicht geantwortet werden")
	}
}

// Ab hier: die Trennung der beiden Bereiche.
//
// Der beobachtete Ordner und das Umwandeln von Hand laufen nebeneinander, aber
// nicht durcheinander. Was hier geprüft wird, sieht man am Fenster erst, wenn
// es zu spät ist: eine Karte, die von vier Läufen überfahren wird, oder ein
// Dauerauftrag, den ein Abbruch nebenbei mit stillgelegt hat.

// countingDispatcher liefert einen Verteiler, der mitschreibt, wie viele Läufe
// er höchstens gleichzeitig zugelassen hat.
func countingDispatcher(highest *int, perArea map[string]int) *Dispatcher {
	var dispatcher *Dispatcher
	dispatcher = NewDispatcher(func(string, ...any) {
		status := dispatcher.QueueStatus()
		if total := status.Areas[areaConvert].Active + status.Areas[areaWatch].Active; total > *highest {
			*highest = total
		}
		if status.Areas[areaConvert].Active > perArea[areaConvert] {
			perArea[areaConvert] = status.Areas[areaConvert].Active
		}
		if status.Areas[areaWatch].Active > perArea[areaWatch] {
			perArea[areaWatch] = status.Areas[areaWatch].Active
		}
	})
	return dispatcher
}

func jobsFor(count int, args []string) []job {
	var jobs []job
	for i := 0; i < count; i++ {
		jobs = append(jobs, job{label: fmt.Sprintf("file-%d", i), args: args})
	}
	return jobs
}

// TestBothAreasRunSideBySide: Der Kern der Trennung. Ein beobachteter Ordner
// darf nicht warten müssen, bis ein Stapel von Hand fertig ist — genau das war
// vorher der Fall.
func TestBothAreasRunSideBySide(t *testing.T) {
	exe, args := shortJob(t)

	highest := 0
	perArea := map[string]int{}
	dispatcher := countingDispatcher(&highest, perArea)

	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 2, false, jobsFor(3, args)); err != nil {
		t.Fatalf("Submit (convert): %v", err)
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaWatch, 1, false, jobsFor(2, args)); err != nil {
		t.Fatalf("Submit (watch): %v", err)
	}

	waitFor(t, "beide Bereiche laufen gleichzeitig", func() bool {
		status := dispatcher.QueueStatus()
		return status.Areas[areaConvert].Active > 0 && status.Areas[areaWatch].Active > 0
	})
	waitFor(t, "alles läuft durch", func() bool { return !dispatcher.Busy() })

	if perArea[areaWatch] > 1 {
		t.Errorf("der beobachtete Ordner lief %dfach — erlaubt ist genau einer", perArea[areaWatch])
	}
	if perArea[areaConvert] > 2 {
		t.Errorf("beim Umwandeln liefen %d gleichzeitig — eingestellt waren 2", perArea[areaConvert])
	}
}

// TestTotalLimitHoldsAcrossAreas: Die Obergrenze gilt für beide Bereiche
// ZUSAMMEN. Sonst wären es in der Spitze vier Läufe — die Karte ist ab drei
// ausgelastet, ein vierter macht alles nur langsamer.
func TestTotalLimitHoldsAcrossAreas(t *testing.T) {
	exe, args := shortJob(t)

	highest := 0
	dispatcher := countingDispatcher(&highest, map[string]int{})

	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 3, false, jobsFor(4, args)); err != nil {
		t.Fatalf("Submit (convert): %v", err)
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaWatch, 1, false, jobsFor(2, args)); err != nil {
		t.Fatalf("Submit (watch): %v", err)
	}

	waitFor(t, "alles läuft durch", func() bool { return !dispatcher.Busy() })

	if highest > maxTotalSlots {
		t.Errorf("es liefen %d Konverter gleichzeitig, erlaubt sind %d", highest, maxTotalSlots)
	}
	if highest < 2 {
		t.Errorf("es lief nie mehr als %d gleichzeitig — dann prüft dieser Test nichts", highest)
	}
}

// TestCPUModeLowersTheTotalLimit: Auf dem Prozessor kostet jeder weitere Lauf
// echte Kerne, die dem anderen fehlen. Es genügt, dass EIN Bereich auf der CPU
// rechnet — die Kerne teilen sich beide.
func TestCPUModeLowersTheTotalLimit(t *testing.T) {
	exe, args := shortJob(t)

	highest := 0
	dispatcher := countingDispatcher(&highest, map[string]int{})

	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 3, true, jobsFor(4, args)); err != nil {
		t.Fatalf("Submit (convert, CPU): %v", err)
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaWatch, 1, false, jobsFor(2, args)); err != nil {
		t.Fatalf("Submit (watch): %v", err)
	}

	waitFor(t, "alles läuft durch", func() bool { return !dispatcher.Busy() })

	if highest > maxTotalSlotsCPU {
		t.Errorf("im Prozessor-Modus liefen %d gleichzeitig, erlaubt sind %d", highest, maxTotalSlotsCPU)
	}
}

// TestWaitingConvertJobsDoNotBlockTheWatchedFolder: Der Verteiler nimmt nicht
// stur den vordersten Auftrag. Läge ein Stapel von zehn Dateien vorn und wäre
// dessen Bereich voll, müsste ein Fund des beobachteten Ordners warten, obwohl
// SEIN Platz frei ist.
func TestWaitingConvertJobsDoNotBlockTheWatchedFolder(t *testing.T) {
	exe, args := shortJob(t)

	dispatcher := NewDispatcher(func(string, ...any) {})

	// Erst ein Stapel, der mehr Dateien hat als Plätze — es bleibt also
	// garantiert etwas liegen.
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 1, false, jobsFor(3, args)); err != nil {
		t.Fatalf("Submit (convert): %v", err)
	}
	// Und danach ein einzelner Fund aus dem beobachteten Ordner.
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaWatch, 1, false, jobsFor(1, args)); err != nil {
		t.Fatalf("Submit (watch): %v", err)
	}

	waitFor(t, "der Fund startet, obwohl noch Dateien warten", func() bool {
		status := dispatcher.QueueStatus()
		return status.Areas[areaWatch].Active == 1 && status.Areas[areaConvert].Pending > 0
	})
	waitFor(t, "alles läuft durch", func() bool { return !dispatcher.Busy() })
}

// TestStoppingOneAreaLeavesTheOtherAlone: Wer seinen Stapel abbricht, will
// nicht nebenbei den Dauerauftrag stilllegen, von dem auf dieser Seite gar
// nichts zu sehen ist.
func TestStoppingOneAreaLeavesTheOtherAlone(t *testing.T) {
	exe, args := shortJob(t)

	dispatcher := NewDispatcher(func(string, ...any) {})

	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 2, false, jobsFor(4, args)); err != nil {
		t.Fatalf("Submit (convert): %v", err)
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaWatch, 1, false, jobsFor(1, args)); err != nil {
		t.Fatalf("Submit (watch): %v", err)
	}

	waitFor(t, "beide Bereiche laufen", func() bool {
		status := dispatcher.QueueStatus()
		return status.Areas[areaConvert].Active > 0 && status.Areas[areaWatch].Active > 0
	})

	if err := dispatcher.RequestStopArea(areaConvert); err != nil {
		t.Fatalf("RequestStopArea: %v", err)
	}

	// Die wartenden Aufträge des abgebrochenen Bereichs sind weg, die des
	// anderen nicht — und der beobachtete Ordner arbeitet weiter.
	waitFor(t, "der abgebrochene Bereich ist leer", func() bool {
		return dispatcher.QueueStatus().Areas[areaConvert].Pending == 0
	})
	if status := dispatcher.QueueStatus(); status.Areas[areaWatch].Active != 1 {
		t.Errorf("der beobachtete Ordner wurde mit angehalten (WatchActive %d)", status.Areas[areaWatch].Active)
	}

	waitFor(t, "der Rest läuft aus", func() bool { return !dispatcher.Busy() })
}

// TestSubmitRefusesAnUnknownArea: Ein vertippter Bereich darf nicht still als
// „Umwandeln" durchgehen — die Arbeit landete dann auf den falschen Plätzen
// und in der falschen Anzeige.
func TestSubmitRefusesAnUnknownArea(t *testing.T) {
	dispatcher := NewDispatcher(func(string, ...any) {})
	err := dispatcher.Submit("x", ".", "somewhere", 1, false, []job{{label: "a", args: []string{"b"}}})
	if err == nil {
		t.Error("ein unbekannter Bereich muss abgelehnt werden")
	}
}

// TestAreaOfSlot: Die Fensterseite erkennt an der Platznummer allein, in welche
// Anzeige eine Meldung gehört. Rechnet sie anders als der Verteiler, landen
// Meldungen im falschen Protokoll.
func TestAreaOfSlot(t *testing.T) {
	for slot := 1; slot <= maxConvertSlots; slot++ {
		if got := areaOfSlot(slot); got != areaConvert {
			t.Errorf("Platz %d gehört zu %q, erwartet %q", slot, got, areaConvert)
		}
	}
	if got := areaOfSlot(watchSlot); got != areaWatch {
		t.Errorf("Platz %d gehört zu %q, erwartet %q", watchSlot, got, areaWatch)
	}
}

// TestToolAreasRunBesideTheCard: Zerlegen und Zusammenfügen kopieren nur
// Spuren — dort ist die Festplatte die Grenze, nicht die Karte. Sie stehen
// deshalb NEBEN der gemeinsamen Obergrenze und nicht dahinter: Ein voll
// ausgelastetes Umwandeln darf ein Zerlegen nicht aufhalten.
func TestToolAreasRunBesideTheCard(t *testing.T) {
	exe, args := shortJob(t)

	dispatcher := NewDispatcher(func(string, ...any) {})

	// Drei Umwandlungen: die Karte ist damit voll.
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 3, false, jobsFor(3, args)); err != nil {
		t.Fatalf("Submit (convert): %v", err)
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaSplit, 1, false, jobsFor(1, args)); err != nil {
		t.Fatalf("Submit (split): %v", err)
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaJoin, 1, false, jobsFor(1, args)); err != nil {
		t.Fatalf("Submit (join): %v", err)
	}

	waitFor(t, "Zerlegen und Zusammenfügen laufen neben dem vollen Umwandeln", func() bool {
		status := dispatcher.QueueStatus()
		return status.Areas[areaConvert].Active == maxTotalSlots &&
			status.Areas[areaSplit].Active == 1 &&
			status.Areas[areaJoin].Active == 1
	})
	waitFor(t, "alles läuft durch", func() bool { return !dispatcher.Busy() })
}

// TestToolAreasStayAtOne: Ein Platz je Werkzeug-Bereich. Zwei Spurauswahl-
// Dialoge gleichzeitig aus demselben Bereich wären eine Zumutung, und schneller
// wird es davon auch nicht.
func TestToolAreasStayAtOne(t *testing.T) {
	exe, args := shortJob(t)

	highest := map[string]int{}
	var dispatcher *Dispatcher
	dispatcher = NewDispatcher(func(string, ...any) {
		status := dispatcher.QueueStatus()
		for _, area := range []string{areaSplit, areaJoin} {
			if status.Areas[area].Active > highest[area] {
				highest[area] = status.Areas[area].Active
			}
		}
	})

	for _, area := range []string{areaSplit, areaJoin} {
		if err := dispatcher.Submit(exe, filepath.Dir(exe), area, 3, false, jobsFor(3, args)); err != nil {
			t.Fatalf("Submit (%s): %v", area, err)
		}
	}
	waitFor(t, "alles läuft durch", func() bool { return !dispatcher.Busy() })

	for _, area := range []string{areaSplit, areaJoin} {
		if highest[area] != 1 {
			t.Errorf("%s lief %dfach gleichzeitig, erlaubt ist genau einer", area, highest[area])
		}
	}
}

// TestCPUCapDoesNotReachIntoTheToolAreas: Der Prozessor-Deckel schützt die
// Kerne der Umwandlung. Ein Zerlegen daneben kopiert bloß und darf davon nicht
// ausgebremst werden.
func TestCPUCapDoesNotReachIntoTheToolAreas(t *testing.T) {
	exe, args := shortJob(t)

	dispatcher := NewDispatcher(func(string, ...any) {})
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaConvert, 3, true, jobsFor(2, args)); err != nil {
		t.Fatalf("Submit (convert, CPU): %v", err)
	}
	if err := dispatcher.Submit(exe, filepath.Dir(exe), areaSplit, 1, false, jobsFor(1, args)); err != nil {
		t.Fatalf("Submit (split): %v", err)
	}

	waitFor(t, "das Zerlegen läuft trotz Prozessor-Deckel", func() bool {
		status := dispatcher.QueueStatus()
		return status.Areas[areaConvert].Active == maxTotalSlotsCPU && status.Areas[areaSplit].Active == 1
	})
	waitFor(t, "alles läuft durch", func() bool { return !dispatcher.Busy() })
}

// TestEveryAreaHasItsOwnSlot: Die Fensterseite ordnet jede Meldung allein
// anhand der Platznummer einem Bereich zu. Zwei Bereiche auf derselben Nummer
// hiessen: Meldungen im falschen Protokoll.
func TestEveryAreaHasItsOwnSlot(t *testing.T) {
	seen := map[string]int{}
	for slot := 1; slot <= joinSlot; slot++ {
		seen[areaOfSlot(slot)]++
	}
	if seen[areaConvert] != maxConvertSlots {
		t.Errorf("Umwandeln hat %d Plätze, erwartet %d", seen[areaConvert], maxConvertSlots)
	}
	for _, area := range []string{areaWatch, areaSplit, areaJoin} {
		if seen[area] != 1 {
			t.Errorf("%s hat %d Plätze, erwartet genau einen", area, seen[area])
		}
	}
	// Und jeder Bereich, den die Oberfläche kennt, kommt auch wirklich vor.
	for _, area := range areas {
		if seen[area] == 0 {
			t.Errorf("für %q gibt es gar keinen Platz", area)
		}
	}
}
