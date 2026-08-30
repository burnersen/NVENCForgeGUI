// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// runargs.go — aus den Schaltern der Oberfläche wird die Befehlszeile.
//
// Eine eigene Datei, weil das die Stelle ist, an der die Oberfläche und der
// Konverter sich berühren: Jeder Schalter hier entspricht genau einem
// dokumentierten Parameter (Help.go). Steht ein Wert nicht drin, wird auch
// nichts übergeben — dann gilt die Einstellung aus der INI, und das ist die
// Vorgabe, die der Nutzer erwartet.
package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Erlaubte Werte der Auswahlfelder. Als Konstanten, damit ein Tippfehler beim
// Vergleich auffällt statt still den Standardfall zu wählen.
const (
	codecAV1  = "av1"
	codecH265 = "h265"

	encoderCPU    = "cpu"
	encoderNvidia = "nvidia"

	containerMP4 = "mp4"

	resolutionOriginal = "original"

	audioCopy = "copy"

	bitDepth8 = "8"

	// Die Gegenstücke. Sie sind NICHT dasselbe wie der leere Wert: Der bedeutet
	// "das Fenster sagt nichts dazu, die INI gilt". Seit NVENCForge 1.23.0
	// stehen diese Entscheidungen in der INI, also muss das Fenster sie in
	// BEIDE Richtungen durchsetzen können — eine Oberfläche kann Argumente nur
	// mitgeben, nie wegnehmen.
	containerMKV        = "mkv"
	resolutionDownscale = "downscale"
	audioAAC            = "aac"
	bitDepth10          = "10"
	qualityAuto         = "auto"
	qualityOff          = "off"
	qualityFixed        = "fixed"

	// qualityCheck ist der Prüflauf: die Suche läuft, meldet den CQ, den sie
	// nehmen würde, und konvertiert nichts. Er steht hier bei der Qualität und
	// nicht als eigener Schalter, weil er dieselbe Frage anders beantwortet —
	// und weil ein fester CQ daneben keinen Sinn ergäbe.
	qualityCheck = "check"

	// Auto-Crop: "on" schneidet schwarze Balken weg, "check" schaut nur nach
	// und legt ein Kontrollbild neben die Quelle, ohne etwas zu konvertieren.
	cropOn    = "on"
	cropCheck = "check"

	// cropOff heißt "ausdrücklich nicht schneiden" und ist NICHT dasselbe wie
	// der leere Wert: der bedeutet nur "das Fenster hat nichts dazu gesagt".
	cropOff = "off"
)

// Grenzen der CQ-Skala, wie der Konverter sie prüft (main.go, parseArgs).
const (
	minCQ     = 1
	maxCQH265 = 51
	maxCQAV1  = 63
)

// Grenzen für die Bitrate in kbit/s. Unten so tief, dass auch sehr sparsame
// Vorgaben möglich bleiben; oben weit über allem, was sinnvoll ist — die
// Zahl soll nur Tippfehler wie 800000 abfangen.
const (
	minBitrateKbps = 100
	maxBitrateKbps = 200000
)

// Die beiden Wege, aus Einzeldateien wieder eine Datei zu machen. Sie brauchen
// dieselbe Zusammenstellung, liefern aber Verschiedenes:
//
//   - modeJoin (-join): alles 1:1 kopiert, Ergebnis ".joined.mkv".
//   - modeJoinDavinci (-davinci): Ton wird nach AAC umkodiert, wo DaVinci ihn
//     sonst nicht liest, Untertitel werden gereinigt, Ergebnis ".sub.mkv".
//
// Sie stehen als Konstanten da, weil sie als Einzige eine eigene Prüfung haben
// — ein Tippfehler im Vergleich würde sie still überspringen.
const (
	modeJoin        = "join"
	modeJoinDavinci = "joindavinci"
)

// modeFlags übersetzt die Modus-Bereiche der Oberfläche in ihr Flag.
//
// Diese Flags MÜSSEN das erste Argument sein: Der Konverter erkennt seinen
// Betriebsmodus an os.Args[1] und sonst nirgends. -json fällt bei ihm schon
// vorher aus der Liste und stört deshalb nicht.
var modeFlags = map[string]string{
	"davinci":       "-davinci",
	"split":         "-split",
	modeJoin:        "-join",
	modeJoinDavinci: "-davinci",
}

// needsJoinOrder sagt, ob ein Modus die Zusammenstellung "genau ein Bild zuerst"
// braucht. Beide Zusammenfüge-Wege reichen dem Konverter dieselbe Liste.
func needsJoinOrder(mode string) bool {
	return mode == modeJoin || mode == modeJoinDavinci
}

