// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// datadir.go — der eine Ort für die kleinen Merkdateien des Fensters.
//
// Drei Dinge merkt sich das Fenster: Größe und Platz, die Sparbilanz und die
// Profile. Bis Version 0.9.4 lagen sie direkt neben der exe und machten den
// Ordner unübersichtlich. Sie liegen jetzt im Ordner "tools", in dem ohnehin
// der Konverter und FFmpeg wohnen — neben der exe bleibt damit nur die exe
// selbst und dieser eine Ordner.
//
// Was sich dadurch NICHT ändert: Es wird weiterhin nichts installiert und
// nichts in die Registrierung geschrieben. Ordner löschen und das Programm ist
// weg, wie im README zugesagt.
//
// Die Kehrseite ist ehrlich zu benennen: Wer den tools-Ordner löscht, um
// FFmpeg neu zu laden, wirft Sparbilanz und Profile mit weg. Deshalb steht es
// auch auf der Info-Seite.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// dataDirName ist der Unterordner neben der exe. Bewusst derselbe Ordner wie
// in converter.go: Zwei Konstanten für einen Ort laufen früher oder später
// auseinander.
const dataDirName = toolsDirName

// dataFilePath nennt den vollen Pfad einer Merkdatei und sorgt dafür, dass sie
// erreichbar ist: Der Ordner wird angelegt, und eine Datei aus einer früheren
// Version zieht beim ersten Zugriff von selbst um.
//
// Geht etwas davon schief, liefert die Funktion den alten Platz neben der exe
// zurück statt eines Fehlers. Das ist Absicht: Ein Ordner, der sich nicht
// anlegen lässt, darf weder den Fensterstart verhindern noch eine vorhandene
// Bilanz unsichtbar machen.
func dataFilePath(fileName string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("datadir.go: dataFilePath: %w", err)
	}
	return dataFilePathIn(filepath.Dir(exe), fileName), nil
}

// dataFilePathIn ist der prüfbare Kern von dataFilePath: derselbe Ablauf,
// aber mit frei wählbarem Programmordner.
//
// Er steht getrennt, weil der Umzug der einzige Teil des Fensters ist, bei dem
// ein Fehler eine Datei des Nutzers kosten könnte — und der Ordner der eigenen
// exe lässt sich in einer Prüfung nicht verstellen.
func dataFilePathIn(home, fileName string) string {
	beside := filepath.Join(home, fileName)
	inside := filepath.Join(home, dataDirName, fileName)

	if err := os.MkdirAll(filepath.Join(home, dataDirName), 0o755); err != nil {
		return beside
	}
	if fileExists(beside) && !fileExists(inside) {
		// Klappt der Umzug nicht (etwa weil ein zweites Fenster die Datei
		// gerade offen hat), wird weiter die alte benutzt. Nichts geht
		// verloren, und der nächste Start versucht es erneut.
		if os.Rename(beside, inside) != nil {
			return beside
		}
	}
	return inside
}
