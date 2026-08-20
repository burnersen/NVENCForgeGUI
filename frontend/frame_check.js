// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// frame_check.js — checks what the window frame is told: the percentage in the
// title, the bar in the taskbar button, and the flash when a batch is through.
//
// Run it with:  node frontend\frame_check.js
//
// Why this needs checking at all: the frame is the one part of the window you
// look at when the window is NOT on screen. Nobody notices a wrong figure
// there while testing, because while testing the window is in front of you.
//
// The taskbar button exists once, but up to four areas can be working. So the
// figure has to be the sum over everything that is running — a frame following
// only the page on show would be wrong exactly when it matters.
const { loadGui, createChecker } = require("./check_harness");

const { gui, calls } = loadGui();
const { check, finish } = createChecker();

const convert = gui.areaOf("convert");
const split = gui.areaOf("split");

// The last thing the frame was told, and how often it was told anything.
const lastCall = () => calls.frame[calls.frame.length - 1] || "(nothing)";
const callCount = () => calls.frame.length;

const entry = (name, sizeMB) =>
  ({ path: "X:\\test\\" + name, name, folder: "X:\\test", sizeMB, status: "", note: "" });

// The four areas as the dispatcher reports them. Anything not named is idle.
const areas = (busy) => Object.assign({
  convert: { active: 0, pending: 0, limit: 1 },
  split: { active: 0, pending: 0, limit: 1 },
  join: { active: 0, pending: 0, limit: 1 },
  watch: { active: 0, pending: 0, limit: 1 }
}, busy || {});
const report = (busy) => gui.onQueueState({ areas: areas(busy), totalLimit: 3 });

// A batch, and how much of it is already through. finished is set by hand
// here: how a file gets that far is progress_check.js's job, this check is
// about the figure the frame is given.
function batchOf(area, files, doneCount) {
  area.queue = files;
  gui.startBatch(area, files.map((file) => file.path));
  files.forEach((file, index) => { file.finished = index < doneCount; });
}

console.log("\n=== nothing running ===");
report({});
check("frame is put away         ", lastCall(), "idle");

console.log("\n=== half a batch of two ===");
batchOf(convert, [entry("a.mkv", 100), entry("b.mkv", 100)], 1);
report({ convert: { active: 1, pending: 0, limit: 1 } });
check("one of two files through  ", lastCall(), "percent:50");

console.log("\n=== the same figure is not sent twice ===");
const before = callCount();
gui.updateFrame();
gui.updateFrame();
check("nothing new to say        ", callCount(), before);

console.log("\n=== a second area joins in ===");
// Splitting adds 200 MB of its own, none of it done: 100 of 400 MB.
batchOf(split, [entry("c.mkv", 100), entry("d.mkv", 100)], 0);
report({ convert: { active: 1, pending: 0, limit: 1 }, split: { active: 1, pending: 0, limit: 1 } });
check("both areas counted        ", lastCall(), "percent:25");

console.log("\n=== an area without a list ===");
// Joining builds ONE file out of several, so there is nothing to count. It
// must say "working" rather than invent a percentage.
report({ join: { active: 1, pending: 0, limit: 1 } });
check("says working, no figure   ", lastCall(), "busy");

console.log("\n=== the batch is through ===");
report({});
check("the button is flashed     ", calls.frame.includes("done"), true);
check("and the bar is put away   ", lastCall(), "idle");

console.log("\n=== the watched folder never reports an end ===");
// It is a standing order, not a batch: a file of its own is no reason to
// flash. Nothing is signalled when it goes quiet again.
const flashesSoFar = calls.frame.filter((call) => call === "done").length;
report({ watch: { active: 1, pending: 0, limit: 1 } });
check("a watched file is working ", lastCall(), "busy");
report({});
check("no flash from the watcher ", calls.frame.filter((call) => call === "done").length, flashesSoFar);

finish();
