// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// join_check.js — the Join area, without a browser.
//
// Three mistakes here are silent and expensive:
//  1. The start button carries the wrong mode — a click re-encodes instead of
//     copying, and nothing looks wrong until the result is there.
//  2. The run is started with files the converter cannot use. He then sends his
//     "run" event and ends WITHOUT A WORD (measured 2026-08-18, exit code 0),
//     so the window would sit there waiting for a summary that never comes.
//  3. The queue of the other pages gets dragged into a join run: entries marked
//     as working, an overall bar measuring files nobody is touching.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls, setJoinReply, created } = loadGui();
const checker = createChecker();

// joinPage cuts out just this page, so a slot found on some OTHER page cannot
// make the structure checks pass by accident.
const joinPage = html.slice(
  html.indexOf('<div id="page-join"'),
  html.indexOf('<div id="page-settings"')
);

// file builds one entry the way Go delivers it.
const file = (name, kind, extra) => Object.assign(
  { path: "C:\\v\\" + name, name, folder: "C:\\v", kind, note: "", sizeMB: 1, missing: false },
  extra || {}
);

const baseVideo = file("film.NoSound.mkv", "video");
const german = file("film.ger.m4a", "audio");
const subtitle = file("film.ger.srt", "subtitle");

// Joining is an area of its own: its own list, its own progress area, its own
// log, and its own slot at the converter (6) so it can run beside a batch.
const join = gui.areaOf("join");
const convert = gui.areaOf("convert");
const JOIN_SLOT = 6;

function setList(files) {
  gui.state.joinFiles = files;
  gui.afterJoinChange();
}

console.log("\nThe area is reachable and no longer marked as unbuilt");
checker.contains("the nav button opens the page", html, '<button class="nav-item" data-page="join">Join</button>');
checker.check("no greyed-out Join button is left", /disabled>Join/.test(html), false);
checker.check("the page itself exists", html.includes('<div id="page-join" hidden>'), true);

// The order of the areas is the user's own choice (2026-08-18): the two
// lossless tools sit right behind Convert, DaVinci and Settings follow. It is
// pure markup order, so nothing else would ever notice if it got shuffled.
const navBlock = html.slice(html.indexOf("<nav>"), html.indexOf("</nav>"));
const navOrder = Array.from(navBlock.matchAll(/nav-item[^>]*data-page="(\w+)"/g)).map((m) => m[1]);
checker.check("the areas stand in the wanted order", navOrder.join(" "), "convert split join watch settings");

console.log("\nThe page brings its own list, progress and log");
checker.check("its own drop area", joinPage.includes('id="join-dropzone"'), true);
checker.check("its own list", joinPage.includes('id="join-list"'), true);
checker.check("a result line", joinPage.includes('id="join-result"'), true);
checker.check("its own progress", joinPage.includes('id="join-lanes"'), true);
checker.check("its own log", joinPage.includes('id="join-logbox"'), true);
checker.check("its own start button", joinPage.includes('id="btn-join-start"'), true);
// A queue of video files must NOT be here: one join run builds exactly one
// file, and a queue would promise a batch this mode cannot do.
checker.check("no queue of video files", joinPage.includes('id="join-queue"'), false);
// No overall bar either — with one file there is nothing for it to add up, and
// an empty bar next to a working one reads like something is stuck.
checker.check("and no overall bar", joinPage.includes('id="join-bar"'), false);

console.log("\nThe page decides what its own start button runs");
gui.showPage("join");
gui.applyJoinMode();
checker.check("mode follows the page", join.mode, "join");
checker.check("and the button says so", element("btn-join-start").textContent, "Join into one MKV");
checker.check("the request carries it", gui.collectRequest(join).mode, "join");
checker.check("and it names its own area", gui.collectRequest(join).area, "join");
gui.showPage("convert");
// Converting keeps its own button and its own mode — leaving this page cannot
// change either any more.
checker.check("converting is untouched", convert.mode, "");
gui.showPage("join");

console.log("\nOnly the chosen page is on show");
checker.check("join is visible", element("page-join").hidden, false);
checker.check("the split page is put away", element("page-split").hidden, true);
checker.check("convert is put away", element("page-convert").hidden, true);
// Leaving the page must hide it again. A page missing from the PAGES list is
// never touched at all — and would then stand open underneath the next one.
gui.showPage("convert");
checker.check("and it is put away again", element("page-join").hidden, true);
gui.showPage("join");

