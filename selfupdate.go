// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// selfupdate.go — nachsehen, ob es eine neuere Ausgabe DIESES Fensters gibt,
// sie einspielen und neu starten.
//
// Der Weg ist derselbe wie beim Konverter (converter.go): GitHub nach der
// neuesten Veröffentlichung fragen, den Anhang laden, erst in eine Teildatei
// schreiben. Neu ist nur, dass die Datei, die ersetzt wird, gerade selbst
// läuft. Windows lässt eine laufende Programmdatei nicht überschreiben, wohl
// aber UMBENENNEN — genau darauf baut der Tausch:
//
//  1. Die neue Ausgabe landet als "NVENCForgeGUI.exe.part" neben der laufenden.
//  2. Die laufende wandert als Sicherung in den tools-Ordner.
//  3. Die Teildatei nimmt ihren Platz ein.
//  4. Die neue Ausgabe wird gestartet, dieses Fenster schließt sich.
//
// Gemessen am 2026-08-21 mit einem eigenen kleinen Programm: Das Verschieben
// der laufenden exe in einen Unterordner gelingt, und das Programm läuft danach
// unbeeindruckt weiter.
//
// Der eine Stolperstein, der das Ganze sonst still kaputtmachen würde, ist die
// Startsperre aus main.go: Die neue Ausgabe darf ihr Fenster erst öffnen, wenn
// die alte wirklich beendet ist. Deshalb bekommt sie beim Start die
// Prozessnummer der alten mit und wartet auf deren Ende.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const (
	// guiExeName ist der Name, unter dem dieses Fenster veröffentlicht wird —
	// als Anhang am Release und als Datei auf der Platte.
	guiExeName = "NVENCForgeGUI.exe"

	// guiLatestReleaseURL zeigt auf das eigene Verzeichnis, nicht auf das des
	// Konverters. Zwei Programme, zwei Ablagen.
	guiLatestReleaseURL = "https://api.github.com/repos/burnersen/NVENCForgeGUI/releases/latest"

	// afterUpdateFlag sagt der frisch gestarteten Ausgabe, dass sie aus einem
	// Update kommt. Dahinter steht die Prozessnummer der alten Ausgabe.
	afterUpdateFlag = "--after-update"

	// predecessorWait ist die Geduld beim Warten auf die alte Ausgabe. Sie
	// beendet sich normalerweise in einer Sekunde; der großzügige Wert deckt
	// den Fall ab, dass sie noch ihre Fenstergröße wegschreibt.
	predecessorWait = 30 * time.Second

	// closeDelayAfterUpdate ist die Atempause zwischen "Ergebnis gemeldet" und
	// "Fenster zu". Ohne sie wäre das Fenster weg, bevor die Oberfläche das
	// Ergebnis überhaupt anzeigen konnte.
	closeDelayAfterUpdate = 1500 * time.Millisecond

	// exeBackupSuffix hängt an der Sicherung der vorigen Ausgabe. Bewusst nicht
	// ".exe": Eine Sicherung soll man nicht aus Versehen doppelklicken. Der
	// Name grenzt sich von backupSuffix in configfile.go ab — das ist die
	// Sicherung einer Einstellungsdatei und hat mit dieser hier nichts zu tun.
	exeBackupSuffix = ".exe.bak"
)

// UpdateCheck ist die Antwort auf die Frage "gibt es etwas Neueres?".
type UpdateCheck struct {
	Newer     bool   `json:"newer"`
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	SizeBytes int64  `json:"sizeBytes"`
	Note      string `json:"note"`
}

// UpdateResult sagt, was das Einspielen bewirkt hat.
type UpdateResult struct {
	Installed  bool   `json:"installed"`
	Restarting bool   `json:"restarting"`
	Version    string `json:"version"`
	BackupPath string `json:"backupPath"`
	Message    string `json:"message"`
}

// checkForUpdate fragt GitHub nach der neuesten Ausgabe dieses Fensters.
//
// Die Prüfung passiert nur auf Knopfdruck: Das Fenster fragt von sich aus
// nichts im Netz nach — so gewünscht.
func checkForUpdate(ctx context.Context) (UpdateCheck, error) {
	check := UpdateCheck{Current: guiVersion}

	release, err := fetchRelease(ctx, guiLatestReleaseURL)
	if err != nil {
		return check, err
	}
	check.Latest = strings.TrimSpace(release.TagName)

	asset, err := pickAsset(release, guiExeName)
	if err != nil {
		return check, err
	}
	check.SizeBytes = asset.Size
	check.Newer = isNewerVersion(guiVersion, check.Latest)

	if check.Newer {
		check.Note = "NVENCForgeGUI " + check.Latest + " is available."
	} else {
		check.Note = "This is the newest release (" + check.Latest + ")."
	}
	return check, nil
}