// RunRequest ist das, was die Oberfläche für einen Lauf schickt.
type RunRequest struct {
	// Area sagt, aus welchem Bereich des Fensters der Auftrag kommt:
	// leer/"convert" für das Umwandeln von Hand, "split" fürs Zerlegen,
	// "join" fürs Zusammenfügen, "watch" für den beobachteten Ordner. Davon
	// hängt ab, auf welchem Platz er läuft und in welcher Anzeige seine
	// Meldungen landen.
	Area       string   `json:"area"`
	Mode       string   `json:"mode"`     // "" = konvertieren, sonst Schlüssel aus modeFlags
	Parallel   int      `json:"parallel"` // gleichzeitige Läufe (1–3), 0 = einer
	Files      []string `json:"files"`
	Codec      string   `json:"codec"`      // "", "av1" oder "h265"
	Encoder    string   `json:"encoder"`    // "" oder "cpu"
	Container  string   `json:"container"`  // "" oder "mp4"
	Resolution string   `json:"resolution"` // "" oder "original"
	Audio      string   `json:"audio"`      // "" oder "copy"
	BitDepth   string   `json:"bitDepth"`   // "" oder "8"
	Quality    string   `json:"quality"`    // "", "auto", "off" oder "fixed"
	Crop       string   `json:"crop"`       // "", "on" oder "check"
	FixedCQ    int      `json:"fixedCQ"`
	MaxBitrate int      `json:"maxBitrate"` // 0 = Wert aus der INI
	KeepSource bool     `json:"keepSource"`

	// MeasuredCQ enthält die CQ-Werte, die ein vorangegangener Prüflauf für
	// einzelne Dateien schon ermittelt hat (Pfad → CQ).
	//
	// Warum das etwas bringt: Die Suche kostet je Datei rund eine halbe Minute.
	// Wer erst prüft und dann konvertiert, bezahlt sie sonst zweimal für
	// dasselbe Ergebnis. Da das Fenster JE DATEI einen eigenen Konverter
	// startet, kann jede ihren eigenen Wert bekommen.
	//
	// Benutzt wird ein Wert nur, wenn die Qualitätswahl weiterhin auf "auto"
	// steht — bei einem festen CQ hat der Nutzer selbst entschieden. Ob die
	// bildrelevanten Einstellungen seit der Messung gleich geblieben sind,
	// entscheidet das Fenster, bevor es diese Liste überhaupt füllt: ein CQ
	// aus einer anderen Auflösung oder einem anderen Codec wäre schlicht falsch.
	MeasuredCQ map[string]int `json:"measuredCQ,omitempty"`

	// Shutdown ist der WUNSCH "PC ausschalten, wenn alles fertig ist" — kein
	// Schalter für die Befehlszeile. Er geht ausdrücklich NICHT an den
	// Konverter: Der kennt nur seine eine Datei und würde nach ihr abschalten,
	// mitten in der Arbeit der übrigen. Ausgeführt wird der Wunsch von
	// shutdown.go, wenn wirklich alles leer ist.
	Shutdown bool `json:"shutdown"`

	// SuppressConverterShutdown ist KEINE Wahl aus dem Fenster, sondern ein
	// Schutz — deshalb steht es auch nicht in der JSON-Schnittstelle.
	//
	// Steht autoShutdown=true in der INI, schaltet der Konverter ab, sobald
	// SEINE eine Datei fertig ist, und reißt den Rest des Stapels mit. Das ist
	// derselbe Schaden, der bis v1.0.1 im Fenster steckte (siehe unten bei den
	// Argumenten), nur diesmal von der Konfigurationsdatei ausgelöst. Setzt
	// app.go, sobald die Programmdatei "-noshutdown" kennt.
	SuppressConverterShutdown bool `json:"-"`

	// CounterFlags ist keine Wahl aus dem Fenster, sondern die Auskunft, ob die
	// vorhandene Programmdatei die Gegenschalter aus NVENCForge 1.23.0 kennt
	// (-mkv, -aac, -10bit, -nokeep, -downscale). Eine ältere exe würde jeden
	// davon als unbekannte Option anmeckern. Gesetzt in app.go, wo der Status
	// der Programmdatei bekannt ist.
	CounterFlags bool `json:"-"`
}

