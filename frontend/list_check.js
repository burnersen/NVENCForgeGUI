// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// list_check.js — die Dateiliste: mitrollen, stehen bleiben, Endgröße, streichen.
//
// Aufruf:  node frontend/list_check.js
//
// Fünf Dinge können hier still schiefgehen:
//
//   1. Die Liste wird bei JEDEM Ereignis neu gebaut. Rettet niemand die
//      Rollstellung, springt sie an den Anfang zurück — wer nachsieht, was
//      eine Datei gespart hat, wird beim Lesen weggerissen.
//   2. Das Mitrollen darf nicht mitrollen, während jemand selbst rollt. Und es
//      muss nach der Ruhepause von selbst wieder anspringen, ohne dass gerade
//      ein Ereignis eintrifft.
//   3. Die Liste des Umwandelns bleibt nach dem Lauf stehen — Zerlegen und
//      Zusammenfügen räumen weiter auf. Verwechselt man das, wird eine Datei
//      ein zweites Mal zerlegt.
//   4. Das ✕ während eines Laufs muss die Datei WIRKLICH abmelden. Verschwindet
//      nur die Zeile, wird sie trotzdem umgewandelt — ein Abbruch, der keiner ist.
//   5. Die Endgröße muss in der Zeile ankommen und dort bleiben.
const { loadGui, createChecker } = require("./check_harness");

const { gui, calls, created, setDropReply } = loadGui();
const { check, finish } = createChecker();

// Schrägstriche statt Windows-Trennern: verglichen wird ohnehin nur der
// kleingeschriebene Pfad, und so steht kein Sonderzeichen im Weg.
const FILE_A = "X:/filme/A.mkv";
const FILE_B = "X:/filme/B.mkv";
const FILE_C = "X:/filme/C.mkv";
gui.showConverter({ found: true, eventChannel: true, autoCrop: true, cqCheck: true });
const area = gui.areaOf("convert");
const box = gui.el(area, "queue");

function fillQueue() {
  area.queue = [
    { path: FILE_A, name: "A.mkv", sizeMB: 1000 },
    { path: FILE_B, name: "B.mkv", sizeMB: 1000 },
    { path: FILE_C, name: "C.mkv", sizeMB: 1000 }
  ];
  area.byIndex = {};
  gui.afterQueueChange(area);
}

