// watch_check.js — the watched folder, without a browser.
//
// What can go wrong here is quiet and lasting: a standing order that converts
// the same file again and again, one that switches the machine off after the
// first video, or one that writes its progress over the display of a batch you
// started by hand. None of that shows up until it has already happened.
//
// Since the two areas run side by side, most of this file is about the fence
// between them: the watched folder has its own options, its own log, its own
// slot and its own books, and every check below asks whether something got
// over that fence.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls } = loadGui();
const checker = createChecker();

// The two areas this file is about. The watched folder runs on slot 4, a batch
// started by hand on 1..3 — that number is the whole fence.
const watch = gui.areaOf("watch");
const convert = gui.areaOf("convert");

const found = (name) => ({
  path: "D:\\Downloads\\" + name, name, folder: "D:\\Downloads", sizeMB: 700, missing: false
});

// A run is only ever started through StartRun, so calls.runs is the record of
// what really left the window.
function lastRun() { return calls.runs[calls.runs.length - 1]; }

// allIdle: a dispatcher report in which nothing is running anywhere. Every
// area has to be named — one left out looks like it has just finished, and
// finishing is something the window acts on.
function allIdle() {
  const areas = {};
  gui.AREA_NAMES.forEach((name) => { areas[name] = { active: 0, pending: 0, limit: 1 }; });
  return areas;
}

function reset() {
  calls.runs.length = 0;
  // Both areas are put back to a standing start. They run side by side now,
  // so a leftover "still running" from one check would quietly stop the next
  // one from ever starting.
  // Both areas are put back to a standing start. They run side by side now, so
  // a leftover "still running" in one of them would quietly keep the next
  // check from ever starting anything.
  convert.queue = [];
  convert.running = false;
  watch.running = false;
  watch.stopping = false;
  gui.state.watchPending = [];
  gui.state.converterFound = true;
  gui.state.watch = { folder: "D:\\Downloads", active: true };
}

// pageOf refuses to work on a missing anchor. A slice whose end anchor is gone
// runs to the end of the file and swallows the whole window — these checks
// then pass no matter where anything really sits, which is exactly what
// happened once already when a page was renamed.
function pageOf(id, nextId) {
  const from = html.indexOf('<div id="page-' + id + '"');
  const to = html.indexOf('<div id="page-' + nextId + '"');
  if (from === -1 || to === -1 || to < from) throw new Error("page " + id + " or " + nextId + " is gone");
  return html.slice(from, to);
}
const watchPage = pageOf("watch", "settings");
const convertPage = pageOf("convert", "split");

console.log("\nThe area has a page of its own, with everything on it");
checker.contains("the nav button opens it", html, '<button class="nav-item" data-page="watch">Watch</button>');
checker.check("the folder button", watchPage.includes('id="btn-watch-pick"'), true);
checker.check("a start button", watchPage.includes('id="btn-watch-start"'), true);
checker.check("and a separate stop button", watchPage.includes('id="btn-watch-stop"'), true);
checker.check("its own progress area", watchPage.includes('id="watch-lanes"'), true);
checker.check("its own log", watchPage.includes('id="watch-logbox"'), true);
checker.check("and its own options", watchPage.includes('id="wopt-codec"'), true);
checker.check("it left the convert page", convertPage.includes('id="panel-watch"'), false);
// The shared run panels must not travel here: they belong to the batch, and
// the whole point is that this page shows only its own work.
checker.check("no shared run panels here", watchPage.includes('data-slot="run"'), false);
checker.check("no run button here", watchPage.includes('id="btn-start"'), false);
// No shutdown here at all. A standing order that switches the machine off
// after the first video would never convert the second one.
checker.check("no shutdown option here", watchPage.includes("wopt-shutdown"), false);
// One at a time, so the card stays free for the work started by hand.
checker.check("and nothing to run several at once", watchPage.includes("wopt-parallel"), false);

console.log("\nA run uses THIS page's options, not the Convert page's");
reset();
element("wopt-codec").value = "av1";
element("wopt-quality").value = "fixed";
element("wopt-cq").value = "30";
element("wopt-keep").checked = true;
// Deliberately the opposite on the other page: if the two ever got mixed up,
// these are the values that would turn up in the run.
element("opt-codec").value = "";
element("opt-quality").value = "auto";
element("opt-keep").checked = false;

