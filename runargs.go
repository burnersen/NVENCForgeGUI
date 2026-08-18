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
)

// Erlaubte Werte der Auswahlfelder. Als Konstanten, damit ein Tippfehler beim
// Vergleich auffällt statt still den Standardfall zu wählen.
const (
	codecAV1 = "av1"

	encoderCPU = "cpu"

	containerMP4 = "mp4"

	resolutionOriginal = "original"

	audioCopy = "copy"

	bitDepth8 = "8"

	qualityAuto  = "auto"
	qualityOff   = "off"
	qualityFixed = "fixed"
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
	Mode       string   `json:"mode"`     // "" = konvertieren, sonst Schlüssel aus modeFlags
	Parallel   int      `json:"parallel"` // gleichzeitige Läufe (1–3), 0 = einer
	Files      []string `json:"files"`
	Codec      string   `json:"codec"`      // "" oder "av1"
	Encoder    string   `json:"encoder"`    // "" oder "cpu"
	Container  string   `json:"container"`  // "" oder "mp4"
	Resolution string   `json:"resolution"` // "" oder "original"
	Audio      string   `json:"audio"`      // "" oder "copy"
	BitDepth   string   `json:"bitDepth"`   // "" oder "8"
	Quality    string   `json:"quality"`    // "", "auto", "off" oder "fixed"
	FixedCQ    int      `json:"fixedCQ"`
	MaxBitrate int      `json:"maxBitrate"` // 0 = Wert aus der INI
	KeepSource bool     `json:"keepSource"`
	Shutdown   bool     `json:"shutdown"`
}

// buildJobs macht aus einer Anfrage die einzelnen Aufträge für den Verteiler.
//
// Beim KONVERTIEREN bekommt jede Datei ihren eigenen Prozess. Nur so können
// mehrere gleichzeitig laufen, ohne dass zwei Konverter sich um dieselbe Datei
// streiten — die Begründung samt Messung steht im Kopf von dispatcher.go.
//
// Die WERKZEUG-Modi bleiben ein einziger Auftrag mit allen Dateien:
//   - Zusammenfügen ist von Natur aus ein Auftrag (ein Video plus Beigaben).
//   - Zerlegen und DaVinci kopieren nur, statt zu rechnen; dort bremst die
//     Festplatte, nicht die Grafikkarte — parallel gewönne man nichts.
//   - Beide fragen nach Spuren. Zwei Dialoge gleichzeitig für zwei Dateien
//     wären eine Zumutung, und eine falsch zugeordnete Antwort zöge die
//     falschen Spuren heraus.
func buildJobs(request RunRequest, eventChannel bool) ([]job, error) {
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
		args, err := buildConverterArgs(single, eventChannel)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job{label: filepath.Base(file), args: args})
	}
	return jobs, nil
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

	if request.Codec == codecAV1 {
		args = append(args, "-av1")
	}
	if request.Encoder == encoderCPU {
		args = append(args, "-cpu")
	}
	if request.Container == containerMP4 {
		args = append(args, "-mp4")
	}
	if request.Resolution == resolutionOriginal {
		args = append(args, "-original")
	}
	if request.Audio == audioCopy {
		args = append(args, "-copyaudio")
	}
	if request.BitDepth == bitDepth8 {
		args = append(args, "-8bit")
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
	}
	if request.Shutdown {
		args = append(args, "-shutdown")
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
