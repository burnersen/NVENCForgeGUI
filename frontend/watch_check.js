// watch_check.js — the watched folder, without a browser.
//
// What can go wrong here is quiet and lasting: a standing order that converts
// the same file again and again, one that switches the machine off after the
// first video, or one that re-encodes with the wrong mode because another page
// happened to be open. None of that shows up until it has already happened.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls } = loadGui();
const checker = createChecker();

const found = (name) => ({
  path: "D:\\Downloads\\" + name, name, folder: "D:\\Downloads", sizeMB: 700, missing: false
});

// A run is only ever started through StartRun, so calls.runs is the record of
// what really left the window.
function lastRun() { return calls.runs[calls.runs.length - 1]; }

function reset() {
  calls.runs.length = 0;
  gui.state.queue = [];
  gui.state.watchPending = [];
  gui.state.running = false;
  gui.state.converterFound = true;
  gui.state.watch = { folder: "D:\\Downloads", active: true };
}

console.log("\nThe area has a page of its own");
// Every slice below is anchored on two things that must both exist. A slice
// whose end anchor is gone silently runs to the end of the file and swallows
// the whole window — these checks then pass no matter where the panel really
// sits, which is exactly what happened when the page they used to be cut
// against was removed.
function pageOf(id, nextId) {
  const from = html.indexOf('<div id="page-' + id + '"');
  const to = html.indexOf('<div id="page-' + nextId + '"');
  if (from === -1 || to === -1 || to < from) throw new Error("page " + id + " or " + nextId + " is gone");
  return html.slice(from, to);
}
const watchPage = pageOf("watch", "settings");
const convertPage = pageOf("convert", "extract");

checker.contains("the nav button opens it", html, '<button class="nav-item" data-page="watch">Watch</button>');
checker.check("the panel is on the watch page", watchPage.includes('id="panel-watch"'), true);
checker.check("with a folder button", watchPage.includes('id="btn-watch-pick"'), true);
checker.check("and its own start button", watchPage.includes('id="btn-watch-toggle"'), true);
checker.check("it left the convert page", convertPage.includes('id="panel-watch"'), false);
checker.check("run panels can still land there", watchPage.includes('data-slot="run"'), true);
// Its own button on purpose: "Start" runs the queue once, watching is a
// standing order. One button cannot honestly mean both, so the page must not
// reach for the run button.
checker.check("the page keeps its hands off the run button", watchPage.includes('id="btn-start"'), false);
checker.check("no greyed-out nav entry is left", /disabled>Watch folder/.test(html), false);

// And the run button really is taken off the page, not merely absent from the
// markup: it travels with the shared panels and would otherwise arrive here.
gui.showPage("watch");
checker.check("no start button while watching", element("btn-start").hidden, true);
checker.check("watch page is the one on show", element("page-watch").hidden, false);
gui.showPage("convert");
checker.check("it is back on the convert page", element("btn-start").hidden, false);
checker.check("watch page is put away", element("page-watch").hidden, true);

console.log("\nNothing happens until a folder is chosen");
reset();
gui.state.watch = { folder: "", active: false };
gui.showWatch(null);
checker.check("the button waits for a folder", element("btn-watch-toggle").disabled, true);
checker.contains("and says so", element("watch-folder").textContent, "No folder chosen");
gui.state.watch.folder = "D:\\Downloads";
gui.showWatch(null);
checker.check("with a folder it can be switched on", element("btn-watch-toggle").disabled, false);
checker.check("the button offers to start", element("btn-watch-toggle").textContent, "Start watching");
gui.state.watch.active = true;
gui.showWatch({ watching: true, folder: "D:\\Downloads" });
checker.check("and to stop once it runs", element("btn-watch-toggle").textContent, "Stop watching");

