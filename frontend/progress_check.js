// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// progress_check.js — checks the progress display without converting anything.
//
// Run it with:  node frontend\progress_check.js
//
// A file's percentage comes from ffmpeg's timestamps against the duration the
// container reports, so the last progress event usually stops just short of
// 100 %. What fills the bar is the result event, and that is what is checked
// here — together with the overall bar, which must never count a file twice.
//
// Every area has its own display now, so every event carries the slot it came
// from: 1..3 convert, 5 split, 6 join. An event landing in the wrong area is
// exactly the kind of fault these checks have to catch.
const { loadGui, createChecker } = require("./check_harness");

const { gui, element } = loadGui();
const { check, contains, finish } = createChecker();

const convert = gui.areaOf("convert");

// Each converter has its own lane, so the file bar is read from the slot the
// events came from. Slot 1 unless a check says otherwise.
const laneOf = (slot) => gui.areaOfSlot(slot || 1).slots[slot || 1];
const fileBar = (slot) => {
  const lane = laneOf(slot);
  if (!lane || !lane.known) return "—";
  return lane.pct.toFixed(1) + " %";
};
const overallBar = (area) => element((area || "convert") + "-pct").textContent;

const entry = (name, sizeMB) =>
  ({ path: "X:\\test\\" + name, name, folder: "X:\\test", sizeMB, status: "", note: "" });

// The run event clears the queue notes, so the queue is restored afterwards —
// in the real window the queue is filled by the user, not by an event.
// resetProgress is what the start button does for the whole batch; the run
// events themselves must NOT clear anything, or the second converter would
// wipe out the first one's lane.
function startRun(queue) {
  convert.queue = queue;
  gui.resetProgress(convert);
  // The overall bar is worked out from the files of THIS batch, so the batch
  // has to be named — that is what the start button does.
  gui.startBatch(convert, queue.map((e) => e.path));
  convert.parallel = 1;
  gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvenc", files: queue.length, version: "test" });
  convert.queue = queue;
  convert.totalMB = queue.reduce((sum, e) => sum + e.sizeMB, 0);
}
const fileEvent = (index, total, e) =>
  ({ ev: "file", slot: 1, index, total, name: e.name, path: e.path, in_mb: e.sizeMB });
const resultEvent = (index, status, e, extra = {}) =>
  Object.assign({ ev: "result", slot: 1, index, status, name: e.name, in_mb: e.sizeMB, out_mb: 40, saved_mb: 60, saved_pct: 60 }, extra);

console.log("\n=== one file whose progress stops at 99.1 % ===");
let queue = [entry("film.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 50, eta: "0:30", speed: "2.0x", fps: "120", bitrate: "4000k", est_mb: 40 });
check("half way                  ", fileBar(), "50.0 %");
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 99.1, eta: "0:00", speed: "2.0x", fps: "120", bitrate: "4000k", est_mb: 40 });
check("last progress event       ", fileBar(), "99.1 %");
gui.onConverterEvent(resultEvent(1, "success", queue[0]));
check("file bar after the result ", fileBar(), "100.0 %");
check("overall after the result  ", overallBar(), "100.0 %");
gui.onConverterEvent({ ev: "summary", slot: 1, files: 1, success: 1, skipped: 0, failed: 0, saved_mb: 60, elapsed_sec: 65 });
check("file bar after the summary", fileBar(), "100.0 %");

console.log("\n=== two files: the overall bar must not count a file twice ===");
queue = [entry("a.mkv", 100), entry("b.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 2, queue[0]));
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 99.2, est_mb: 40 });
gui.onConverterEvent(resultEvent(1, "success", queue[0]));
check("file 1 done -> file bar   ", fileBar(), "100.0 %");
check("file 1 done -> overall    ", overallBar(), "50.0 %");
gui.onConverterEvent(fileEvent(2, 2, queue[1]));
check("file 2 starts -> file bar ", fileBar(), "0.0 %");
check("file 2 starts -> overall  ", overallBar(), "50.0 %");
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 50, est_mb: 40 });
check("file 2 half -> overall    ", overallBar(), "75.0 %");
gui.onConverterEvent(resultEvent(2, "success", queue[1]));
check("file 2 done -> overall    ", overallBar(), "100.0 %");

