// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// crop_check.js — the black-bar controls.
//
// Run it with:  node frontend\crop_check.js
//
// Two things here can go wrong quietly, which is why they are checked at all:
//
//   1. The check button must send "check", never "on". Send the wrong one and
//      a user who only wanted to LOOK gets every file cut instead - and the
//      cut cannot be undone by looking at the result, only by finding the
//      original again.
//   2. The controls must disappear when the converter next door is older than
//      1.21.0. NVENCForge shrugs an unknown option off as a log warning, so a
//      run with a dead -crop looks completely normal and simply keeps the bars.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls } = loadGui();
const { check, contains, finish } = createChecker();

const ableConverter = { found: true, eventChannel: true, autoCrop: true };
const oldConverter = { found: true, eventChannel: true, autoCrop: false };

// A converter that can do it, and one file to work on, so the buttons have
// something to be enabled for.
gui.showConverter(ableConverter);
gui.areaOf("convert").queue = [{ path: "X:\\filme\\Breitbild.mkv" }];

console.log("\n=== the box decides what the run is told ===");
element("opt-crop").checked = false;
check("unticked sends nothing    ", gui.collectRequest(gui.areaOf("convert")).crop, "");
element("opt-crop").checked = true;
check("ticked sends \"on\"         ", gui.collectRequest(gui.areaOf("convert")).crop, "on");

console.log("\n=== the check button looks, it does not cut ===");
element("opt-crop").checked = false;
element("opt-shutdown").checked = true;
calls.runs.length = 0;
gui.start("convert", "crop");
const checkRun = calls.runs[calls.runs.length - 1];
check("the run asks for a check  ", checkRun && checkRun.crop, "check");
check("and never for a cut       ", checkRun && checkRun.crop === "on", false);
// A check converts nothing, so there is nothing to switch the machine off for -
// and somebody has to look at the pictures afterwards.
check("no shutdown after a check ", checkRun && checkRun.shutdown, false);

console.log("\n=== a normal start still converts ===");
element("opt-crop").checked = true;
calls.runs.length = 0;
gui.start("convert");
const normalRun = calls.runs[calls.runs.length - 1];
check("start sends \"on\"          ", normalRun && normalRun.crop, "on");

console.log("\n=== the watched folder has the box too ===");
element("wopt-crop").checked = true;
check("watch run sends \"on\"      ", gui.collectWatchRequest(["a.mkv"]).crop, "on");
element("wopt-crop").checked = false;
check("and nothing when unticked ", gui.collectWatchRequest(["a.mkv"]).crop, "");

console.log("\n=== profiles remember the setting ===");
element("opt-crop").checked = true;
check("saved into the profile    ", gui.profileFromOptions("Breitbild").crop, true);
gui.applyProfile("opt", { name: "ohne", crop: false });
check("loading a profile clears  ", element("opt-crop").checked, false);
gui.applyProfile("opt", { name: "mit", crop: true });
check("loading a profile sets    ", element("opt-crop").checked, true);
// An older profile has no crop field at all. It must read as "off" rather than
// quietly switching cutting on for somebody who never asked for it.
gui.applyProfile("opt", { name: "alt" });
check("an old profile means off  ", element("opt-crop").checked, false);

console.log("\n=== an older converter gets no black-bar controls ===");
element("opt-crop").checked = true;
gui.showConverter(oldConverter);
check("the box is switched off   ", element("opt-crop").disabled, true);
check("and unticked, not just grey", element("opt-crop").checked, false);
gui.updateButtons();
check("the check button is off   ", element("btn-convert-cropcheck").disabled, true);
// The reason belongs in the help bubble, where every other setting explains
// itself - a box that is simply grey leaves the user guessing.
contains("the bubble says why       ", gui.HELP.crop().now, "1.21.0");

console.log("\n=== and gets them back with a new one ===");
gui.showConverter(ableConverter);
gui.updateButtons();
check("box usable again          ", element("opt-crop").disabled, false);
check("check button usable again ", element("btn-convert-cropcheck").disabled, false);

console.log("\n=== the bubble tells the truth about file size ===");
// The measured effect turns around with Auto CQ, and the window must say so:
// with Auto CQ on, cutting the bars makes files BIGGER, not smaller. A text
// promising savings would be plainly wrong for the default setting.
gui.applyConfig({ found: true, autoCQ: true, autoCQKnown: true });
contains("auto CQ on: larger        ", gui.HELP.crop().now, "larger");
gui.applyConfig({ found: true, autoCQ: false, autoCQKnown: true });
contains("auto CQ off: smaller      ", gui.HELP.crop().now, "smaller");
contains("the text explains why     ", gui.HELP.crop().text, "flatter");
contains("and points at the check   ", gui.HELP.crop().text, "Check black bars");

console.log("\n=== the page carries the controls ===");
contains("the box is on the page    ", html, 'id="opt-crop"');
contains("the check button too      ", html, 'id="btn-convert-cropcheck"');
contains("the watched folder's box  ", html, 'id="wopt-crop"');

finish();