// buildJobs macht aus einer Anfrage die einzelnen Aufträge für den Verteiler.
//
// Beim KONVERTIEREN bekommt jede Datei ihren eigenen Prozess. Nur so können
// mehrere gleichzeitig laufen, ohne dass zwei Konverter sich um dieselbe Datei
// streiten — die Begründung samt Messung steht im Kopf von dispatcher.go.
//
// ZUSAMMENFÜGEN bekommt einen Auftrag je Bild-Grundlage. Der Konverter baut
// pro Lauf genau eine Datei, ein Stapel zerlegter Filme sind aber viele — und
// die Zuordnung steht im Namen (joinfiles.go). Sie laufen nacheinander: Der
// Bereich hat genau einen Platz, mehr wäre auch nichts wert, weil hier die
// Festplatte bremst und nicht die Grafikkarte.
//
// ZERLEGEN und der DaVinci-Weg bleiben ein einziger Auftrag mit allen Dateien:
//   - Sie kopieren nur, statt zu rechnen; parallel gewönne man nichts.
//   - Sie fragen nach Spuren. Zwei Dialoge gleichzeitig für zwei Dateien wären
//     eine Zumutung, und eine falsch zugeordnete Antwort zöge die falschen
//     Spuren heraus. Der Konverter fragt innerhalb seines einen Laufs ohnehin
//     Datei für Datei nach.
func buildJobs(request RunRequest, eventChannel bool) ([]job, error) {
	if needsJoinOrder(request.Mode) {
		return buildJoinJobs(request, eventChannel)
	}
	if request.Mode != "" {
		args, err := buildConverterArgs(request, eventChannel)
		if err != nil {
			return nil, err
		}
		return []job{{label: request.Mode, args: args}}, nil
	}

	if len(request.Files) == 0 {
		return nil, fmt.Errorf("runargs.go: buildJobs: the queue is empty")
	}

	jobs := make([]job, 0, len(request.Files))
	for _, file := range request.Files {
		single := request
		single.Files = []string{file}
		// Ein schon gemessener CQ macht die Suche für DIESE Datei überflüssig.
		// Er wird als fester Wert übergeben, was Auto-CQ im Konverter
		// überstimmt — genau das ist gewollt, denn er IST das Ergebnis von
		// Auto-CQ, nur eben von vorhin. Kostendeckel und Bremsen stecken
		// bereits darin, es ist der Wert nach allen Korrekturen.
		if cq, ok := measuredCQFor(request, file); ok {
			single.Quality = qualityFixed
			single.FixedCQ = cq
		}
		args, err := buildConverterArgs(single, eventChannel)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job{label: filepath.Base(file), args: args})
	}
	return jobs, nil
}

// measuredCQFor sucht den für diese Datei bereits gemessenen CQ heraus.
//
// Zwei Bedingungen, beide nötig.
//
// Erstens die Qualitätswahl: "auto" heißt ausdrücklich messen, der LEERE Wert
// heißt "die INI entscheidet" — und da ein Prüflauf überhaupt einen CQ
// geliefert hat, stand sie auf Auto-CQ. Beides ist also in Ordnung. "fixed"
// und "off" sind dagegen Entscheidungen des Nutzers; eine Messung von vorhin
// darf sie nicht überstimmen.
//
// Zweitens muss der Wert in der Skala liegen — ein kaputter Wert aus der
// Oberfläche darf nicht ungeprüft zum Konverter durchgereicht werden.
//
// Pfade werden ohne Rücksicht auf Groß- und Kleinschreibung verglichen: unter
// Windows ist "C:\Filme\A.mkv" dieselbe Datei wie "c:\filme\a.mkv".
func measuredCQFor(request RunRequest, file string) (int, bool) {
	if request.Quality != qualityAuto && request.Quality != "" {
		return 0, false
	}
	if len(request.MeasuredCQ) == 0 {
		return 0, false
	}
	upperBound := maxCQH265
	if request.Codec == codecAV1 {
		upperBound = maxCQAV1
	}
	wanted := strings.ToLower(file)
	for path, cq := range request.MeasuredCQ {
		if strings.ToLower(path) != wanted {
			continue
		}
		if cq < minCQ || cq > upperBound {
			return 0, false
		}
		return cq, true
	}
	return 0, false
}

