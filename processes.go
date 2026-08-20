// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// processes.go — nachsehen, ob außerhalb dieses Fensters noch umgewandelt wird.
//
// Wozu: Dieses Fenster weiß genau, was es selbst gestartet hat — aber nicht,
// dass nebenbei NVENCForge.exe von Hand läuft (Kontextmenü "Senden an"). Vor
// dem Ausschalten des Rechners ist genau das die eine Frage, die die eigene
// Buchführung nicht beantworten kann. Deshalb wird zusätzlich die
// Prozessliste des Systems gelesen.
//
// Warum die Windows-Schnittstelle und nicht "tasklist": Ein Kommandozeilen-
// Werkzeug aufzurufen kostet einen Prozessstart, seine Ausgabe ist von der
// Sprache des Systems abhängig, und es müsste geparst werden. Der
// Prozess-Schnappschuss liefert die Namen unverfälscht.
package main

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// watchedProcessNames sind die Namen, die "hier wird noch umgewandelt" bedeuten.
//
// Der eigene Prozess heißt NVENCForgeGUI.exe und steht bewusst nicht in der
// Liste — verglichen wird der ganze Name, ein Fenster würde sich sonst selbst
// im Weg stehen.
var watchedProcessNames = []string{"NVENCForge.exe", "ffmpeg.exe", "ffprobe.exe"}

// runningConverters zählt die laufenden Prozesse aus watchedProcessNames.
//
// Ein Name fehlt in der Rückgabe, wenn kein solcher Prozess läuft; eine leere
// Karte heißt also "nichts davon läuft".
func runningConverters() (map[string]int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("processes.go: runningConverters (CreateToolhelp32Snapshot): %w", err)
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, fmt.Errorf("processes.go: runningConverters (Process32First): %w", err)
	}

	found := make(map[string]int)
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		for _, watched := range watchedProcessNames {
			if strings.EqualFold(name, watched) {
				found[watched]++
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			// ERROR_NO_MORE_FILES ist das normale Ende der Liste, jeder andere
			// Fehler bedeutet, dass die Zählung unvollständig ist — und eine
			// unvollständige Zählung darf hier nicht als "nichts läuft"
			// durchgehen.
			if err == windows.ERROR_NO_MORE_FILES {
				return found, nil
			}
			return nil, fmt.Errorf("processes.go: runningConverters (Process32Next): %w", err)
		}
	}
}

// describeProcesses macht aus der Zählung einen Satzteil für das Protokoll,
// z. B. "1 × NVENCForge.exe, 2 × ffmpeg.exe". Die Reihenfolge folgt
// watchedProcessNames, damit die Meldung nicht bei jedem Mal anders aussieht.
func describeProcesses(found map[string]int) string {
	parts := make([]string, 0, len(found))
	for _, name := range watchedProcessNames {
		if count := found[name]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d × %s", count, name))
		}
	}
	return strings.Join(parts, ", ")
}