// isNewerVersion sagt, ob latest eine höhere Ausgabe ist als current.
//
// Verglichen wird Zahl für Zahl, nicht Zeichen für Zeichen: Buchstabenweise
// wäre "1.9.0" größer als "1.10.0", und das Update käme nie an.
func isNewerVersion(current, latest string) bool {
	currentParts := versionNumbers(current)
	latestParts := versionNumbers(latest)

	length := len(currentParts)
	if len(latestParts) > length {
		length = len(latestParts)
	}
	for index := 0; index < length; index++ {
		// Fehlende Stellen zählen als 0, damit "1.1" und "1.1.0" gleich sind.
		currentValue := numberAt(currentParts, index)
		latestValue := numberAt(latestParts, index)
		if latestValue != currentValue {
			return latestValue > currentValue
		}
	}
	return false
}

// versionNumbers zerlegt "v1.10.2" in 1, 10, 2.
//
// Was keine reine Zahl ist, beendet die Zerlegung: Aus "1.2.0-beta" wird
// 1, 2, 0. Eine Vorab-Ausgabe gilt damit als ihre eigene Nummer — genauer muss
// es hier nicht sein, weil dieses Programm nur fertige Ausgaben veröffentlicht.
func versionNumbers(version string) []int {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")

	var numbers []int
	for _, part := range strings.Split(trimmed, ".") {
		digits := part
		if cut := strings.IndexFunc(part, isNotDigit); cut >= 0 {
			digits = part[:cut]
		}
		value, err := strconv.Atoi(digits)
		if err != nil {
			break
		}
		numbers = append(numbers, value)
	}
	return numbers
}

// isNotDigit sagt, wo eine Zahl aufhört.
func isNotDigit(letter rune) bool {
	return letter < '0' || letter > '9'
}

// numberAt liefert die Stelle oder 0, wenn es sie nicht gibt.
func numberAt(numbers []int, index int) int {
	if index >= len(numbers) {
		return 0
	}
	return numbers[index]
}

// installUpdate lädt die neueste Ausgabe und setzt sie an den Platz der
// laufenden. Gestartet wird sie hier NICHT — das entscheidet der Aufrufer,
// der als Einziger weiß, ob das Fenster gerade gehen darf.
func installUpdate(ctx context.Context, report func(done, total int64)) (UpdateResult, error) {
	result := UpdateResult{}

	self, err := os.Executable()
	if err != nil {
		return result, fmt.Errorf("selfupdate.go: installUpdate (Executable): %w", err)
	}

	// Vor dem Laden prüfen, ob hier überhaupt geschrieben werden darf: Zwölf
	// Megabyte zu holen, um dann an "Zugriff verweigert" zu scheitern, wäre
	// verschenkte Zeit — und die Meldung käme aus dem falschen Schritt.
	if err := canWriteInto(filepath.Dir(self)); err != nil {
		return result, err
	}

	release, err := fetchRelease(ctx, guiLatestReleaseURL)
	if err != nil {
		return result, err
	}
	result.Version = strings.TrimSpace(release.TagName)

	if !isNewerVersion(guiVersion, result.Version) {
		result.Message = "Already running the newest release (" + result.Version + ") — nothing was changed."
		return result, nil
	}

	asset, err := pickAsset(release, guiExeName)
	if err != nil {
		return result, err
	}

	partPath := self + ".part"
	if err := downloadToFile(ctx, asset, partPath, report); err != nil {
		return result, err
	}

	// Eine halbe exe darf niemals den Platz der laufenden einnehmen. Die
	// angehängte Größe ist der einzige Vergleichswert, den GitHub mitliefert.
	if err := verifyDownloadedSize(partPath, asset.Size); err != nil {
		_ = os.Remove(partPath)
		return result, err
	}

	backupPath := backupPathFor(self, guiVersion)
	if err := swapInNewExe(self, partPath, backupPath); err != nil {
		_ = os.Remove(partPath)
		return result, err
	}

	result.Installed = true
	result.BackupPath = backupPath
	result.Message = "NVENCForgeGUI " + result.Version + " installed. The previous version is kept as " +
		filepath.Base(backupPath) + " in the tools folder."
	return result, nil
}

// verifyDownloadedSize vergleicht die geladene Datei mit der angekündigten
// Größe. Ist keine Größe angekündigt, wird nur auf "nicht leer" geprüft.
func verifyDownloadedSize(path string, expected int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("selfupdate.go: verifyDownloadedSize (Stat): %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("selfupdate.go: verifyDownloadedSize: the download is empty")
	}
	if expected > 0 && info.Size() != expected {
		return fmt.Errorf(
			"selfupdate.go: verifyDownloadedSize: the download is incomplete (%d of %d bytes)",
			info.Size(), expected)
	}
	return nil
}