// buildJoinJobs macht aus einer Join-Ablage einen Auftrag je Bild-Grundlage.
//
// Die Aufteilung selbst steht in joinfiles.go — dort, wo auch entschieden wird,
// welche Datei was ist. Hier wird nur noch für jede Gruppe dieselbe Befehlszeile
// gebaut wie früher für die eine.
//
// Beschriftet wird der Auftrag mit seiner Bild-Grundlage: In der Anzeige steht
// dann der Filmname und nicht fünfmal "join".
func buildJoinJobs(request RunRequest, eventChannel bool) ([]job, error) {
	groups, err := joinGroupPaths(request.Files)
	if err != nil {
		return nil, err
	}

	jobs := make([]job, 0, len(groups))
	for _, group := range groups {
		single := request
		single.Files = group
		args, err := buildConverterArgs(single, eventChannel)
		if err != nil {
			return nil, err
		}
		// Die Bild-Grundlage steht nach joinArgOrder vorn — buildConverterArgs
		// hat sie gerade dorthin sortiert, hier ist die Gruppe noch ungeordnet.
		jobs = append(jobs, job{label: filepath.Base(joinLabelOf(group)), args: args})
	}
	return jobs, nil
}

// joinLabelOf nennt die Bild-Grundlage einer Gruppe. Findet sich keine (was
// joinGroupPaths ausschließt), steht die erste Datei da — eine leere
// Beschriftung wäre in der Warteschlange schlimmer als eine ungenaue.
func joinLabelOf(group []string) string {
	for _, file := range classifyJoinFiles(group) {
		if file.Kind == joinKindVideo {
			return file.Path
		}
	}
	return group[0]
}

// buildConverterArgs setzt die Befehlszeile zusammen.
//
// eventChannel schaltet -json dazu. Kann die vorhandene Programmdatei den
// Datenkanal nicht (ältere Ausgabe), bleibt das Flag weg — sonst würde der
// Konverter es als Dateinamen behandeln und über eine unbekannte Option warnen.
func buildConverterArgs(request RunRequest, eventChannel bool) ([]string, error) {
	if len(request.Files) == 0 {
		return nil, fmt.Errorf("runargs.go: buildConverterArgs: the queue is empty")
	}
	// Zwei Prüfläufe zusammen ergeben keinen Sinn, und der Konverter würde
	// stillschweigend den Balken-Prüflauf gewinnen lassen: der steigt früher
	// aus, die Qualitätssuche käme nie dran. Lieber hier eine klare Meldung als
	// dort ein Ergebnis, auf das niemand gewartet hat.
	if request.Quality == qualityCheck && request.Crop == cropCheck {
		return nil, fmt.Errorf(
			"runargs.go: buildConverterArgs: check the quality or check the black bars, not both in one run")
	}

	var args []string
	if eventChannel {
		args = append(args, "-json")
	}

	// Die Werkzeug-Modi kopieren nur — sie kennen weder Codec noch Qualität.
	// Würde man ihnen "-av1" mitgeben, hielte der Konverter das für einen
	// Dateinamen. Deshalb hier ein eigener, kurzer Weg statt eines Dutzends
	// Ausnahmen weiter unten.
	if request.Mode != "" {
		flag, known := modeFlags[request.Mode]
		if !known {
			return nil, fmt.Errorf("runargs.go: buildConverterArgs: unknown mode %q", request.Mode)
		}
		files := request.Files
		// Zusammenfügen ist der einzige Modus, der eine bestimmte Zusammen-
		// stellung braucht: genau EIN Bild plus mindestens eine Ton- oder
		// Untertiteldatei, Bild zuerst (die Begründung steht bei joinArgOrder).
		if needsJoinOrder(request.Mode) {
			ordered, err := joinArgOrder(files)
			if err != nil {
				return nil, err
			}
			files = ordered
		}
		return append(append(args, flag), files...), nil
	}

	// Wie bei cropOff: "h265" ist ein ausdrückliches "H.265, egal was in der
	// INI steht". Ohne den Gegenschalter gewönne encoder=av1 aus der Datei
	// gegen die Auswahl im Fenster.
	switch request.Codec {
	case codecAV1:
		args = append(args, "-av1")
	case codecH265:
		// Nur mit den Gegenschaltern: Eine exe vor NVENCForge 1.22.0 kennt
		// "-h265" nicht und würde es als unbekannte Option anmeckern. Und ohne
		// codec=av1 in der INI (1.23.0) gibt es ohnehin nichts zu überstimmen.
		if request.CounterFlags {
			args = append(args, "-h265")
		}
	}
	if request.Encoder == encoderCPU {
		args = append(args, "-cpu")
	} else if request.Encoder == encoderNvidia && request.CounterFlags {
		args = append(args, "-gpu")
	}
	if request.Container == containerMP4 {
		args = append(args, "-mp4")
	} else if request.Container == containerMKV && request.CounterFlags {
		args = append(args, "-mkv")
	}
	if request.Resolution == resolutionOriginal {
		args = append(args, "-original")
	} else if request.Resolution == resolutionDownscale && request.CounterFlags {
		args = append(args, "-downscale")
	}
	if request.Audio == audioCopy {
		args = append(args, "-copyaudio")
	} else if request.Audio == audioAAC && request.CounterFlags {
		args = append(args, "-aac")
	}
	if request.BitDepth == bitDepth8 {
		args = append(args, "-8bit")
	} else if request.BitDepth == bitDepth10 && request.CounterFlags {
		args = append(args, "-10bit")
	}
	// Der Prüflauf schließt das Schneiden aus: "-cropcheck" legt nur das
	// Kontrollbild an. Beide Schalter zusammen zu schicken wäre widersprüchlich.
	//
	// cropOff schickt ausdrücklich "-nocrop", statt sich aufs Weglassen zu
	// verlassen: steht autoCrop=true in der INI, würde der Konverter sonst
	// schneiden, obwohl das Kästchen im Fenster leer ist. Ein Bedienelement,
	// das nur in eine Richtung wirkt, ist schlimmer als gar keines.
	//
	// Gesetzt wird cropOff erst in app.go und nur, wenn die vorhandene
	// Programmdatei den Schalter überhaupt kennt — eine ältere würde ihn als
	// unbekannte Option anmeckern.
	switch request.Crop {
	case cropCheck:
		args = append(args, "-cropcheck")
	case cropOn:
		args = append(args, "-crop")
	case cropOff:
		args = append(args, "-nocrop")
	}

	qualityArgs, err := buildQualityArgs(request)
	if err != nil {
		return nil, err
	}
	args = append(args, qualityArgs...)

	if request.MaxBitrate != 0 {
		if request.MaxBitrate < minBitrateKbps || request.MaxBitrate > maxBitrateKbps {
			return nil, fmt.Errorf(
				"runargs.go: buildConverterArgs: max bitrate must be between %d and %d kbit/s, got %d",
				minBitrateKbps, maxBitrateKbps, request.MaxBitrate)
		}
		args = append(args, fmt.Sprintf("-%d", request.MaxBitrate))
	}
	if request.KeepSource {
		args = append(args, "-keep")
	} else if request.CounterFlags {
		args = append(args, "-nokeep")
	}

	// Hier fehlt mit Absicht "-shutdown". Der Schalter fährt den Rechner
	// herunter, sobald DIESER Aufruf fertig ist — und dieses Fenster ruft den
	// Konverter je Datei einmal auf. Bis v1.0.1 schaltete deshalb die erste
	// fertige Datei den Rechner ab und riss den Rest des Stapels mit.
	// Zuständig ist jetzt shutdown.go.
	//
	// Weglassen allein genügt aber nicht: autoShutdown=true in der INI richtet
	// denselben Schaden an, ohne dass je ein Schalter geschickt wurde. Kennt
	// die Programmdatei "-noshutdown", wird das Abschalten deshalb ausdrücklich
	// abgeschaltet.
	if request.SuppressConverterShutdown {
		args = append(args, "-noshutdown")
	}

	return append(args, request.Files...), nil
}

