// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// davinci_check.js — the track chooser and the DaVinci area, without a browser.
//
// Two things here can go wrong in a way nobody would notice by looking at the
// window: an answer that carries the wrong numbers, and a question that is
// never answered at all. The first hands the user tracks they did not pick,
// the second leaves the converter standing still for good. Both are checked
// against the shipped index.html.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, calls, created, element, setQueryAll } = loadGui();
const checker = createChecker();

// Taking a file apart for Resolve runs in the split area, on its own slot (5).
// The questions and the display therefore have to land there — an answer that
// went to another area would be sent to a converter that never asked.
const split = gui.areaOf("split");
const convert = gui.areaOf("convert");
const SPLIT_SLOT = 5;

// idle: a dispatcher report in which nothing is running anywhere.
function idle() {
  const areas = {};
  gui.AREA_NAMES.forEach((name) => { areas[name] = { active: 0, pending: 0, limit: 1 }; });
  return areas;
}

// tick baut einen Ankreuzkasten, wie ihn der Dialog erzeugt.
const tick = (n) => ({ dataset: { n: String(n) } });

const trackQuestion = {
  ev: "question",
  slot: SPLIT_SLOT,
  kind: "tracks",
  file: "C:\\Videos\\Movie.mkv",
  hint: "Enter = all tracks WITHOUT stereo mix",
  options: [
    { n: 1, label: "Audio  ger  EAC3 6ch" },
    { n: 2, label: "  ↳ Stereo mix of [1]  (extra .stereo.m4a)" },
    { n: 3, label: "Sub    eng  SUBRIP" }
  ]
};

console.log("\nThe chooser opens and names the file");
split.answerAll = false;
gui.onQuestion(trackQuestion);
checker.check("dialog is visible", element("ask").hidden, false);
checker.check("the file is named", element("ask-file").textContent, "C:\\Videos\\Movie.mkv");
checker.contains("stereo mixes are explained", element("ask-hint").textContent, "only when you tick it");

console.log("\nWhat is pre-ticked matches the converter's own default");
// This is the quiet one: a stereo mix that arrives pre-ticked would be written
// on every file without anyone asking for it, and nothing on screen would look
// wrong. The converter itself never makes one unless it is asked to.
created.length = 0;
// The question from just above is still open — questions queue up now, because
// with several converters two of them can ask at once. Clear it first, or this
// one would only be put in line behind it and never drawn.
gui.state.questions = [];
gui.onQuestion(trackQuestion);
const boxes = created.filter((el) => el.type === "checkbox");
checker.check("one box per option", boxes.length, 3);
checker.check("audio track is pre-ticked", boxes[0].checked, true);
checker.check("stereo mix is NOT pre-ticked", boxes[1].checked, false);
checker.check("subtitle is pre-ticked", boxes[2].checked, true);
checker.check("boxes carry their number", String(boxes[2].dataset.n), "3");

console.log("\nStereo mixes are told apart from real tracks");
checker.check("a real track", gui.isExtraOption("Audio  ger  EAC3 6ch"), false);
checker.check("a stereo mix", gui.isExtraOption("  ↳ Stereo mix of [1]  (extra .stereo.m4a)"), true);

console.log("\nThe answer carries the ticked numbers, in the order shown");
setQueryAll("#ask-options input:checked", [tick(1), tick(3)]);
checker.check("selection line", gui.askSelection(), "1,3");
setQueryAll("#ask-options input:checked", []);
checker.check("nothing ticked", gui.askSelection(), "");

console.log("\nEvery button sends exactly one answer");
calls.answers.length = 0;
setQueryAll("#ask-options input:checked", [tick(2)]);
gui.sendAnswer(gui.askSelection());
checker.check("“use selection” sends the numbers", calls.answers.join("|"), "2");
checker.check("and closes the dialog", element("ask").hidden, true);

calls.answers.length = 0;
gui.sendAnswer("");
checker.check("“keep all” sends an empty line", calls.answers.length, 1);
checker.check("which means all tracks", calls.answers[0], "");

console.log("\n“Stop asking” answers by itself, without showing anything");
calls.answers.length = 0;
element("ask").hidden = true;
split.answerAll = true;
gui.onQuestion(trackQuestion);
checker.check("answered straight away", calls.answers.length, 1);
checker.check("with all tracks", calls.answers[0], "");
checker.check("no dialog appeared", element("ask").hidden, true);
split.answerAll = false;

