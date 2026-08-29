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

// file builds one entry the way Go delivers it — including the two answers Go
// works out about grouping: which job the file belongs to (group) and whether
// that job is complete (ready). Both come from joinfiles.go, so the checks here
// hand them in rather than working them out a second time.
const file = (name, kind, extra) => Object.assign(
  {
    path: "C:\\v\\" + name, name, folder: "C:\\v", kind, note: "",
    group: "film", ready: true, sizeMB: 1, missing: false
  },
  extra || {}
);

const baseVideo = file("film.NoSound.mkv", "video");
const german = file("film.ger.m4a", "audio");
const subtitle = file("film.ger.srt", "subtitle");
// A group of its own: a second film in the same folder.
const otherVideo = file("zweiter.NoSound.mkv", "video", { group: "zweiter" });
const otherAudio = file("zweiter.eng.ac3", "audio", { group: "zweiter" });

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
// lossless tools sit right behind Convert, DaVinci and Settings follow, and
// About closes the list. It is pure markup order, so nothing else would ever
// notice if it got shuffled.
const navBlock = html.slice(html.indexOf("<nav>"), html.indexOf("</nav>"));
const navOrder = Array.from(navBlock.matchAll(/nav-item[^>]*data-page="(\w+)"/g)).map((m) => m[1]);
checker.check("the areas stand in the wanted order", navOrder.join(" "), "convert split join watch settings about");

console.log("\nThe page brings its own list, progress and log");
checker.check("its own drop area", joinPage.includes('id="join-dropzone"'), true);
checker.check("its own list", joinPage.includes('id="join-list"'), true);
checker.check("a result line", joinPage.includes('id="join-result"'), true);
checker.check("its own progress", joinPage.includes('id="join-lanes"'), true);
checker.check("its own log", joinPage.includes('id="join-logbox"'), true);
checker.check("its own start button", joinPage.includes('id="btn-join-start"'), true);
// The shared queue element must NOT be here: this page keeps a list of its
// own, because it takes audio and subtitle files as well and groups them into
// jobs by name.
checker.check("no shared queue element", joinPage.includes('id="join-queue"'), false);
// No overall bar either — with one file there is nothing for it to add up, and
// an empty bar next to a working one reads like something is stuck.
checker.check("and no overall bar", joinPage.includes('id="join-bar"'), false);

console.log("\nThe page decides what its own start button runs");
gui.showPage("join");
gui.applyJoinMode();
checker.check("mode follows the page", join.mode, "join");
checker.check("and the button says so", element("btn-join-start").textContent, "Join the queue");
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
setList([file("film.NoSound.mkv", "video", { ready: false, note: "no audio or subtitle for this video yet" })]);
checker.check("video alone is not enough", element("btn-join-start").disabled, true);
setList([file("film.ger.m4a", "audio", { group: "", ready: false, note: "no video of this name in the list" })]);
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

/* ---------- the queue: one job per film ----------
   This is what makes the area worth queueing at all. Splitting is done in
   batches, so putting back together has to be too: drop the results of ten
   films in, get ten jobs. If they all ran together instead, the first film
   would be built with the sound of the other nine. */

console.log("\nSeveral films become several jobs");
setList([baseVideo, german, subtitle, otherVideo, otherAudio]);
const jobs = gui.joinJobs();
checker.check("two jobs", jobs.length, 2);
checker.check("the first is the first film", jobs[0].name, "film");
checker.check("the second is the other one", jobs[1].name, "zweiter");
checker.check("and every file goes along", gui.joinRunFiles().length, 5);
checker.contains("the count is on show", element("join-info").textContent, "2 jobs");

console.log("\nThe list is grouped by job, not by file type");
// With ten films, a list sorted by type would show ten videos, then thirty
// sound tracks — and which belongs to which would have to be read off names.
created.length = 0;
setList([baseVideo, german, subtitle, otherVideo, otherAudio]);
const headings = created.filter((el) => el.className === "group-head").map((el) => el.textContent);
checker.check("one heading per job", headings.length, 2);
checker.contains("it names the film", headings[0], "film");
checker.contains("and says what it is made of", headings[0], "1 audio track + 1 subtitle");

console.log("\nA group that cannot run says why and lets the others go");
// The decision was: leave it standing, greyed out — one leftover file must not
// hold up a whole batch.
created.length = 0;
const lonely = file("allein.NoSound.mkv", "video", { group: "allein", ready: false });
const orphanSub = file("waise.ger.srt", "subtitle", { group: "", ready: false, note: "no video of this name in the list" });
setList([baseVideo, german, lonely, orphanSub]);
checker.check("only the complete one runs", gui.joinJobs().length, 1);
checker.check("and the start is free", element("btn-join-start").disabled, false);
const withLeftovers = created.filter((el) => el.className === "group-head").map((el) => el.textContent);
checker.contains("the lonely video says why", withLeftovers.join(" | "), "nothing to add to it yet");
checker.contains("and the orphan is named as left over", withLeftovers.join(" | "), "Left over");
// Neither of them may reach the converter.
const sent = gui.collectRequest(join).files;
checker.check("the lonely video stays behind", sent.includes(lonely.path), false);
checker.check("the orphan stays behind", sent.includes(orphanSub.path), false);

console.log("\nA missing file stops its own job — and only that one");
setList([baseVideo, Object.assign({}, german, { missing: true }), otherVideo, otherAudio]);
checker.check("the other film still runs", gui.joinJobs().length, 1);
checker.check("and it is the untouched one", gui.joinJobs()[0].name, "zweiter");
checker.check("nothing of the broken job is sent", gui.joinRunFiles().includes(baseVideo.path), false);

console.log("\nWhat the converter cannot use never leaves the window");
const orphanVobSub = file("film.ger.sub", "unusable", { group: "", ready: false, note: "a .sub only works together with its .idx file" });
setList([baseVideo, german, subtitle, orphanVobSub]);
const request = gui.collectRequest(join);
// Passing it along would make the converter refuse the whole run ("Unknown
// file types") — a .sub without its .idx is nothing he can read.
checker.check("the orphaned .sub stays behind", request.files.includes(orphanVobSub.path), false);
checker.check("but the job itself runs", request.files.length, 3);
// …and an unusable file must not block a run that is otherwise fine.
checker.check("it does not block the start", element("btn-join-start").disabled, false);

console.log("\nThe result line says what will happen, without inventing a name");
setList([]);
checker.contains("nothing there yet", element("join-result").textContent, "sort themselves into jobs");
setList([file("film.NoSound.mkv", "video", { ready: false })]);
checker.contains("video alone", element("join-result").textContent, "at least one audio or subtitle");
setList([baseVideo, german, subtitle, otherVideo, otherAudio]);
const resultLine = element("join-result").textContent;
checker.contains("counts the jobs", resultLine, "2 jobs");
checker.contains("says what comes out", resultLine, ".joined.mkv");
// The converter builds the names himself and tidies them (measured: "Big Buck
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
  checker.check("and the button says so", element("btn-join-start").textContent, "Join the queue");
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
