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
checker.check("the areas stand in the wanted order", navOrder.join(" "), "convert split join davinci settings");

console.log("\nThe page brings its own list and lets the shared panels in");
checker.check("its own drop area", joinPage.includes('id="join-dropzone"'), true);
checker.check("its own list", joinPage.includes('id="join-list"'), true);
checker.check("a result line", joinPage.includes('id="join-result"'), true);
// The shared queue must NOT be here: it takes video files only, and one join
// run builds exactly one file — a queue would promise a batch.
checker.check("no shared queue slot", joinPage.includes('data-slot="queue"'), false);
checker.check("run slot (progress + log)", joinPage.includes('data-slot="run"'), true);

console.log("\nThe page decides what the start button runs");
gui.showPage("join");
checker.check("mode follows the page", gui.state.mode, "join");
checker.check("and the button says so", element("btn-start").textContent, "Join into one MKV");
checker.check("the request carries it", gui.collectRequest().mode, "join");
gui.showPage("convert");
checker.check("back to converting", gui.state.mode, "");
gui.showPage("join");

console.log("\nOnly the chosen page is on show");
checker.check("join is visible", element("page-join").hidden, false);
checker.check("split is put away", element("page-split").hidden, true);
checker.check("convert is put away", element("page-convert").hidden, true);
// Leaving the page must hide it again. A page missing from the PAGES list is
// never touched at all — and would then stand open underneath the next one.
gui.showPage("convert");
checker.check("and it is put away again", element("page-join").hidden, true);
gui.showPage("join");

console.log("\nStart stays locked until the run can actually work");
gui.state.converterFound = true;
gui.state.running = false;
setList([]);
checker.check("nothing dropped yet", element("btn-start").disabled, true);
setList([baseVideo]);
checker.check("video alone is not enough", element("btn-start").disabled, true);
setList([german]);
checker.check("audio without a video is not enough", element("btn-start").disabled, true);
setList([baseVideo, german]);
checker.check("video + audio starts", element("btn-start").disabled, false);
setList([baseVideo, subtitle]);
checker.check("video + subtitle alone also starts", element("btn-start").disabled, false);

// A file that has moved away in the meantime would let the converter give up
// mid-run — and he reports that only in the log, never in the data channel.
setList([Object.assign({}, baseVideo, { missing: true }), german]);
checker.check("a missing video locks it", element("btn-start").disabled, true);
setList([baseVideo, Object.assign({}, german, { missing: true })]);
checker.check("a missing audio file locks it", element("btn-start").disabled, true);

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
const request = gui.collectRequest();
checker.check("video first", request.files[0], baseVideo.path);
checker.check("then audio", request.files[1], german.path);
checker.check("then subtitles", request.files[2], subtitle.path);
checker.check("nothing else goes along", request.files.length, 3);
// Passing either one would make the converter refuse the whole run ("Unknown
// file types") — the .sub is read by ffmpeg next to its .idx, not as argument.
checker.check("the orphaned .sub stays behind", request.files.includes(orphan.path), false);
checker.check("the companion .sub stays behind", request.files.includes(companion.path), false);
// …but an unusable file must not block a run that is otherwise fine.
checker.check("and it does not block the start", element("btn-start").disabled, false);

console.log("\nWith several videos the user picks the base");
const second = file("other.mkv", "video");
setList([baseVideo, second, german]);
checker.check("the first one stands in", gui.joinBase().path, baseVideo.path);
gui.state.joinBasePath = second.path;
gui.afterJoinChange();
checker.check("the chosen one wins", gui.joinBase().path, second.path);
checker.check("and it is what gets sent", gui.collectRequest().files[0], second.path);
checker.check("the other video does not come along", gui.collectRequest().files.includes(baseVideo.path), false);
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
  gui.state.queue = [
    { path: "C:\\v\\film.NoSound.mkv", name: "film.NoSound.mkv", sizeMB: 500, status: "", note: "" }
  ];
  gui.state.totalMB = 500;
  gui.onConverterEvent({ ev: "run", mode: "join", version: "1.18.0" });
  checker.check("joining does not feed on the queue", gui.runUsesQueue(), false);
  checker.check("no queue entry is re-labelled", gui.state.queue[0].note, "");
  // The converter names the same file the queue happens to hold — it must not
  // light up as if it were being worked on.
  gui.onConverterEvent({
    ev: "file", index: 1, total: 1,
    name: "film.NoSound.mkv", path: "C:\\v\\film.NoSound.mkv", in_mb: 14
  });
  checker.check("and none is marked as running", gui.state.queue[0].status, "");
  checker.check("the overall bar stays quiet", element("pct-all").textContent, "—");
  checker.contains("the file is still named", element("nowfile").textContent, "film.NoSound.mkv");

  console.log("\nSplitting still works the old way");
  gui.onConverterEvent({ ev: "run", mode: "split", version: "1.18.0" });
  checker.check("split keeps its queue", gui.runUsesQueue(), true);
  checker.check("and is still a tool run", gui.isToolRun(), true);

  checker.finish();
});