console.log("\n=== failed and stopped runs must NOT jump to 100 % ===");
queue = [entry("broken.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 42, est_mb: 20 });
gui.onConverterEvent(resultEvent(1, "failed", queue[0], { out_mb: 0, saved_mb: 0, saved_pct: 0, message: "encoder error" }));
check("failed -> bar stays       ", fileBar(), "42.0 %");

queue = [entry("stopped.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 27.3, est_mb: 12 });
gui.onConverterEvent(resultEvent(1, "preview", queue[0], { out_mb: 0, saved_mb: 0, saved_pct: 0 }));
check("stopped -> bar stays      ", fileBar(), "27.3 %");

console.log("\n=== a skipped file counts as done ===");
queue = [entry("already-h265.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent(resultEvent(1, "skipped", queue[0], { in_mb: 0, out_mb: 0, saved_mb: 0, saved_pct: 0 }));
check("skipped -> file bar       ", fileBar(), "100.0 %");
check("skipped -> overall        ", overallBar(), "100.0 %");

// The tool modes (davinci, split, join) send NO result event at all, so nothing
// ever filled the file bar: it kept the last percentage ffmpeg reported — 95.6 %
// on a finished join, measured 2026-08-18 — while the overall bar sat at 100 %.
// That read as "something was left undone" on a job that had gone through.
console.log("\n=== a tool run has no result event: the summary closes the bar ===");
const joinSummary = { ev: "summary", slot: 6, files: 1, success: 0, skipped: 0, failed: 0, saved_mb: 0, elapsed_sec: 12 };
gui.onConverterEvent({ ev: "run", slot: 6, mode: "join", version: "test" });
gui.onConverterEvent({ ev: "file", slot: 6, index: 1, total: 1, name: "film.NoSound.mkv", path: "X:\\test\\film.NoSound.mkv", in_mb: 14 });
gui.onConverterEvent({ ev: "progress", slot: 6, pct: 95.6, est_mb: 15 });
check("last progress falls short ", fileBar(6), "95.6 %");
gui.onConverterEvent(joinSummary);
check("summary closes the bar    ", fileBar(6), "100.0 %");

// A run that was stopped keeps its last value: there the rest really was not
// written, and a full bar would claim otherwise.
const splitSummary = { ev: "summary", slot: 5, files: 1, success: 0, skipped: 0, failed: 0, saved_mb: 0, elapsed_sec: 12 };
gui.onConverterEvent({ ev: "run", slot: 5, mode: "split", version: "test" });
gui.onConverterEvent({ ev: "file", slot: 5, index: 1, total: 1, name: "film.mkv", path: "X:\\test\\film.mkv", in_mb: 14 });
gui.onConverterEvent({ ev: "progress", slot: 5, pct: 31.5, est_mb: 5 });
gui.areaOf("split").stopping = true;
gui.onConverterEvent(splitSummary);
check("stopped tool run stays    ", fileBar(5), "31.5 %");
gui.areaOf("split").stopping = false;

// Splitting and joining must not have drawn a single line into the convert
// area on the way — that separation is what the slot numbers are for.
check("split stayed in its area  ", Object.keys(gui.areaOf("split").slots).join(","), "5");
check("join stayed in its area   ", Object.keys(gui.areaOf("join").slots).join(","), "6");

console.log("\n=== a finished run clears its own list ===");
// A list left standing after a finished run is an invitation to press Start
// again and convert the lot a second time.
const idleAreas = () => ({
  convert: { active: 0, pending: 0, limit: 2 },
  split: { active: 0, pending: 0, limit: 1 },
  join: { active: 0, pending: 0, limit: 1 },
  watch: { active: 0, pending: 0, limit: 1 }
});

function afterRun(entries, opts) {
  convert.queue = entries;
  convert.stopping = (opts && opts.stopped) || false;
  convert.trouble = (opts && opts.trouble) || false;
  convert.tally = { files: entries.length, success: entries.length, skipped: 0,
                    failed: (opts && opts.failed) || 0, savedMB: 100, seconds: 30 };
  convert.runMode = "convert";
  convert.running = true;
  convert.finished = false;
  gui.onQueueState({ areas: idleAreas(), totalLimit: 3 });
  return convert.queue.length;
}

const allDone = [
  { path: "C:\\a.mkv", name: "a.mkv", sizeMB: 10, status: "success", note: "done", finished: true },
  { path: "C:\\b.mkv", name: "b.mkv", sizeMB: 10, status: "success", note: "done", finished: true }
];
check("a clean run empties the list ", afterRun(allDone.map((e) => Object.assign({}, e))), 0);
// The result itself has to survive - otherwise nobody can see what happened.
contains("the summary stays           ", element("convert-summary").textContent, "converted");

// What the user asked for: once a run is over the bars go away and the result
// line stays — marked, because it is now the only thing left of that run.
console.log("\n=== a finished run leaves only its result line ===");
check("the lanes are cleared       ", Object.keys(convert.slots).length, 0);
check("the overall bar is put away ", element("convert-barline").hidden, true);
check("the result line stands out  ", element("convert-summary").className, "summary final");
check("and it is visible           ", element("convert-summary").style.display, "block");
// A new run starts from a clean display: bar back, old result line gone.
gui.resetProgress(convert);
check("a new run brings the bar back", element("convert-barline").hidden, false);
check("and clears the old result    ", element("convert-summary").style.display, "none");

// A stopped run keeps its list: it is the one place that still says which file
// it was, and which ones never got their turn.
const stopped = allDone.map((e) => Object.assign({}, e));
check("a stopped run keeps it      ", afterRun(stopped, { stopped: true }), 2);

// So does a run with a file that did not make it.
const withFailure = [
  { path: "C:\\a.mkv", name: "a.mkv", sizeMB: 10, status: "success", note: "done", finished: true },
  { path: "C:\\b.mkv", name: "b.mkv", sizeMB: 10, status: "failed", note: "failed", finished: true }
];
check("a failed file keeps it      ", afterRun(withFailure, { failed: 1 }), 2);

// And one whose file never reported at all - the quiet kind of failure.
const silent = [
  { path: "C:\\a.mkv", name: "a.mkv", sizeMB: 10, status: "", note: "no result reported", finished: true }
];
check("a silent one keeps it too   ", afterRun(silent), 1);

// The areas that report no result per file (splitting, joining) would have
// looked like a clean run and had their list cleared under them. A bad exit
// code is remembered for exactly that reason.
const troubled = allDone.map((e) => Object.assign({}, e));
check("a bad exit code keeps it    ", afterRun(troubled, { trouble: true }), 2);

finish();