async function run() {
  console.log("=== die Endgröße kommt in der Zeile an ===");
  fillQueue();
  gui.startBatch(area, [FILE_A, FILE_B, FILE_C]);
  gui.onConverterEvent({ ev: "file", index: 1, total: 3, name: "A.mkv", path: FILE_A, slot: 1 });
  gui.onConverterEvent({
    ev: "result", index: 1, status: "success", name: "A.mkv",
    in_mb: 1000, out_mb: 340, saved_mb: 660, saved_pct: 66, slot: 1
  });
  check("die Endgröße steht am Eintrag", area.queue[0].outMB, 340);
  const arrow = created.filter((node) => node.className === "size-after").pop();
  check("und als eigene Zelle daneben ", arrow && arrow.textContent, " → 340.0 MB");

  console.log("");
  console.log("=== ein Fehlschlag erfindet keine Endgröße ===");
  // out_mb ist dort 0. Eine 0 in der Spalte sähe aus wie eine sagenhaft kleine
  // Datei — schlimmer als gar keine Angabe.
  gui.onConverterEvent({ ev: "file", index: 2, total: 3, name: "B.mkv", path: FILE_B, slot: 1 });
  gui.onConverterEvent({
    ev: "result", index: 2, status: "failed", name: "B.mkv",
    in_mb: 1000, out_mb: 0, saved_mb: 0, saved_pct: 0, slot: 1
  });
  check("kein Wert nach Fehlschlag    ", area.queue[1].outMB, undefined);

  console.log("");
  console.log("=== die Rollstellung überlebt den Neuaufbau ===");
  // Der Fall, um den es geht: jemand rollt gerade selbst durch die Liste,
  // während im Hintergrund Ereignisse eintreffen. Jedes davon baut die Liste
  // neu auf — ohne gerettete Stellung stünde er sofort wieder ganz oben.
  box.dispatch("wheel");
  box.scrollTop = 120;
  gui.renderList(area);
  check("die Liste bleibt, wo sie war ", box.scrollTop, 120);

  console.log("");
  console.log("=== Mitrollen: erst die Ruhepause, dann der Sprung ===");
  // Jetzt ist Datei C an der Reihe — ihre Zeile ist die, der gefolgt werden
  // soll. Sie liegt weit unten und ist aus dem Bild gerutscht.
  gui.onConverterEvent({ ev: "file", index: 3, total: 3, name: "C.mkv", path: FILE_C, slot: 1 });
  box.clientHeight = 100;
  box.scrollTop = 0;
  const runningRow = box.children.find((row) => String(row.className).includes("active"));
  check("es gibt eine laufende Zeile  ", !!runningRow, true);
  if (runningRow) {
    // Der Listenkasten sitzt selbst 300 Pixel tief auf der Seite, die laufende
    // Zeile liegt bei 800 — im Kasten also bei 500.
    //
    // GENAU HIER ging es schief (vom Nutzer am 05.09.2026 gemeldet): Die erste
    // Fassung rechnete mit offsetTop, und das zählt ab dem nächsten
    // positionierten Vorfahren — nicht ab dem Kasten. Die Liste sprang, aber
    // weit an der laufenden Datei vorbei. Deshalb steht in offsetTop hier
    // absichtlich ein falscher Wert: Wer wieder danach greift, wird rot.
    box.rectTop = 300;
    runningRow.rectTop = 800;
    runningRow.offsetHeight = 20;
    runningRow.offsetTop = 9999;
  }
  // Jemand hat gerade selbst gerollt.
  box.dispatch("wheel");
  gui.followQueueBox(box);
  check("nach eigenem Rollen: Ruhe    ", box.scrollTop, 0);

  // Zwanzig Sekunden später — die Uhr wird vorgestellt, statt zu warten.
  const realNow = Date.now;
  Date.now = () => realNow() + gui.FOLLOW_PAUSE_MS + 1000;
  gui.followQueueBox(box);
  // 500 - (100 - 20) / 2 = 460: die laufende Zeile steht mittig im Kasten.
  check("danach holt sie die Zeile    ", box.scrollTop, 460);

  // Steht die Zeile ohnehin im Bild, wird nicht gesprungen: ein Ruck ohne
  // Anlass ist unruhiger als gar keiner. Bei Rollstand 495 liegt die Zeile
  // (Inhaltshöhe 500) fünf Pixel unter der Oberkante des Kastens — im Bild
  // also bei 300 + 5 = 305.
  box.scrollTop = 495;
  if (runningRow) runningRow.rectTop = 305;
  gui.followQueueBox(box);
  check("kein Sprung ohne Anlass      ", box.scrollTop, 495);
  Date.now = realNow;

  console.log("");
  console.log("=== die laufende Datei lässt sich nicht wegklicken ===");
  // Sie ist längst beim Konverter. Verschwände ihre Zeile, sähe das nach einem
  // Abbruch aus, der keiner ist — dafür gibt es den Stopp-Knopf.
  calls.drops.length = 0;
  await gui.removeFromQueue(area, area.queue[2]);
  check("die laufende Zeile bleibt    ", area.queue.length, 3);
  check("und niemand wurde gefragt    ", calls.drops.length, 0);

  console.log("");
  console.log("=== das ✕ meldet eine wartende Datei wirklich ab ===");
  // Frischer Stapel: Datei A läuft, B und C warten. Nur eine wartende Datei
  // kann überhaupt gestrichen werden.
  fillQueue();
  gui.startBatch(area, [FILE_A, FILE_B, FILE_C]);
  gui.onConverterEvent({ ev: "file", index: 1, total: 3, name: "A.mkv", path: FILE_A, slot: 1 });
  area.running = true;
  calls.drops.length = 0;
  setDropReply(true);
  await gui.removeFromQueue(area, area.queue[2]);
  check("die Go-Seite wurde gefragt   ", calls.drops.length, 1);
  check("und zwar für diese Datei     ", calls.drops[0] && calls.drops[0].path, FILE_C);
  check("erst dann geht die Zeile     ", area.queue.length, 2);

  console.log("");
  console.log("=== zu spät heißt: die Zeile bleibt stehen ===");
  // Zwischen Klick und Antwort kann ein Platz frei geworden sein. Dann läuft
  // die Datei schon, und eine verschwundene Zeile wäre eine Lüge.
  fillQueue();
  gui.startBatch(area, [FILE_A, FILE_B, FILE_C]);
  area.running = true;
  calls.drops.length = 0;
  setDropReply(false);
  await gui.removeFromQueue(area, area.queue[2]);
  check("gefragt wurde trotzdem       ", calls.drops.length, 1);
  check("die Zeile bleibt             ", area.queue.length, 3);
  setDropReply(true);

  console.log("");
  console.log("=== ohne Lauf wird nicht gefragt ===");
  area.running = false;
  calls.drops.length = 0;
  await gui.removeFromQueue(area, area.queue[2]);
  check("keine Rückfrage nötig        ", calls.drops.length, 0);
  check("die Zeile ist weg            ", area.queue.length, 2);

  console.log("");
  console.log("=== neue Dateien räumen den alten Lauf weg ===");
  fillQueue();
  area.queue.forEach((entry) => { entry.finished = true; entry.status = "success"; });
  area.running = false;
  gui.addItems([{ path: "X:/filme/Neu.mkv", name: "Neu.mkv", sizeMB: 500 }], area);
  check("nur die neue Datei bleibt    ", area.queue.length, 1);
  check("und zwar die neue            ", area.queue[0].name, "Neu.mkv");

  console.log("");
  console.log("=== während eines Laufs wird NICHTS weggeräumt ===");
  // Wer nachlegt, will nachlegen — und nicht die Warteschlange leeren, die
  // gerade abgearbeitet wird.
  fillQueue();
  area.queue[0].finished = true;
  area.queue[0].status = "success";
  area.running = true;
  gui.addItems([{ path: "X:/filme/Nachgelegt.mkv", name: "Nachgelegt.mkv", sizeMB: 500 }], area);
  check("alles bleibt plus die neue   ", area.queue.length, 4);
  area.running = false;

  console.log("");
  console.log("=== eine noch nie gelaufene Datei geht nie verloren ===");
  fillQueue();
  area.queue[0].finished = true;
  area.queue[0].status = "success";
  gui.dropFinishedEntries(area);
  check("die Wartenden bleiben        ", area.queue.length, 2);

  console.log("");
  console.log("=== nach dem Lauf bleibt die Umwandeln-Liste stehen ===");
  fillQueue();
  gui.startBatch(area, [FILE_A, FILE_B, FILE_C]);
  area.queue.forEach((entry) => { entry.finished = true; entry.status = "success"; });
  area.tally = { files: 3, success: 3, skipped: 0, failed: 0, savedMB: 900, seconds: 60 };
  area.stopping = false;
  area.trouble = false;
  area.checkRun = "";
  area.runMode = "convert";
  gui.finishArea(area);
  check("alle drei Zeilen stehen noch ", area.queue.length, 3);

  console.log("");
  console.log("=== Zerlegen räumt weiter auf ===");
  const split = gui.areaOf("split");
  split.queue = [{ path: FILE_A, name: "A.mkv", sizeMB: 1000, status: "success", finished: true }];
  split.batch = new Set([FILE_A.toLowerCase()]);
  split.tally = { files: 1, success: 1, skipped: 0, failed: 0, savedMB: 0, seconds: 5 };
  split.stopping = false;
  split.trouble = false;
  split.runMode = "split";
  gui.finishArea(split);
  check("die Zerlegen-Liste ist leer  ", split.queue.length, 0);

  finish();
}

run();