console.log("\nA tool run is told apart from a conversion");
gui.onConverterEvent({ ev: "run", slot: SPLIT_SLOT, mode: "davinci", version: "1.18.0" });
checker.check("davinci is a tool run", gui.isToolRun(split), true);
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvidia", files: 2, version: "1.18.0" });
checker.check("converting is not", gui.isToolRun(convert), false);
// And neither told the other what it was doing — they can run at the same time.
checker.check("and the split area kept its own", gui.isToolRun(split), true);

console.log("\nIn a tool run a file is closed off when the next one starts");
split.queue = [
  { path: "C:\\a.mkv", name: "a.mkv", sizeMB: 10, status: "", note: "queued" },
  { path: "C:\\b.mkv", name: "b.mkv", sizeMB: 10, status: "", note: "queued" }
];
gui.onConverterEvent({ ev: "run", slot: SPLIT_SLOT, mode: "davinci", version: "1.18.0" });
gui.onConverterEvent({ ev: "file", slot: SPLIT_SLOT, index: 1, total: 2, name: "a.mkv", path: "C:\\a.mkv", in_mb: 10 });
checker.check("first file is running", split.queue[0].status, "running");
gui.onConverterEvent({ ev: "file", slot: SPLIT_SLOT, index: 2, total: 2, name: "b.mkv", path: "C:\\b.mkv", in_mb: 10 });
checker.check("first file is closed off", split.queue[0].status, "processed");
checker.contains("and says where to look", split.queue[0].note, "log");
checker.check("no success is claimed", split.queue[0].note.includes("smaller"), false);

// Each converter sends its own summary now, so the balance for the whole batch
// is added up in the window and drawn when the dispatcher reports that nothing
// is left — that is what onQueueState stands for here.
console.log("\nThe summary does not report a tool run as “0 converted”");
gui.onConverterEvent({ ev: "summary", slot: SPLIT_SLOT, files: 2, success: 0, skipped: 0, failed: 0, saved_mb: 0, elapsed_sec: 125 });
split.running = true;
gui.onQueueState({ areas: idle(), totalLimit: 3 });
checker.contains("says what really happened", element("split-summary").textContent, "2 file(s) processed in 2 min 5 s");

// The conversion area writes its own balance, in its own place — the two must
// not overwrite each other, which is what having two of everything is for.
gui.resetProgress(convert);
gui.onConverterEvent({ ev: "run", slot: 1, mode: "convert", codec: "h265", encoder: "nvidia", files: 2, version: "1.18.0" });
gui.onConverterEvent({ ev: "result", slot: 1, index: 1, status: "success", name: "a.mkv", in_mb: 100, out_mb: 40, saved_mb: 60, saved_pct: 60 });
gui.onConverterEvent({ ev: "result", slot: 1, index: 2, status: "success", name: "b.mkv", in_mb: 100, out_mb: 40, saved_mb: 60, saved_pct: 60 });
gui.onConverterEvent({ ev: "summary", slot: 1, files: 2, success: 2, skipped: 0, failed: 0, saved_mb: 120, elapsed_sec: 60 });
convert.running = true;
gui.onQueueState({ areas: idle(), totalLimit: 3 });
checker.contains("a conversion still reports its savings", element("convert-summary").textContent, "2 converted");
checker.contains("and the tool run kept its line", element("split-summary").textContent, "processed");

console.log("\nThe page decides which mode its own start button runs");
gui.showPage("split");
element("split-mode").value = "davinci";
gui.applySplitMode();
checker.check("mode follows the picker", split.mode, "davinci");
checker.check("and the button says so", element("btn-split-start").textContent, "Split for Resolve");
checker.check("the request carries it", gui.collectRequest(split).mode, "davinci");

gui.showPage("settings");
checker.check("settings changes nothing about it", split.mode, "davinci");
gui.showPage("convert");
// Two buttons, two meanings — neither renames the other any more.
checker.check("the split page keeps its route", split.mode, "davinci");
checker.check("converting keeps its own", convert.mode, "");
// Read from the page itself: nothing renames this button any more, so the
// only place its label can come from is the markup.
checker.contains("and its button says Start", html, 'id="btn-convert-start">Start<');

checker.finish();
