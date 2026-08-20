// parallel_check.js — several converters at once, without a browser.
//
// Every mistake in here is one you only notice after the fact: a progress bar
// that shows another converter's file, a result booked onto the wrong queue
// entry, an answer sent to the converter that did not ask, or a summary that
// counts one file and forgets the other three.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls } = loadGui();
const checker = createChecker();

// Every area runs on its own, so every check has to say WHICH one it means.
// Most of them are about converting by hand — that is what runs several at a
// time in the first place.
const convert = gui.areaOf("convert");
const split = gui.areaOf("split");

// idle builds a dispatcher report in which every area is quiet, except the
// ones a check names. Listing all four matters: an area left out would look
// like it had just finished, and finishing is something the window acts on.
function idle(busy) {
  const areas = {};
  gui.AREA_NAMES.forEach((name) => { areas[name] = { active: 0, pending: 0, limit: 1 }; });
  return Object.assign(areas, busy || {});
}

const entry = (name, sizeMB) =>
  ({ path: "X:\\v\\" + name, name, folder: "X:\\v", sizeMB, status: "", note: "" });

function startBatch(queue, parallel) {
  gui.resetProgress(convert);
  convert.queue = queue;
  convert.totalMB = queue.reduce((sum, e) => sum + e.sizeMB, 0);
  convert.parallel = parallel;
  convert.running = true;
  gui.startBatch(convert, queue.map((e) => e.path));
}

console.log("\nThe setting is on the Convert page and offers 1 to 3");
checker.check("the field exists", html.includes('id="opt-parallel"'), true);
checker.check("three choices", (html.match(/<option value="[123]"/g) || []).length >= 3, true);
// Two is the measured sweet spot (72 s → 52 s with four clips); three brought
// nothing more, so it may be chosen but is not the default.
checker.contains("two is preselected", html, '<option value="2" selected>');

console.log("\nOnly converting runs several at a time");
gui.showPage("convert");
element("opt-parallel").value = "3";
checker.check("converting uses the setting", gui.collectRequest(convert).parallel, 3);
// Taking apart and joining copy instead of encoding and ask about tracks —
// there is nothing to win and a second dialog to lose. Both routes of the
// Split page are checked: they are one page now, but still two jobs.
gui.showPage("split");
element("split-mode").value = "split";
gui.applySplitMode();
checker.check("splitting stays single", gui.collectRequest(split).parallel, 1);
element("split-mode").value = "davinci";
gui.applySplitMode();
checker.check("the Resolve route stays single", gui.collectRequest(split).parallel, 1);
gui.showPage("join");
checker.check("joining stays single", gui.collectRequest(gui.areaOf("join")).parallel, 1);
gui.showPage("convert");
element("opt-parallel").value = "2";

console.log("\nEach converter has its own line and its own file");
const queue = [entry("a.mkv", 100), entry("b.mkv", 100), entry("c.mkv", 200)];
startBatch(queue, 2);
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "run", slot: 2, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "file", slot: 1, index: 1, total: 1, name: "a.mkv", path: "X:\\v\\a.mkv", in_mb: 100 });
gui.onConverterEvent({ ev: "file", slot: 2, index: 1, total: 1, name: "b.mkv", path: "X:\\v\\b.mkv", in_mb: 100 });
checker.check("slot 1 has its file", convert.slots[1].name, "a.mkv");
checker.check("slot 2 has its own", convert.slots[2].name, "b.mkv");
checker.check("both queue entries are working", queue[0].status + "/" + queue[1].status, "running/running");
checker.check("the third one waits", queue[2].status, "");

console.log("\nProgress from one converter does not move the other");
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 80, speed: "2.0x", eta: "0:10", est_mb: 40 });
checker.check("slot 1 moved", convert.slots[1].pct, 80);
checker.check("slot 2 did not", convert.slots[2].pct, 0);
// 80 % of a 100 MB file out of 400 MB in total = 20 %.
checker.check("the overall bar adds them up", element("convert-pct").textContent, "20.0 %");
gui.onConverterEvent({ ev: "progress", slot: 2, pct: 50, speed: "1.8x", eta: "0:20", est_mb: 30 });
checker.check("now both count", element("convert-pct").textContent, "32.5 %");

