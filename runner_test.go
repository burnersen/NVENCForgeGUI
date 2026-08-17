package main

import (
	"strings"
	"testing"
)

// Echte Zeilen aus einem Lauf vom 2026-08-17. Sie sind der Grund für diese
// Datei: Im ersten Anlauf standen die Steuerzeichen unverändert im
// Protokollfenster, weil der Weg ohne Datenkanal sie ungefiltert durchreichte.
const (
	realHeading   = "\x1b[46;30;1m\x1b[46;30;1m   INFO   \x1b[0m\x1b[0m \x1b[96m\x1b[96mChecking GPU capabilities...\x1b[0m\x1b[0m"
	realEmptyBar  = "\x1b[44m\x1b[44m                    \x1b[0m\x1b[0m"
	realRedraw    = "\x1b[3A\x1b[2K  Frame  150   27.3%"
	realPlainText = "  Skipped: output file already exists."
)

func TestColoursSurviveButCursorCommandsDoNot(t *testing.T) {
	line, ok := toLogLine(realHeading)
	if !ok {
		t.Fatal("a line with text must be kept")
	}
	if !strings.Contains(line.Text, "Checking GPU capabilities...") {
		t.Errorf("the text was lost: %q", line.Text)
	}
	if !strings.Contains(line.Text, "\x1b[96m") {
		t.Errorf("the colour must survive so the window can show it: %q", line.Text)
	}
	if line.Back != 0 {
		t.Errorf("this line overwrites nothing, got Back=%d", line.Back)
	}
}

func TestCursorCommandsAreRemoved(t *testing.T) {
	line, ok := toLogLine(realRedraw)
	if !ok {
		t.Fatal("a redraw with text must be kept")
	}
	if strings.Contains(line.Text, "\x1b[3A") || strings.Contains(line.Text, "\x1b[2K") {
		t.Errorf("cursor commands must be gone: %q", line.Text)
	}
	if !strings.Contains(line.Text, "Frame  150") {
		t.Errorf("the text was lost: %q", line.Text)
	}
	// Ohne diese Zahl stapelten sich zehn fast gleiche Zeilen pro Sekunde.
	if line.Back != 3 {
		t.Errorf("a jump back of three lines must be reported, got %d", line.Back)
	}
}

func TestCursorUpWithoutNumberCountsAsOne(t *testing.T) {
	line, _ := toLogLine("\x1b[Aback one")
	if line.Back != 1 {
		t.Errorf("an empty count means one line, got %d", line.Back)
	}
}

func TestLinesWithNothingToShowAreDropped(t *testing.T) {
	// Reine Farbbalken ohne Text sind nur Zierrat der Terminal-Überschriften.
	if _, ok := toLogLine(realEmptyBar); ok {
		t.Error("a bar without text must not reach the log")
	}
	if _, ok := toLogLine("   \t "); ok {
		t.Error("an empty line must not reach the log")
	}
	// Ein reiner Rücksprung ohne Text muss aber durch: Er räumt auf.
	line, ok := toLogLine("\x1b[2A")
	if !ok || line.Text != "" || line.Back != 2 {
		t.Errorf("a pure jump back must be passed on: ok=%v line=%+v", ok, line)
	}
}

func TestPlainTextSurvivesUnchanged(t *testing.T) {
	line, ok := toLogLine(realPlainText)
	if !ok || line.Text != realPlainText {
		t.Errorf("plain text must not be touched: ok=%v %q", ok, line.Text)
	}
}

func TestEventsAreRecognisedOnlyAsSuchOnTheMainChannel(t *testing.T) {
	if _, ok := parseEvent(`{"ev":"progress","pct":12.5}`); !ok {
		t.Error("a real event must be recognised")
	}
	// Das Wichtigste: Ohne Datenkanal kommt Bildschirmausgabe über denselben
	// Kanal herein. Nichts davon darf als Ereignis durchgehen.
	for _, notAnEvent := range []string{
		realHeading, realPlainText, "{not json}", `{"a":1}`, "",
	} {
		if _, ok := parseEvent(notAnEvent); ok {
			t.Errorf("screen output must not pass as an event: %q", notAnEvent)
		}
	}
}

func TestSplitLinesAndReturns(t *testing.T) {
	// Die Fortschrittsanzeige beendet ihre Zeilen mit "\r". Ohne diese
	// Trennung wüchse daraus eine einzige riesige Zeile.
	data := []byte("first\rsecond\nthird")
	advance, token, err := splitLinesAndReturns(data, false)
	if err != nil || string(token) != "first" || advance != 6 {
		t.Fatalf("carriage return must end a line: token=%q advance=%d err=%v", token, advance, err)
	}
	advance, token, _ = splitLinesAndReturns(data[advance:], false)
	if string(token) != "second" || advance != 7 {
		t.Fatalf("newline must end a line: token=%q advance=%d", token, advance)
	}
	_, token, _ = splitLinesAndReturns([]byte("third"), true)
	if string(token) != "third" {
		t.Errorf("the last piece without a line ending must still arrive: %q", token)
	}
}