gui.onWatchFiles([found("neu.mkv")]);
checker.check("a run went off", calls.runs.length, 1);
checker.check("marked as the watched area", lastRun().area, "watch");
checker.check("codec from THIS page", lastRun().codec, "av1");
checker.check("quality from THIS page", lastRun().quality, "fixed");
checker.check("the fixed CQ too", lastRun().fixedCQ, 30);
checker.check("and keeping the source", lastRun().keepSource, true);
checker.check("always one at a time", lastRun().parallel, 1);
checker.check("and never a shutdown", lastRun().shutdown, false);
element("wopt-codec").value = "";
element("wopt-quality").value = "auto";
element("wopt-keep").checked = false;

console.log("\nIts finds stay out of the batch's list");
reset();
gui.onWatchFiles([found("eins.mkv"), found("zwei.mkv")]);
// The queue on the Convert page is what the user dropped in. A folder filling
// it was the very thing that made the two areas impossible to tell apart.
checker.check("the queue stays empty", convert.queue.length, 0);
checker.check("one find went off, one waits", gui.state.watchPending.length, 1);

console.log("\nA batch running by hand no longer holds it up");
reset();
convert.running = true;   // a batch is going on the other page
gui.onWatchFiles([found("waehrenddessen.mkv")]);
checker.check("it starts anyway", calls.runs.length, 1);
checker.check("as the watched area", lastRun().area, "watch");

console.log("\nBut only one of its own at a time");
reset();
gui.onWatchFiles([found("erste.mkv")]);
checker.check("the first one runs", calls.runs.length, 1);
gui.onWatchFiles([found("zweite.mkv")]);
checker.check("the second one waits", calls.runs.length, 1);
checker.check("it is on the waiting list", gui.state.watchPending.length, 1);
// Its own end is what starts the next find — and only its own. A batch
// finishing on the other page must not reach in here.
gui.onQueueState({ areas: { convert: { active: 0, pending: 0, limit: 2 }, watch: { active: 1, pending: 0, limit: 1 } }, totalLimit: 3 });
checker.check("the other area finishing changes nothing", calls.runs.length, 1);
gui.onQueueState({ areas: { convert: { active: 0, pending: 0, limit: 2 }, watch: { active: 0, pending: 0, limit: 1 } }, totalLimit: 3 });
checker.check("its own end starts the next", calls.runs.length, 2);
checker.check("with the file that waited", lastRun().files[0], "D:\\Downloads\\zweite.mkv");
checker.check("and only that one", lastRun().files.length, 1);
checker.check("the waiting list is empty", gui.state.watchPending.length, 0);
gui.onQueueState({ areas: { convert: { active: 0, pending: 0, limit: 2 }, watch: { active: 0, pending: 0, limit: 1 } }, totalLimit: 3 });
checker.check("nothing starts on an empty list", calls.runs.length, 2);

console.log("\nNothing is converted twice");
reset();
watch.running = true;   // something of its own is already going
gui.onWatchFiles([found("doppelt.mkv")]);
gui.onWatchFiles([found("doppelt.mkv")]);
checker.check("the waiting list holds it once", gui.state.watchPending.length, 1);
checker.check("and nothing was started", calls.runs.length, 0);

console.log("\nSwitching off stops what was waiting");
reset();
watch.running = true;
gui.onWatchFiles([found("wartet.mkv")]);
checker.check("it is waiting", gui.state.watchPending.length, 1);
gui.state.watch.active = false;
gui.state.watchPending = [];
gui.onQueueState({ areas: { convert: { active: 0, pending: 0, limit: 2 }, watch: { active: 0, pending: 0, limit: 1 } }, totalLimit: 3 });
checker.check("after switching off nothing starts", calls.runs.length, 0);

console.log("\nIts messages land in ITS log");
// The complaint that started all this: a folder working in the background kept
// writing into the log of the batch you were reading.
const convertLog = element("convert-logbox");
const watchLog = element("watch-logbox");
convertLog.appended = 0;
watchLog.appended = 0;
gui.log({ text: "a line from the watched folder", back: 0, slot: gui.WATCH_SLOT });
checker.check("it writes into its own box", watchLog.appended > 0, true);
checker.check("and not into the batch's log", convertLog.appended > 0, false);
gui.log({ text: "a line from the batch", back: 0, slot: 1 });
checker.check("the batch writes into its own", convertLog.appended > 0, true);

console.log("\nIts balance is its own");
reset();
watch.tally = { files: 0, success: 0, skipped: 0, failed: 0, savedMB: 0, seconds: 0 };
convert.tally = { files: 0, success: 0, skipped: 0, failed: 0, savedMB: 0, seconds: 0 };
gui.onConverterEvent({
  ev: "result", slot: gui.WATCH_SLOT, index: 1, status: "success",
  name: "neu.mkv", in_mb: 700, out_mb: 300, saved_mb: 400, saved_pct: 57
});
checker.check("the watched folder counted it", watch.tally.files, 1);
checker.check("with what it saved", watch.tally.savedMB, 400);
checker.check("the batch counted nothing", convert.tally.files, 0);
checker.contains("and it says so on its own page", element("watch-summary").textContent, "saved");