console.log("\nA found file is converted, whatever page is open");
reset();
gui.showPage("join");          // the join page sets the mode to "join"…
gui.onWatchFiles([found("neu.mkv")]);
checker.check("one run was started", calls.runs.length, 1);
// …but a watched folder converts. Starting it in join mode would copy streams
// around instead of encoding — and nothing on screen would look wrong.
checker.check("it converts, not joins", lastRun().mode, "");
checker.check("with the file that was found", lastRun().files[0], "D:\\Downloads\\neu.mkv");
checker.check("and it is in the queue", gui.state.queue.length, 1);
gui.showPage("convert");

console.log("\nThe options above the panel are the ones that count");
reset();
element("opt-codec").value = "av1";
element("opt-quality").value = "auto";
gui.onWatchFiles([found("neu.mkv")]);
checker.check("codec comes from the page", lastRun().codec, "av1");
checker.check("quality too", lastRun().quality, "auto");
element("opt-codec").value = "";
element("opt-quality").value = "";

console.log("\nShutting down and a standing order do not go together");
// Two things stop it, and both are checked: the box is put out of reach, and
// the run itself refuses to carry the flag. The second one is what counts —
// the machine would switch itself off after the first video, and every one
// after it would never be converted.
reset();
element("opt-shutdown").checked = true;   // ticked before watching was switched on
gui.updateButtons();
checker.check("the box is out of reach", element("opt-shutdown").disabled, true);
checker.check("and cleared", element("opt-shutdown").checked, false);
// Tick it by hand, past the greyed-out box, and start the waiting file
// directly: this is the run itself, with nothing else covering for it.
element("opt-shutdown").checked = true;
gui.state.watchPending = ["D:\\Downloads\\neu.mkv"];
gui.maybeStartWatchRun();
checker.check("the run never carries shutdown", lastRun().shutdown, false);
element("opt-shutdown").checked = false;

console.log("\nNothing is converted twice");
reset();
gui.onWatchFiles([found("neu.mkv")]);
checker.check("first run", calls.runs.length, 1);
gui.state.running = true;
gui.onWatchFiles([found("neu.mkv")]);
checker.check("the same file again starts nothing", calls.runs.length, 1);
checker.check("and does not queue up twice", gui.state.queue.length, 1);
// The watcher reports what it finds; a file that is reported twice must not
// end up on the waiting list twice, or the converter gets it twice in one go.
reset();
gui.state.running = true;
gui.onWatchFiles([found("doppelt.mkv")]);
gui.onWatchFiles([found("doppelt.mkv")]);
checker.check("the waiting list holds it once", gui.state.watchPending.length, 1);

console.log("\nFiles arriving during a run are next in line");
reset();
gui.onWatchFiles([found("erste.mkv")]);
checker.check("the first run went off", calls.runs.length, 1);
gui.state.running = true;
gui.onWatchFiles([found("zweite.mkv")]);
checker.check("the second file waits", calls.runs.length, 1);
checker.check("it is on the waiting list", gui.state.watchPending.length, 1);
// The converter takes his file list at startup, so the waiting ones can only
// go in the next run — which the end of this one has to trigger.
gui.onQueueState({ active: 0, pending: 0, limit: 1 });
checker.check("the end of the run starts them", calls.runs.length, 2);
checker.check("with the file that waited", lastRun().files[0], "D:\\Downloads\\zweite.mkv");
checker.check("and only that one", lastRun().files.length, 1);
checker.check("the waiting list is empty", gui.state.watchPending.length, 0);
// A finished run must not start itself over on an empty list.
gui.onQueueState({ active: 0, pending: 0, limit: 1 });
checker.check("nothing starts on an empty list", calls.runs.length, 2);

console.log("\nSwitching off stops everything that was waiting");
reset();
gui.state.running = true;
gui.onWatchFiles([found("wartet.mkv")]);
checker.check("it is waiting", gui.state.watchPending.length, 1);
toggleOff();
function toggleOff() {
  gui.state.watch.active = false;
  gui.state.watchPending = [];
}
gui.onQueueState({ active: 0, pending: 0, limit: 1 });
checker.check("after switching off nothing starts", calls.runs.length, 0);

checker.finish();