// buildQualityArgs entscheidet über die Qualitätswahl.
//
// Getrennt, weil hier als Einzigem etwas geprüft werden muss: Ein CQ außerhalb
// der Skala würde der Konverter zwar abfangen, aber erst nachdem der Lauf
// gestartet ist — die Meldung gehört vorher ins Fenster.
func buildQualityArgs(request RunRequest) ([]string, error) {
	switch request.Quality {
	case qualityAuto:
		return []string{"-autocq"}, nil
	case qualityOff:
		return []string{"-noautocq"}, nil
	case qualityCheck:
		// "-cqcheck" schaltet die Suche selbst ein, "-autocq" wäre doppelt.
		return []string{"-cqcheck"}, nil
	case qualityFixed:
		upperBound := maxCQH265
		if request.Codec == codecAV1 {
			upperBound = maxCQAV1
		}
		if request.FixedCQ < minCQ || request.FixedCQ > upperBound {
			return nil, fmt.Errorf(
				"runargs.go: buildQualityArgs: CQ must be between %d and %d for this codec, got %d",
				minCQ, upperBound, request.FixedCQ)
		}
		return []string{"-cq", fmt.Sprintf("%d", request.FixedCQ)}, nil
	default:
		// Leer oder unbekannt: nichts übergeben, es gilt die INI.
		return nil, nil
	}
}
