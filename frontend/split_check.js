// split_check.js — the Split area, without a browser.
//
// The expensive mistake here is silent: if the start button on this page still
// carries the convert mode, a click re-encodes the files instead of taking them
// apart 1:1 — and nothing on screen looks wrong until the result is there.
// The second one is just as quiet: a page that never gets the queue and the
// progress panels leaves the user staring at an empty area during a real run.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element } = loadGui();
const checker = createChecker();

// splitPage cuts out just this page, so a slot found on some OTHER page cannot
// make the structure checks pass by accident.
const splitPage = html.slice(
  html.indexOf('<div id="page-split"'),
  html.indexOf('<div id="page-settings"')
);

console.log("\nThe area is reachable and no longer marked as unbuilt");
checker.contains("the nav button opens the page", html, '<button class="nav-item" data-page="split">Split</button>');
checker.check("no greyed-out Split button is left", /disabled>Split/.test(html), false);
checker.check("the page itself exists", html.includes('<div id="page-split" hidden>'), true);

console.log("\nThe shared panels have somewhere to go");
checker.check("queue slot", splitPage.includes('data-slot="queue"'), true);
checker.check("run slot (progress + log)", splitPage.includes('data-slot="run"'), true);

console.log("\nThe page decides what the start button runs");
gui.showPage("split");
checker.check("mode follows the page", gui.state.mode, "split");
checker.check("and the button says so", element("btn-start").textContent, "Split losslessly");
checker.check("the request carries it", gui.collectRequest().mode, "split");

console.log("\nSwitching pages cannot leave the wrong mode behind");
gui.showPage("settings");
checker.check("settings changes nothing about it", gui.state.mode, "split");
gui.showPage("convert");
checker.check("back to converting", gui.state.mode, "");
checker.check("button back to normal", element("btn-start").textContent, "Start");
checker.check("and the request too", gui.collectRequest().mode, "");

console.log("\nOnly the chosen page is on show");
gui.showPage("split");
checker.check("split is visible", element("page-split").hidden, false);
checker.check("convert is put away", element("page-convert").hidden, true);
checker.check("davinci is put away", element("page-davinci").hidden, true);
gui.showPage("convert");
checker.check("split is put away again", element("page-split").hidden, true);

console.log("\nA split run is treated as a tool run, not as a conversion");
gui.onConverterEvent({ ev: "run", mode: "split", version: "1.18.0" });
checker.check("split is a tool run", gui.isToolRun(), true);

// -split writes several files per source and reports no result per file, so the
// only honest sign that one is done is the next one starting. Claiming success
// here would be an invention.
gui.state.queue = [
  { path: "C:\\a.mkv", name: "a.mkv", sizeMB: 10, status: "", note: "queued" },
  { path: "C:\\b.mkv", name: "b.mkv", sizeMB: 10, status: "", note: "queued" }
];
gui.onConverterEvent({ ev: "run", mode: "split", version: "1.18.0" });
gui.onConverterEvent({ ev: "file", index: 1, total: 2, name: "a.mkv", path: "C:\\a.mkv", in_mb: 10 });
gui.onConverterEvent({ ev: "file", index: 2, total: 2, name: "b.mkv", path: "C:\\b.mkv", in_mb: 10 });
checker.check("the first file is closed off", gui.state.queue[0].status, "processed");
checker.check("no success is claimed", gui.state.queue[0].note.includes("smaller"), false);

console.log("\nThe chooser promises no stereo mix here");
// -split hides the stereo option on purpose (a downmix would be a re-encode),
// so the hint the DaVinci area shows would be a promise this mode cannot keep.
gui.state.answerAllThisRun = false;
gui.onQuestion({
  ev: "question",
  kind: "tracks",
  file: "C:\\Videos\\Movie.mkv",
  options: [
    { n: 1, label: "Audio  ger  EAC3 6ch" },
    { n: 2, label: "Sub    eng  SUBRIP" }
  ]
});
checker.check("no stereo mix mentioned", element("ask-hint").textContent.includes("stereo mix"), false);
checker.contains("plain wording instead", element("ask-hint").textContent, "every track listed above");

checker.finish();