console.log("\nA result is booked onto the file that produced it");
// Both converters count their files from 1. Without the slot in the key, the
// result of the second one would land on the first one's file — the queue would
// show the wrong size saved on the wrong line.
gui.onConverterEvent({
  ev: "result", slot: 2, index: 1, status: "success",
  name: "b.mkv", in_mb: 100, out_mb: 30, saved_mb: 70, saved_pct: 70
});
checker.contains("b.mkv is done", queue[1].note, "70 % smaller");
checker.check("a.mkv is untouched", queue[0].status, "running");
checker.check("its lane is full", convert.slots[2].pct, 100);
checker.check("the other lane is not", convert.slots[1].pct, 80);

// …and the other way round: the result of slot 1 must find slot 1's file, even
// though both converters call their file number 1.
gui.onConverterEvent({
  ev: "result", slot: 1, index: 1, status: "success",
  name: "a.mkv", in_mb: 100, out_mb: 40, saved_mb: 60, saved_pct: 60
});
checker.contains("a.mkv got its own result", queue[0].note, "60 % smaller");
checker.contains("and b.mkv kept its own", queue[1].note, "70 % smaller");

console.log("\nThe balance counts every converter, not just the last one");
gui.onConverterEvent({ ev: "summary", slot: 1, files: 1, success: 1, skipped: 0, failed: 0, saved_mb: 60, elapsed_sec: 30 });
gui.onConverterEvent({ ev: "summary", slot: 2, files: 1, success: 1, skipped: 0, failed: 0, saved_mb: 70, elapsed_sec: 40 });
gui.onQueueState({ areas: idle({ convert: { active: 0, pending: 0, limit: 2 } }), totalLimit: 3 });
const summary = element("convert-summary").textContent;
checker.contains("both files counted", summary, "2 file(s)");
checker.contains("both successes", summary, "2 converted");
checker.contains("savings added up", summary, "130.0 MB");
// The wall clock is shorter than the times added up, so it must not pretend to
// be a stopwatch.
checker.contains("honest about the time", summary, "converter time");

console.log("\nThe overall bar never falls back — the run from 2026-08-18");
// The user's report: two files, one converter each. When the first one finished,
// the bar JUMPED BACK, and with both of them done it stood at 44.5 %. The old
// version counted megabytes as they went by, so a file dropped out of the sum
// the moment its converter closed. Read from the queue instead, a finished file
// stays finished.
const two = [entry("gross.mkv", 432), entry("klein.mkv", 271)];
startBatch(two, 2);
const seen = [];
const overall = () => {
  const text = element("convert-pct").textContent;
  const value = text === "—" ? 0 : parseFloat(text);
  seen.push(value);
  return value;
};
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "run", slot: 2, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "file", slot: 1, index: 1, total: 1, name: "gross.mkv", path: two[0].path, in_mb: 432 });
gui.onConverterEvent({ ev: "file", slot: 2, index: 1, total: 1, name: "klein.mkv", path: two[1].path, in_mb: 271 });
overall();
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 40 });
gui.onConverterEvent({ ev: "progress", slot: 2, pct: 70 });
overall();
// The small one finishes first and its converter closes.
gui.onConverterEvent({ ev: "result", slot: 2, index: 1, status: "success", name: "klein.mkv", in_mb: 271, out_mb: 151, saved_mb: 120, saved_pct: 44 });
gui.onConverterEvent({ ev: "summary", slot: 2, files: 1, success: 1, skipped: 0, failed: 0, saved_mb: 120, elapsed_sec: 92 });
gui.onRunState({ running: false, exitCode: 0, slot: 2 });
const afterFirst = overall();
// 271 of 703 MB are done, plus 40 % of the big one — and never less than the
// share of the file that is finished.
checker.check("the finished file is counted in full", afterFirst >= (271 / 703) * 100, true);

gui.onConverterEvent({ ev: "progress", slot: 1, pct: 90 });
overall();
gui.onConverterEvent({ ev: "result", slot: 1, index: 1, status: "success", name: "gross.mkv", in_mb: 432, out_mb: 226, saved_mb: 206, saved_pct: 48 });
gui.onConverterEvent({ ev: "summary", slot: 1, files: 1, success: 1, skipped: 0, failed: 0, saved_mb: 206, elapsed_sec: 149 });
gui.onRunState({ running: false, exitCode: 0, slot: 1 });
gui.onQueueState({ areas: idle({ convert: { active: 0, pending: 0, limit: 2 } }), totalLimit: 3 });
const atTheEnd = overall();
checker.check("everything done means 100 %", atTheEnd, 100);

let fellBack = false;
for (let i = 1; i < seen.length; i++) if (seen[i] < seen[i - 1] - 0.01) fellBack = true;
checker.check("it only ever went up", fellBack, false);
checker.check("(the readings)", seen.join(" → "), seen.join(" → "));