console.log("\nStart stays locked until the run can actually work");
gui.state.converterFound = true;
join.running = false;
setList([]);
checker.check("nothing dropped yet", element("btn-join-start").disabled, true);
setList([baseVideo]);
checker.check("video alone is not enough", element("btn-join-start").disabled, true);
setList([german]);
checker.check("audio without a video is not enough", element("btn-join-start").disabled, true);
setList([baseVideo, german]);
checker.check("video + audio starts", element("btn-join-start").disabled, false);
setList([baseVideo, subtitle]);
checker.check("video + subtitle alone also starts", element("btn-join-start").disabled, false);

// A file that has moved away in the meantime would let the converter give up
// mid-run — and he reports that only in the log, never in the data channel.
setList([Object.assign({}, baseVideo, { missing: true }), german]);
checker.check("a missing video locks it", element("btn-join-start").disabled, true);
setList([baseVideo, Object.assign({}, german, { missing: true })]);
checker.check("a missing audio file locks it", element("btn-join-start").disabled, true);

console.log("\nThe groups are named the way the user reads them");
// "Picture" was the first wording and the user asked for "Video" — the list
// heading and the row label are the only two places that say it out loud.
created.length = 0;
setList([baseVideo, german, subtitle]);
const headings = created.filter((el) => el.className === "group-head").map((el) => el.textContent);
checker.check("the video group", headings[0], "Video (1)");
checker.check("the audio group", headings[1], "Audio (1)");
checker.check("the subtitle group", headings[2], "Subtitles (1)");
const baseLabel = created.filter((el) => el.className === "state base").map((el) => el.textContent);
checker.check("the chosen row says video", baseLabel[0], "video");

console.log("\nOnly files the converter can use are handed over");
const orphan = file("film.ger.sub", "unusable", { note: "a .sub only works together with its .idx file" });
const companion = file("film.eng.sub", "companion", { note: "goes along with the .idx of the same name" });
setList([baseVideo, german, subtitle, orphan, companion]);
const request = gui.collectRequest(join);
checker.check("video first", request.files[0], baseVideo.path);
checker.check("then audio", request.files[1], german.path);
checker.check("then subtitles", request.files[2], subtitle.path);
checker.check("nothing else goes along", request.files.length, 3);
// Passing either one would make the converter refuse the whole run ("Unknown
// file types") — the .sub is read by ffmpeg next to its .idx, not as argument.
checker.check("the orphaned .sub stays behind", request.files.includes(orphan.path), false);
checker.check("the companion .sub stays behind", request.files.includes(companion.path), false);
// …but an unusable file must not block a run that is otherwise fine.
checker.check("and it does not block the start", element("btn-join-start").disabled, false);

console.log("\nWith several videos the user picks the base");
const second = file("other.mkv", "video");
setList([baseVideo, second, german]);
checker.check("the first one stands in", gui.joinBase().path, baseVideo.path);
gui.state.joinBasePath = second.path;
gui.afterJoinChange();
checker.check("the chosen one wins", gui.joinBase().path, second.path);
checker.check("and it is what gets sent", gui.collectRequest(join).files[0], second.path);
checker.check("the other video does not come along", gui.collectRequest(join).files.includes(baseVideo.path), false);
// A video that was removed must not stay chosen invisibly.
setList([baseVideo, german]);
checker.check("a vanished choice falls back", gui.joinBase().path, baseVideo.path);

console.log("\nThe result line says what will happen, without inventing a name");
setList([]);
checker.contains("nothing there yet", element("join-result").textContent, "Add one video file");
setList([baseVideo]);
checker.contains("video alone", element("join-result").textContent, "at least one audio or subtitle");
setList([baseVideo, german, subtitle]);
const resultLine = element("join-result").textContent;
checker.contains("names the base file", resultLine, "film.NoSound.mkv");
checker.contains("says what comes out", resultLine, ".joined.mkv");
checker.contains("counts the audio", resultLine, "1 audio track");
checker.contains("counts the subtitles", resultLine, "1 subtitle");
// The converter builds the name himself and tidies it (measured: "Big Buck
// Bunny.NoSound.mkv" comes back as "Big.Buck.Bunny.joined.mkv"). A name spelled
// out here would be a promise the window cannot keep.
checker.check("no invented file name", resultLine.includes("film.joined.mkv"), false);

