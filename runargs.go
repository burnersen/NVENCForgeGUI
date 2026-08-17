// runargs.go — aus den Schaltern der Oberfläche wird die Befehlszeile.
//
// Eine eigene Datei, weil das die Stelle ist, an der die Oberfläche und der
// Konverter sich berühren: Jeder Schalter hier entspricht genau einem
// dokumentierten Parameter (Help.go). Steht ein Wert nicht drin, wird auch
// nichts übergeben — dann gilt die Einstellung aus der INI, und das ist die
// Vorgabe, die der Nutzer erwartet.
package main

import "fmt"

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

// RunRequest ist das, was die Oberfläche für einen Lauf schickt.
type RunRequest struct {
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