console.log("\nA question goes back to the converter that asked");
convert.answerAll = false;
gui.state.questions = [];
calls.answers.length = 0;
calls.answerSlots.length = 0;
gui.onQuestion({ ev: "question", slot: 2, kind: "tracks", file: "X:\\v\\b.mkv", options: [{ n: 1, label: "Audio ger" }, { n: 2, label: "Sub eng" }] });
checker.check("the dialog is up", element("ask").hidden, false);
checker.contains("and says who is waiting", element("ask-slot").textContent, "converter 2");
// A second converter asking must not overwrite the first dialog — both are
// standing still until they get their line.
gui.onQuestion({ ev: "question", slot: 1, kind: "tracks", file: "X:\\v\\a.mkv", options: [{ n: 1, label: "Audio ger" }] });
checker.check("the second question waits", gui.state.questions.length, 2);
checker.check("the first one is still shown", element("ask-file").textContent, "X:\\v\\b.mkv");

gui.sendAnswer("");
checker.check("the answer went to slot 2", calls.answerSlots[0], 2);

console.log("\nOne converter finishing does not end the run");
startBatch([entry("a.mkv", 100), entry("b.mkv", 100)], 2);
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "run", slot: 2, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onRunState({ running: false, exitCode: 0, slot: 1 });
checker.check("still running", convert.running, true);
// Only the dispatcher knows whether anything is left — a slot going quiet says
// nothing about the jobs still waiting for a free one.
gui.onQueueState({ areas: idle({ convert: { active: 1, pending: 3, limit: 2 } }), totalLimit: 3 });
checker.check("and it stays that way while jobs wait", convert.running, true);
gui.onQueueState({ areas: idle({ convert: { active: 0, pending: 0, limit: 2 } }), totalLimit: 3 });
checker.check("now it is over", convert.running, false);

console.log("\nOne converter can be stopped without taking the batch down");
const three = [entry("eins.mkv", 100), entry("zwei.mkv", 100), entry("drei.mkv", 100)];
startBatch(three, 2);
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "run", slot: 2, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "file", slot: 1, index: 1, total: 1, name: "eins.mkv", path: three[0].path, in_mb: 100 });
gui.onConverterEvent({ ev: "file", slot: 2, index: 1, total: 1, name: "zwei.mkv", path: three[1].path, in_mb: 100 });
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 30 });
gui.onConverterEvent({ ev: "progress", slot: 2, pct: 60 });
calls.stops.length = 0;
gui.stopSlot(2);
checker.check("only that one is asked to stop", calls.stops.join(","), "2");
checker.check("and it says so", convert.slots[2].stopping, true);
checker.check("the other one is untouched", !!convert.slots[1].stopping, false);
// The big button means "the whole run"; a single slot must not set it, or the
// summary would treat the batch as aborted and leave the bar short.
checker.check("the run is not marked as stopping", convert.stopping, false);
checker.check("and it is still running", convert.running, true);
// The waiting job is untouched: it moves onto the freed slot.
checker.check("nothing was thrown away", three[2].status, "");

// A stopped converter keeps its last percentage — the rest really was not
// encoded, so filling the bar would claim otherwise.
gui.onConverterEvent({ ev: "summary", slot: 2, files: 1, success: 0, skipped: 0, failed: 0, saved_mb: 0, elapsed_sec: 20 });
checker.check("its bar stays where it stopped", convert.slots[2].pct, 60);

// Same thing in a tool run, where the summary is what fills the bar (those
// modes report no result per file). A slot that was stopped by hand must be
// left short even though the whole run was not stopped.
gui.resetProgress(split);
split.running = true;
gui.onConverterEvent({ ev: "run", slot: 5, mode: "split", version: "t" });
gui.onConverterEvent({ ev: "file", slot: 5, index: 1, total: 1, name: "teil.mkv", path: "X:\\v\\teil.mkv", in_mb: 100 });
gui.onConverterEvent({ ev: "progress", slot: 5, pct: 44 });
gui.stopSlot(5);
gui.onConverterEvent({ ev: "summary", slot: 5, files: 1, success: 0, skipped: 0, failed: 0, saved_mb: 0, elapsed_sec: 10 });
checker.check("a stopped tool slot stays short", split.slots[5].pct, 44);
// …while one that ran through is closed off at 100 %.
gui.resetProgress(split);
gui.onConverterEvent({ ev: "run", slot: 5, mode: "split", version: "t" });
gui.onConverterEvent({ ev: "file", slot: 5, index: 1, total: 1, name: "teil.mkv", path: "X:\\v\\teil.mkv", in_mb: 100 });
gui.onConverterEvent({ ev: "progress", slot: 5, pct: 95.6 });
gui.onConverterEvent({ ev: "summary", slot: 5, files: 1, success: 0, skipped: 0, failed: 0, saved_mb: 0, elapsed_sec: 10 });
checker.check("a finished tool slot is closed off", split.slots[5].pct, 100);
// And none of it reached the converting area, whose batch was running at the
// same time.
checker.check("splitting drew nothing into convert", Object.keys(convert.slots).join(","), "1,2");
split.running = false;