console.log("\nThe Convert page owns up to sharing the card");
reset();
gui.state.watch.active = true;
element("opt-encoder").value = "";
gui.limitParallelChoice();
checker.contains("it says why", element("parallel-note").textContent, "watched folder");
gui.state.watch.active = false;
gui.limitParallelChoice();
checker.check("and stays quiet when it is off", element("parallel-note").textContent, "");
// The processor mode caps everything at two, watched folder or not.
element("opt-encoder").value = "cpu";
gui.limitParallelChoice();
checker.contains("the processor mode is named too", element("parallel-note").textContent, "processor");
element("opt-encoder").value = "";

console.log("\nShutting down and a standing order still do not go together");
reset();
element("opt-shutdown").checked = true;   // ticked before watching was switched on
gui.updateButtons();
checker.check("the box is out of reach", element("opt-shutdown").disabled, true);
checker.check("and cleared", element("opt-shutdown").checked, false);


console.log("\nClear takes the folder path with it");
// The path says which folder someone was working through. Once they are done
// with it, that is nobody's business - which is why Clear is not just "empty
// the log".
reset();
gui.state.watch = { folder: "D:\\Privat\\Serien", active: false };
watch.tally = { files: 3, success: 3, skipped: 0, failed: 0, savedMB: 900, seconds: 60 };
gui.showWatch(null);
gui.clearWatchArea();
checker.check("the folder is forgotten  ", gui.state.watch.folder, "");
checker.check("the display says so      ", element("watch-folder").textContent, "No folder chosen yet.");
checker.check("the running total is gone", watch.tally.files, 0);
checker.check("and its line with it     ", element("watch-summary").textContent, "");

// While the order is still standing, the path stays: it is still moving
// originals about, and a window that no longer says where would be hiding
// something that is still happening.
reset();
gui.state.watch = { folder: "D:\\Privat\\Serien", active: true };
gui.showWatch(null);
gui.clearWatchArea();
checker.check("while watching it stays  ", gui.state.watch.folder, "D:\\Privat\\Serien");

/* Reported by the user, 2026-08-20: after "stop watching" and then "stop
   this file", the finished video was still sitting in the progress area
   with no way to get rid of it. A standing order that is switched off never
   starts another file, so nothing would ever have replaced that lane. */
console.log("\nA finished file leaves the progress area");
reset();
gui.onWatchFiles([found("fertig.mkv")]);
gui.onConverterEvent({ ev: "run", slot: gui.WATCH_SLOT, mode: "convert", codec: "h265", encoder: "nvidia", files: 1, version: "t" });
gui.onConverterEvent({ ev: "file", slot: gui.WATCH_SLOT, index: 1, total: 1, name: "fertig.mkv", path: "D:\\Downloads\\fertig.mkv", in_mb: 700 });
gui.onConverterEvent({ ev: "result", slot: gui.WATCH_SLOT, index: 1, status: "success", name: "fertig.mkv", in_mb: 700, out_mb: 300, saved_mb: 400, saved_pct: 57 });
checker.check("while it runs it is shown", Object.keys(watch.slots).length, 1);
// Watching is switched off, exactly as the user had it.
gui.state.watch.active = false;
watch.running = true;
gui.onQueueState({ areas: allIdle(), totalLimit: 3 });
checker.check("afterwards the lane is gone", Object.keys(watch.slots).length, 0);
// …but what it did is still readable: the bars are cleared away, the books
// are not.
checker.contains("the running total stays   ", element("watch-summary").textContent, "saved");

/* The log goes to the clipboard line by line. Taken as one block it arrives
   as a single endless line — which is exactly how it turned up when the user
   pasted one into a message. */
console.log("\nCopying the log keeps its lines apart");
calls.clipboard.length = 0;
element("watch-logbox").children.length = 0;
gui.watchNote("erste Zeile");
gui.watchNote("zweite Zeile");
gui.watchNote("dritte Zeile");
gui.copyAreaLog(watch);
checker.check("one copy went out        ", calls.clipboard.length, 1);
checker.check("with a line each         ", (calls.clipboard[0].match(/\n/g) || []).length, 2);
checker.contains("and the text is all there", calls.clipboard[0], "zweite Zeile");

checker.finish();
