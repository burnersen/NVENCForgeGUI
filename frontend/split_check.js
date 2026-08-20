// split_check.js — the Split area, without a browser.
//
// The expensive mistake here is silent: if the start button on this page still
// carries the convert mode, a click re-encodes the files instead of taking them
// apart — and nothing on screen looks wrong until the result is there. Since
// the two routes share one page, a second one joined it: the chosen route has
// to reach the converter, or the user gets untouched parts where they asked
// for Resolve-ready ones, and the difference only shows up in DaVinci.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element } = loadGui();
const checker = createChecker();

// Taking files apart is an area of its own now: its own list, its own progress
// area, its own log, its own start button — and its own slot at the converter
// (5), so it can run while a batch is being converted on slot 1.
const split = gui.areaOf("split");
const SPLIT_SLOT = 5;

// splitPage cuts out just this page, so a slot found on some OTHER page
// cannot make the structure checks pass by accident.
const splitPage = html.slice(
  html.indexOf('<div id="page-split"'),
  html.indexOf('<!-- Join has a list of its own')
);

// choose sets the picker and lets the page react, the way a click would.
function choose(mode) {
  element("split-mode").value = mode;
  gui.applySplitMode();
}

console.log("\nOne page, reachable, with both routes on it");
checker.contains("the nav button opens the page", html, '<button class="nav-item" data-page="split">Split</button>');
checker.check("the page itself exists", html.includes('<div id="page-split" hidden>'), true);
// The whole point of the merge: ONE page for taking a file apart, not two
// that differ only in their explanation.
checker.check("no separate DaVinci page left", html.includes('id="page-davinci"'), false);
checker.check("no DaVinci nav button left", /data-page="davinci"/.test(html), false);
checker.check("exactly one page for it", (html.match(/data-page="split"/g) || []).length, 1);
checker.contains("1:1 is offered", splitPage, 'value="split"');
checker.contains("Resolve-ready is offered", splitPage, 'value="davinci"');

console.log("\nThe page owns its list, its progress and its log");
// Nothing travels between pages any anymore: this page has its own, so a batch
// being converted next door cannot draw a single line into it.
checker.check("its own list", splitPage.includes('id="split-queue"'), true);
checker.check("its own progress", splitPage.includes('id="split-lanes"'), true);
checker.check("its own log", splitPage.includes('id="split-logbox"'), true);
checker.check("its own start button", splitPage.includes('id="btn-split-start"'), true);
checker.check("and no wandering panels left", /data-slot="/.test(html), false);
// The convert area's own elements must not turn up on this page — that would
// be the old shared display in a new coat.
checker.check("nothing of the convert area", /id="convert-/.test(splitPage), false);

console.log("\nThe chosen route reaches the converter");
gui.showPage("split");
choose("split");
checker.check("mode follows the picker", split.mode, "split");
checker.check("and the button says so", element("btn-split-start").textContent, "Split losslessly");
checker.check("the request carries it", gui.collectRequest(split).mode, "split");
checker.check("and it names its own area", gui.collectRequest(split).area, "split");

choose("davinci");
checker.check("the other route too", split.mode, "davinci");
checker.check("button follows", element("btn-split-start").textContent, "Split for Resolve");
checker.check("request follows", gui.collectRequest(split).mode, "davinci");

console.log("\nEach route explains itself, and only itself");
choose("split");
checker.contains("1:1 promises no re-encode", element("split-mode-hint").textContent, "nothing is re-encoded");
checker.check("its details are shown", element("split-details-1to1").hidden, false);
checker.check("the other ones are not", element("split-details-davinci").hidden, true);

choose("davinci");
checker.contains("Resolve-ready names AAC", element("split-mode-hint").textContent, "AAC");
checker.check("its details are shown", element("split-details-davinci").hidden, false);
checker.check("the other ones are not", element("split-details-1to1").hidden, true);

// A hand-edited or future value must not leave the window without a mode:
// falling back to the untouched copy is the harmless half of the choice.
console.log("\nAn unknown value falls back to the harmless route");
element("split-mode").value = "something-else";
gui.applySplitMode();
checker.check("mode is the 1:1 one", split.mode, "split");
checker.check("button says so", element("btn-split-start").textContent, "Split losslessly");

console.log("\nThe route belongs to the page, not to whatever is on show");
choose("davinci");
gui.showPage("settings");
checker.check("settings changes nothing about it", split.mode, "davinci");
gui.showPage("convert");
// The convert area never had a mode to lose: it converts, and its button says
// so whatever another page is set to.
checker.check("converting is unaffected", gui.areaOf("convert").mode, "");
checker.check("its request too", gui.collectRequest(gui.areaOf("convert")).mode, "");
checker.check("and this page kept its route", split.mode, "davinci");
gui.showPage("split");
checker.check("still there when you come back", split.mode, "davinci");

console.log("\nOnly the chosen page is on show");
checker.check("the split page is visible", element("page-split").hidden, false);
checker.check("convert is put away", element("page-convert").hidden, true);
checker.check("join is put away", element("page-join").hidden, true);
gui.showPage("convert");
checker.check("the split page is put away again", element("page-split").hidden, true);

console.log("\nBoth routes count as tool runs, not as conversions");
gui.onConverterEvent({ ev: "run", slot: SPLIT_SLOT, mode: "split", version: "1.18.0" });
checker.check("splitting is a tool run", gui.isToolRun(split), true);
gui.onConverterEvent({ ev: "run", slot: SPLIT_SLOT, mode: "davinci", version: "1.18.0" });
checker.check("the Resolve route too", gui.isToolRun(split), true);
// And the area next door is untouched by it: converting stays converting.
checker.check("converting is still converting", gui.isToolRun(gui.areaOf("convert")), false);

// -split writes several files per source and reports no result per file, so the
// only honest sign that one is done is the next one starting. Claiming success
// here would be an invention.
console.log("\nA finished file is closed off without claiming success");
split.queue = [
  { path: "C:\a.mkv", name: "a.mkv", sizeMB: 10, status: "", note: "queued" },
  { path: "C:\b.mkv", name: "b.mkv", sizeMB: 10, status: "", note: "queued" }
];
gui.onConverterEvent({ ev: "run", slot: SPLIT_SLOT, mode: "split", version: "1.18.0" });
gui.onConverterEvent({ ev: "file", slot: SPLIT_SLOT, index: 1, total: 2, name: "a.mkv", path: "C:\a.mkv", in_mb: 10 });
gui.onConverterEvent({ ev: "file", slot: SPLIT_SLOT, index: 2, total: 2, name: "b.mkv", path: "C:\b.mkv", in_mb: 10 });
checker.check("the first file is closed off", split.queue[0].status, "processed");
checker.check("no success is claimed", split.queue[0].note.includes("smaller"), false);

console.log("\nThe chooser promises no stereo mix on the 1:1 route");
// -split hides the stereo option on purpose (a downmix would be a re-encode),
// so the hint the Resolve route shows would be a promise this one cannot keep.
split.answerAll = false;
gui.state.questions = [];
gui.onQuestion({
  ev: "question",
  slot: SPLIT_SLOT,
  kind: "tracks",
  file: "C:\Videos\Movie.mkv",
  options: [
    { n: 1, label: "Audio  ger  EAC3 6ch" },
    { n: 2, label: "Sub    eng  SUBRIP" }
  ]
});
checker.check("no stereo mix mentioned", element("ask-hint").textContent.includes("stereo mix"), false);
checker.contains("plain wording instead", element("ask-hint").textContent, "every track listed above");

checker.finish();
