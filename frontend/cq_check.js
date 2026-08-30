// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// cq_check.js — the quality check and the CQ shown in the list.
//
// Run it with:  node frontend\cq_check.js
//
// Three things here can go wrong quietly:
//
//   1. The check button must send quality "check". Send "auto" instead and a
//      user who only wanted a number gets the whole queue encoded.
//   2. The CQ arrives as its own event and is written onto the file. The
//      result event lands afterwards and used to overwrite that line with
//      "skipped" - which is the least useful thing to know about a check run.
//   3. A CQ belongs to ONE run. Left standing, the next run would show a
//      number measured under different settings: worse than none, because it
//      looks right.
const { loadGui, createChecker } = require("./check_harness");

const { gui, calls, element } = loadGui();
const { check, finish } = createChecker();

const ableConverter = { found: true, eventChannel: true, autoCrop: true, cqCheck: true };
const oldConverter = { found: true, eventChannel: true, autoCrop: true, cqCheck: false };

const FILE = "X:\\filme\\Film.mkv";
gui.showConverter(ableConverter);
const area = gui.areaOf("convert");
area.queue = [{ path: FILE, name: "Film.mkv", sizeMB: 2000 }];
gui.afterQueueChange(area);

console.log("\n=== the check button asks for a check, not for an encode ===");
element("opt-shutdown").checked = true;
calls.runs.length = 0;
gui.start("convert", "cq");
const checkRun = calls.runs[calls.runs.length - 1];
check("the run asks for a check  ", checkRun && checkRun.quality, "check");
// Nothing was converted, and somebody wants to read the numbers afterwards.
check("no shutdown after a check ", checkRun && checkRun.shutdown, false);

console.log("\n=== a normal start still converts ===");
calls.runs.length = 0;
gui.start("convert");
const normalRun = calls.runs[calls.runs.length - 1];
check("start does not send check ", normalRun && normalRun.quality === "check", false);

console.log("\n=== two check runs never go out together ===");
// The converter would let the black-bar check win and never run the search.
element("opt-crop").checked = false;
calls.runs.length = 0;
gui.start("convert", "cq");
const soloRun = calls.runs[calls.runs.length - 1];
check("crop check is cleared     ", soloRun && soloRun.crop === "check", false);

console.log("\n=== the measured CQ lands on the file ===");
gui.startBatch(area, [FILE]);
gui.onConverterEvent({ ev: "file", index: 1, total: 1, name: "Film.mkv", path: FILE, slot: 1 });
gui.onConverterEvent({ ev: "cq", index: 1, cq: 28, vmaf: 96.642, target: 96.5, note: "verified", slot: 1 });
check("the list shows the pick   ", area.queue[0].cq, 28);
check("with the score beside it  ", area.queue[0].note, "CQ 28 · VMAF 96.6");

console.log("\n=== \"skipped\" must not wipe the answer away ===");
gui.onConverterEvent({
  ev: "result", index: 1, status: "skipped", name: "Film.mkv",
  in_mb: 2000, out_mb: 2000, saved_mb: 0, saved_pct: 0, slot: 1
});
check("the CQ survives the result", area.queue[0].note, "CQ 28 · VMAF 96.6");

console.log("\n=== on a real run both are worth knowing ===");
gui.startBatch(area, [FILE]);
gui.onConverterEvent({ ev: "file", index: 1, total: 1, name: "Film.mkv", path: FILE, slot: 1 });
gui.onConverterEvent({ ev: "cq", index: 1, cq: 30, vmaf: 96.7, target: 96.5, note: "verified", slot: 1 });
gui.onConverterEvent({
  ev: "result", index: 1, status: "success", name: "Film.mkv",
  in_mb: 2000, out_mb: 760, saved_mb: 1240, saved_pct: 62, slot: 1
});
check("saving and CQ side by side", area.queue[0].note, "done, 62 % smaller · CQ 30");

console.log("\n=== a new run clears the old number ===");
gui.startBatch(area, [FILE]);
check("the CQ is gone            ", area.queue[0].cq, undefined);
check("and the line reads queued ", area.queue[0].note, "queued");

console.log("\n=== an older converter gets no check button ===");
gui.showConverter(oldConverter);
gui.afterQueueChange(area);
check("button off on 1.29 and up ", element("btn-convert-cqcheck").disabled, true);
gui.showConverter(ableConverter);
gui.afterQueueChange(area);
check("and usable again on 1.30  ", element("btn-convert-cqcheck").disabled, false);

finish("cq");
