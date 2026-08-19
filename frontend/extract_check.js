// extract_check.js — the Take apart area, without a browser.
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

// extractPage cuts out just this page, so a slot found on some OTHER page
// cannot make the structure checks pass by accident.
const extractPage = html.slice(
  html.indexOf('<div id="page-extract"'),
  html.indexOf('<!-- Join has a list of its own')
);

// choose sets the picker and lets the page react, the way a click would.
function choose(mode) {
  element("extract-mode").value = mode;
  gui.applyExtractMode();
}

console.log("\nOne page, reachable, with both routes on it");
checker.contains("the nav button opens the page", html, '<button class="nav-item" data-page="extract">Take apart</button>');
checker.check("the page itself exists", html.includes('<div id="page-extract" hidden>'), true);
checker.check("no separate Split page left", html.includes('id="page-split"'), false);
checker.check("no separate DaVinci page left", html.includes('id="page-davinci"'), false);
checker.check("no Split nav button left", /data-page="split"/.test(html), false);
checker.check("no DaVinci nav button left", /data-page="davinci"/.test(html), false);
checker.contains("1:1 is offered", extractPage, 'value="split"');
checker.contains("Resolve-ready is offered", extractPage, 'value="davinci"');

console.log("\nThe shared panels have somewhere to go");
checker.check("queue slot", extractPage.includes('data-slot="queue"'), true);
checker.check("run slot (progress + log)", extractPage.includes('data-slot="run"'), true);

console.log("\nThe chosen route reaches the converter");
gui.showPage("extract");
choose("split");
checker.check("mode follows the picker", gui.state.mode, "split");
checker.check("and the button says so", element("btn-start").textContent, "Split losslessly");
checker.check("the request carries it", gui.collectRequest().mode, "split");

choose("davinci");
checker.check("the other route too", gui.state.mode, "davinci");
checker.check("button follows", element("btn-start").textContent, "Split for Resolve");
checker.check("request follows", gui.collectRequest().mode, "davinci");

console.log("\nEach route explains itself, and only itself");
choose("split");
checker.contains("1:1 promises no re-encode", element("extract-mode-hint").textContent, "nothing is re-encoded");
checker.check("its details are shown", element("extract-details-split").hidden, false);
checker.check("the other ones are not", element("extract-details-davinci").hidden, true);

choose("davinci");
checker.contains("Resolve-ready names AAC", element("extract-mode-hint").textContent, "AAC");
checker.check("its details are shown", element("extract-details-davinci").hidden, false);
checker.check("the other ones are not", element("extract-details-split").hidden, true);

// A hand-edited or future value must not leave the window without a mode:
// falling back to the untouched copy is the harmless half of the choice.
console.log("\nAn unknown value falls back to the harmless route");
element("extract-mode").value = "something-else";
gui.applyExtractMode();
checker.check("mode is the 1:1 one", gui.state.mode, "split");
checker.check("button says so", element("btn-start").textContent, "Split losslessly");

console.log("\nSwitching pages cannot leave the wrong mode behind");
choose("davinci");
gui.showPage("settings");
checker.check("settings changes nothing about it", gui.state.mode, "davinci");
gui.showPage("convert");
checker.check("back to converting", gui.state.mode, "");
checker.check("button back to normal", element("btn-start").textContent, "Start");
checker.check("and the request too", gui.collectRequest().mode, "");
// Coming back has to restore the route that was picked, not the first one in
// the list — the picker still shows it, so anything else would be a lie.
gui.showPage("extract");
checker.check("the page remembers its route", gui.state.mode, "davinci");

console.log("\nOnly the chosen page is on show");
checker.check("take apart is visible", element("page-extract").hidden, false);
checker.check("convert is put away", element("page-convert").hidden, true);
checker.check("join is put away", element("page-join").hidden, true);
gui.showPage("convert");
checker.check("take apart is put away again", element("page-extract").hidden, true);

console.log("\nBoth routes count as tool runs, not as conversions");
gui.onConverterEvent({ ev: "run", mode: "split", version: "1.18.0" });
checker.check("splitting is a tool run", gui.isToolRun(), true);
gui.onConverterEvent({ ev: "run", mode: "davinci", version: "1.18.0" });
checker.check("the Resolve route too", gui.isToolRun(), true);

// -split writes several files per source and reports no result per file, so the
// only honest sign that one is done is the next one starting. Claiming success
// here would be an invention.
console.log("\nA finished file is closed off without claiming success");
gui.state.queue = [
  { path: "C:\a.mkv", name: "a.mkv", sizeMB: 10, status: "", note: "queued" },
  { path: "C:\b.mkv", name: "b.mkv", sizeMB: 10, status: "", note: "queued" }
];
gui.onConverterEvent({ ev: "run", mode: "split", version: "1.18.0" });
gui.onConverterEvent({ ev: "file", index: 1, total: 2, name: "a.mkv", path: "C:\a.mkv", in_mb: 10 });
gui.onConverterEvent({ ev: "file", index: 2, total: 2, name: "b.mkv", path: "C:\b.mkv", in_mb: 10 });
checker.check("the first file is closed off", gui.state.queue[0].status, "processed");
checker.check("no success is claimed", gui.state.queue[0].note.includes("smaller"), false);

console.log("\nThe chooser promises no stereo mix on the 1:1 route");
// -split hides the stereo option on purpose (a downmix would be a re-encode),
// so the hint the Resolve route shows would be a promise this one cannot keep.
gui.state.answerAllThisRun = false;
gui.state.questions = [];
gui.onQuestion({
  ev: "question",
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