// canWriteInto stellt fest, ob in einem Ordner geschrieben werden darf.
//
// Der Weg über eine echte Testdatei ist der einzige verlässliche: Über Rechte,
// Vererbung und Schreibschutz entscheidet Windows erst beim Zugriff.
func canWriteInto(dir string) error {
	probe, err := os.CreateTemp(dir, "nvencforgegui-write-*.tmp")
	if err != nil {
		return fmt.Errorf(
			"selfupdate.go: canWriteInto: cannot write into %s — move the program to a folder you own, "+
				"then try again: %w", dir, err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

// backupPathFor nennt den Platz der Sicherung: im tools-Ordner, nicht neben
// der exe — neben der Programmdatei soll nur die Programmdatei liegen.
func backupPathFor(exePath, version string) string {
	base := strings.TrimSuffix(filepath.Base(exePath), filepath.Ext(exePath))
	name := base + "-" + strings.TrimSpace(version) + exeBackupSuffix
	return filepath.Join(filepath.Dir(exePath), toolsDirName, name)
}

// swapInNewExe schiebt die laufende Programmdatei zur Seite und setzt die
// geladene an ihren Platz.
//
// Der Rückweg ist der Grund für diese Reihenfolge: Solange nur umbenannt wird,
// ist jeder Schritt umkehrbar. Scheitert der zweite, wandert die alte Datei
// sofort zurück — sonst stünde beim nächsten Start gar kein Programm mehr da.
func swapInNewExe(currentExe, newFile, backupPath string) error {
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return fmt.Errorf("selfupdate.go: swapInNewExe (MkdirAll): %w", err)
	}
	// Eine Sicherung derselben Ausgabe aus einem früheren Versuch weicht.
	_ = os.Remove(backupPath)

	if err := os.Rename(currentExe, backupPath); err != nil {
		return fmt.Errorf("selfupdate.go: swapInNewExe (move the running file aside): %w", err)
	}
	if err := os.Rename(newFile, currentExe); err != nil {
		if back := os.Rename(backupPath, currentExe); back != nil {
			return fmt.Errorf(
				"selfupdate.go: swapInNewExe (put the new file in place): %w — "+
					"the previous version now sits at %s and has to be moved back by hand", err, backupPath)
		}
		return fmt.Errorf("selfupdate.go: swapInNewExe (put the new file in place): %w", err)
	}
	return nil
}

// startSuccessor startet die frisch eingespielte Ausgabe und sagt ihr, auf
// welchen Prozess sie warten muss.
func startSuccessor(exePath string) error {
	command := exec.Command(exePath, afterUpdateFlag, strconv.Itoa(os.Getpid()))
	command.Dir = filepath.Dir(exePath)
	// DETACHED_PROCESS ist hier Pflicht, nicht Geschmackssache: Ohne das erbt
	// der Nachfolger die versteckte Konsole DIESES Prozesses (wincon.go). Sein
	// eigenes AllocConsole schlüge dann fehl, und sobald dieser Prozess endet,
	// stünde er ohne Konsole da — der saubere Abbruch eines Laufs wäre kaputt,
	// ohne dass man es dem Fenster ansieht.
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("selfupdate.go: startSuccessor (Start): %w", err)
	}
	// Bewusst kein Wait: Der Nachfolger soll diesen Prozess überleben. Release
	// gibt nur das Handle frei, das Go sich gemerkt hat.
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("selfupdate.go: startSuccessor (Release): %w", err)
	}
	return nil
}

// predecessorPID liest die Prozessnummer der alten Ausgabe aus den
// Startargumenten. 0 heißt "ganz normal gestartet, es gibt nichts zu warten".
func predecessorPID(args []string) int {
	for index, argument := range args {
		if argument != afterUpdateFlag {
			continue
		}
		if index+1 >= len(args) {
			return 0
		}
		pid, err := strconv.Atoi(args[index+1])
		if err != nil || pid <= 0 {
			return 0
		}
		return pid
	}
	return 0
}

// waitForPredecessor hält den Start an, bis die alte Ausgabe beendet ist.
//
// Ohne dieses Warten wäre das Selbst-Update still kaputt: Die Startsperre in
// main.go sieht die noch laufende alte Ausgabe, das neue Fenster beendet sich
// sofort wieder — und der Nutzer bekäme nach dem Update weiter die alte
// Version zu sehen, ohne zu verstehen, warum.
//
// Weitergemacht wird danach in JEDEM Fall. Läuft die alte Ausgabe wider
// Erwarten immer noch, greift eben die Startsperre; das ist unschön, aber
// deutlich besser als ein Fenster, das gar nicht mehr aufgeht.
func waitForPredecessor(pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// Kein Zugriff heißt hier fast immer: Der Prozess ist schon weg.
		return
	}
	defer func() { _ = windows.CloseHandle(handle) }()

	milliseconds := timeout.Milliseconds()
	if milliseconds < 0 {
		milliseconds = 0
	}
	_, _ = windows.WaitForSingleObject(handle, uint32(milliseconds))
}