// Back to the three-file batch for the checks below.
startBatch(three, 2);
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "file", slot: 1, index: 1, total: 1, name: "eins.mkv", path: three[0].path, in_mb: 100 });

// The big button, by contrast, marks every lane.
calls.stops.length = 0;
gui.stop("convert");
// One area, by name. Stopping your own batch must not take the watched folder
// or a split down with it — they are not on this page at all.
checker.check("it stops this area only", calls.stops.join(","), "convert");
checker.check("the run is marked as stopping", convert.stopping, true);
checker.check("and the running lane too", convert.slots[1].stopping, true);
// A new batch is not a stopped one.
gui.resetProgress(convert);
checker.check("a new batch starts clean", convert.stopping, false);

console.log("\nOnly the files of THIS batch weigh on the bar");
// The watched folder starts just the new arrivals while the finished ones stay
// in the list as a record. Counting those in would leave the bar stuck at a
// fraction while the batch that is actually running is long done.
const withHistory = [entry("alt.mkv", 900), entry("neu.mkv", 100)];
gui.resetProgress(convert);
convert.queue = withHistory;
convert.parallel = 1;
convert.running = true;
withHistory[0].status = "success";       // from an earlier run
withHistory[0].finished = true;
gui.startBatch(convert, [withHistory[1].path]);   // only the new file is this batch
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvenc", version: "t" });
gui.onConverterEvent({ ev: "file", slot: 1, index: 1, total: 1, name: "neu.mkv", path: withHistory[1].path, in_mb: 100 });
gui.onConverterEvent({ ev: "progress", slot: 1, pct: 50 });
// Half of the one file that is running — not 95 % because a 900 MB record sits
// above it in the list.
checker.check("the old entry is left out", element("convert-pct").textContent, "50.0 %");
gui.onConverterEvent({ ev: "result", slot: 1, index: 1, status: "success", name: "neu.mkv", in_mb: 100, out_mb: 50, saved_mb: 50, saved_pct: 50 });
checker.check("and the batch reaches 100 %", element("convert-pct").textContent, "100.0 %");
convert.running = false;

console.log("\nClearing puts the progress display back to idle");
// After a finished run the last state stays on show — right while you are still
// reading it. But once the list or the log is cleared, leaving old bars and a
// stale summary standing would pretend a finished run is still current.
convert.running = false;
gui.clearProgress(convert);
checker.check("no lanes are left", Object.keys(convert.slots).length, 0);
checker.check("the overall bar is blank", element("convert-pct").textContent, "—");
checker.check("the summary is hidden", element("convert-summary").style.display, "none");
// A run that is still going must not be cleared out from under it.
convert.running = true;
convert.slots[1] = { slot: 1, name: "läuft.mkv", entry: null, pct: 50, known: true, stage: "", stats: "", done: false };
gui.clearProgress(convert);
checker.check("a live run is left alone", Object.keys(convert.slots).length, 1);
convert.running = false;

console.log("\nA redraw only takes back its own lines");
// The converter's progress display overwrites itself by taking lines back.
// With two of them writing at once, one redraw would delete the other one's
// output and the log would eat itself.
convert.parallel = 2;
const logbox = element("convert-logbox");

convert.lastLogSlot = 2;
logbox.removed = 0;
gui.log({ text: "slot 2 redrawing", back: 3, slot: 2 });
checker.check("its own redraw takes lines back", logbox.removed, 3);

convert.lastLogSlot = 1;
logbox.removed = 0;
gui.log({ text: "slot 2 after slot 1 wrote", back: 3, slot: 2 });
checker.check("a redraw after someone else takes none", logbox.removed, 0);
checker.check("and the writer is remembered", convert.lastLogSlot, 2);

checker.finish();
