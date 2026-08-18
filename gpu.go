// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// gpu.go — welche Grafikkarte steckt im Rechner?
//
// Die Oberfläche zeigt die Karte nur an; sie graut nichts aus und blockiert
// nichts. Ob eine Karte wirklich mit den gewünschten Einstellungen umgehen
// kann, entscheidet weiterhin allein die Start-Probe des Konverters (v1.16.0) —
// die fragt die Karte praktisch und nicht nach ihrem Namen.
package main

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// gpuQueryTimeout begrenzt die Abfrage. nvidia-smi antwortet normalerweise in
// weniger als einer Sekunde; hängt der Treiber, darf das Fenster deswegen nicht
// beim Start stehen bleiben.
const gpuQueryTimeout = 5 * time.Second

// winCreateNoWindow verhindert das kurze schwarze Fenster bei Hilfsaufrufen.
// Für den Konverter selbst ist dieses Flag verboten (siehe wincon.go).
const winCreateNoWindow = 0x08000000

// GPUInfo ist das, was die Oberfläche über die Karte anzeigt.
type GPUInfo struct {
	Detected bool   `json:"detected"`
	Name     string `json:"name"`
	Driver   string `json:"driver"`
	MemoryMB int    `json:"memoryMB"`
	Note     string `json:"note"`
}

// queryGPU fragt nvidia-smi nach Name, Treiber und Speicher der ersten Karte.
//
// nvidia-smi gehört zum NVIDIA-Treiber; fehlt es, steckt entweder keine
// NVIDIA-Karte im Rechner oder der Treiber ist nicht installiert. Beides ist
// kein Fehler dieses Programms, deshalb wird es als Hinweis gemeldet und nicht
// als Absturz.
func queryGPU() GPUInfo {
	ctx, cancel := context.WithTimeout(context.Background(), gpuQueryTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,driver_version,memory.total",
		"--format=csv,noheader,nounits")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: winCreateNoWindow}

	out, err := cmd.Output()
	if err != nil {
		return GPUInfo{Note: "No NVIDIA GPU detected (nvidia-smi not available)."}
	}
	return parseGPUQuery(string(out))
}

// parseGPUQuery liest die erste Zeile der nvidia-smi-Ausgabe.
//
// Eigene Funktion, damit sie ohne Grafikkarte testbar ist. Bei mehreren Karten
// zählt die erste — genau die benutzt der Konverter auch.
func parseGPUQuery(raw string) GPUInfo {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 3 {
			continue
		}
		memoryMB, err := strconv.Atoi(strings.TrimSpace(fields[2]))
		if err != nil {
			memoryMB = 0
		}
		return GPUInfo{
			Detected: true,
			Name:     strings.TrimSpace(fields[0]),
			Driver:   strings.TrimSpace(fields[1]),
			MemoryMB: memoryMB,
		}
	}
	return GPUInfo{Note: "nvidia-smi answered, but reported no GPU."}
}