console.log("\nDropping hands the WHOLE list to the sorting, not just the new files");
// Whether a .sub can be used depends on the .idx next to it — and that one may
// only arrive with the second drop.
setJoinReply([baseVideo, german]);
gui.state.joinFiles = [baseVideo];
calls.joinSorts.length = 0;
gui.addJoinPaths([german.path]).then(() => {
  checker.check("one call", calls.joinSorts.length, 1);
  checker.check("the known file goes along", calls.joinSorts[0][0], baseVideo.path);
  checker.check("and the new one too", calls.joinSorts[0][1], german.path);

  console.log("\nA join run leaves the queue of the other pages alone");
  convert.queue = [
    { path: "C:\\v\\film.NoSound.mkv", name: "film.NoSound.mkv", sizeMB: 500, status: "", note: "" }
  ];
  convert.totalMB = 500;
  // The convert bar is put to its resting state first, so that "it never
  // moved" is a real reading and not just an element nobody has touched.
  gui.resetProgress(convert);
  gui.onConverterEvent({ ev: "run", slot: JOIN_SLOT, mode: "join", version: "1.18.0" });
  checker.check("no queue entry is re-labelled", convert.queue[0].note, "");
  // The converter names the same file the convert queue happens to hold — it
  // must not light up as if it were being worked on.
  gui.onConverterEvent({
    ev: "file", slot: JOIN_SLOT, index: 1, total: 1,
    name: "film.NoSound.mkv", path: "C:\\v\\film.NoSound.mkv", in_mb: 14
  });
  checker.check("and none is marked as running", convert.queue[0].status, "");
  checker.check("the convert bar stays quiet", element("convert-pct").textContent, "—");
  // The file is named where it belongs: in this area's own progress display.
  checker.contains("the file is named here", join.slots[JOIN_SLOT].name, "film.NoSound.mkv");
  checker.check("and nowhere else", Object.keys(convert.slots).length, 0);

  console.log("\nSplitting keeps its own queue, on its own slot");
  gui.onConverterEvent({ ev: "run", slot: 5, mode: "split", version: "1.18.0" });
  checker.check("splitting works from a list", gui.areaOf("split").hasQueue, true);
  checker.check("joining does not", join.hasQueue, false);
  checker.check("and both are tool runs", gui.isToolRun(gui.areaOf("split")) && gui.isToolRun(join), true);

  // The two ways of joining send the same files but do NOT do the same job:
  // -join copies everything, -davinci re-encodes the audio Resolve cannot read
  // and cleans the subtitles. A start button that quietly ran the wrong one
  // would either lose the AAC conversion or re-encode a lossless round trip.
  console.log("\nThe join page offers both routes");
  gui.showPage("join");
  element("join-mode").value = "join";
  gui.applyJoinMode();
  checker.check("1:1 is the mode", join.mode, "join");
  checker.check("and the button says so", element("btn-join-start").textContent, "Join into one MKV");
  checker.contains("the hint mentions copying", element("join-mode-hint").textContent, "copied exactly as it is");

  element("join-mode").value = "joindavinci";
  gui.applyJoinMode();
  checker.check("Resolve-ready is the mode", join.mode, "joindavinci");
  checker.check("and the button changes", element("btn-join-start").textContent, "Join for Resolve");
  checker.contains("the hint names AAC", element("join-mode-hint").textContent, "AAC");

  console.log("\nBoth routes draw from the join list, whichever is chosen");
  // Where the files come from is the area's business now, not the mode's: this
  // page has no queue at all, so both routes can only take the join list.
  element("join-mode").value = "join";
  gui.applyJoinMode();
  checker.check("1:1 sends the list", gui.collectRequest(join).files[0], baseVideo.path);
  element("join-mode").value = "joindavinci";
  gui.applyJoinMode();
  checker.check("Resolve-ready sends the same", gui.collectRequest(join).files[0], baseVideo.path);
  checker.check("both are known routes", !!(gui.JOIN_MODES.join && gui.JOIN_MODES.joindavinci), true);

  // A run started as -davinci from this page must not touch another area's
  // list either — it is still a join, on the join slot.
  gui.onConverterEvent({ ev: "run", slot: JOIN_SLOT, mode: "joindavinci", version: "1.18.0" });
  checker.check("a Resolve join spares the queue", convert.queue[0].status, "");

  console.log("\nAn unknown value falls back instead of sending nonsense");
  element("join-mode").value = "something-else";
  checker.check("falls back to 1:1", gui.joinMode(), "join");

  console.log("\nThe choice leaves another page's button alone");
  gui.showPage("split");
  element("split-mode").value = "split";
  gui.applySplitMode();
  element("join-mode").value = "joindavinci";
  gui.applyJoinMode();
  checker.check("the split page keeps its mode", gui.areaOf("split").mode, "split");
  checker.check("and the join page keeps its own", join.mode, "joindavinci");

  checker.finish();
});
