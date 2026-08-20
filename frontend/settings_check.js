// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// settings_check.js — checks the settings page without touching a real INI.
//
// Run it with:  node frontend\settings_check.js
//
// The page is generated from the INI, so the risky part is the reading: an
// "Allowed:" line that is understood wrongly would offer values the converter
// rejects, and a default that is picked wrongly would undo the user's work on
// the next click of the undo arrow. Both are checked here against entries in
// exactly the shape the app delivers them.
const { loadGui, createChecker } = require("./check_harness");

const { gui, element } = loadGui();
const { check, contains, finish } = createChecker();

// Entries as the app parses them out of NVENCForge_Config.ini, values and
// ranges taken from the real file.
const settings = [
  { key: "maxResolution", value: "1080", default: "1080", allowed: "720, 1080, 1440, 2160",
    description: "Videos larger than this are scaled down.", group: "common", section: "" },
  { key: "retireMode", value: "folder", default: "folder", allowed: "folder, recyclebin",
    description: "What happens to the original.", group: "common", section: "" },
  { key: "targetCQ", value: "26", default: "26", allowed: "1 to 51",
    description: "Fixed quality value for H.265.", group: "expert", section: "Quality and bitrate" },
  { key: "maxBitrate1080p", value: "8000", default: "8000", allowed: "more than 1000",
    description: "Upper bitrate limit.", group: "expert", section: "Quality and bitrate" },
  { key: "casStrength", value: "0", default: "0.4", allowed: "0.0 to 1.0",
    description: "Sharpening strength.", group: "expert", section: "Quality and bitrate" },
  { key: "autoCQTolerance", value: "0.5", default: "0.5", allowed: "0 to 5",
    description: "How far below the target is acceptable.", group: "expert", section: "Automatic quality" },
  { key: "autoCQTargetVMAF", value: "96", default: "96", allowed: "70 to 99",
    description: "Quality the automatic search aims for.", group: "expert", section: "Automatic quality" },
  { key: "bFrames", value: "5", default: "5", allowed: "0 to 5",
    description: "Number of B-frames.", group: "expert", section: "Encoder internals" },
  { key: "nvencPreset", value: "p5", default: "p5", allowed: "p1 to p7",
    description: "Graphics card encoder preset.", group: "expert", section: "Encoder internals" },
  { key: "aqStrength", value: "2", default: "2", allowed: "1 to 15",
    description: "How strongly the encoder shifts bits. Measured across four real files: dropping this from 8 to 2 made every one of them 8-28% smaller.", group: "expert", section: "Encoder internals" },
  { key: "cpuPreset", value: "fast", default: "fast", allowed: "ultrafast ... fast, medium, slow ... placebo",
    description: "Speed/quality trade-off. Measured: medium gains almost nothing.", group: "expert", section: "CPU mode" },
  { key: "gpuDecode", value: "true", default: "true", allowed: "true, false",
    description: "Unpack on the graphics card.", group: "expert", section: "Speed" },
  { key: "extraFilenameChars", value: "", default: "", allowed: "any characters, or empty",
    description: "Characters that survive file name cleaning.", group: "expert", section: "Everything else" }
];

function reset() {
  gui.state.settingsFile = { found: true, path: "X:\\tools\\NVENCForge_Config.ini", settings: settings };
  gui.state.edits = {};
  gui.state.gpuAdvice = {};
  gui.renderSettings();
}
const model = (key) => gui.settingModel(settings.find((entry) => entry.key === key));

reset();

console.log("\n=== the allowed range decides the control ===");
check("maxResolution is a list      ", model("maxResolution").kind, "choice");
check("  with four entries          ", model("maxResolution").choices.join(","), "720,1080,1440,2160");
check("gpuDecode is a list          ", model("gpuDecode").choices.join(","), "true,false");
check("retireMode is a list         ", model("retireMode").choices.join(","), "folder,recyclebin");
check("nvencPreset ladder           ", model("nvencPreset").choices.join(","), "p1,p2,p3,p4,p5,p6,p7");
check("cpuPreset knows all presets  ", model("cpuPreset").choices.length, 10);
check("  starts with the fastest    ", model("cpuPreset").choices[0], "ultrafast");
check("  ends with the slowest      ", model("cpuPreset").choices[9], "placebo");
check("targetCQ is a number         ", model("targetCQ").kind, "number");
check("  lowest allowed             ", model("targetCQ").min, "1");
check("  highest allowed            ", model("targetCQ").max, "51");
check("'more than 1000' starts at   ", model("maxBitrate1080p").min, "1001");
check("  and has no upper limit     ", model("maxBitrate1080p").max, "");
check("free text stays free text    ", model("extraFilenameChars").kind, "text");

console.log("\n=== whole numbers and decimals are told apart ===");
check("bFrames counts in ones       ", model("bFrames").step, "1");
check("autoCQTolerance in tenths    ", model("autoCQTolerance").step, "0.1");
check("casStrength in tenths        ", model("casStrength").step, "0.1");
check("autoCQTargetVMAF in tenths   ", model("autoCQTargetVMAF").step, "0.1");

console.log("\n=== impossible values are marked, never blocked ===");
const targetCQ = settings.find((entry) => entry.key === "targetCQ");
check("26 is fine                   ", gui.looksInvalid(targetCQ), false);
gui.editSetting("targetCQ", "60");
check("60 is out of range           ", gui.looksInvalid(targetCQ), true);
check("but it is still kept         ", gui.changedValues().targetCQ, "60");
gui.editSetting("targetCQ", "26");
check("back to 26 -> no change left ", Object.keys(gui.changedValues()).length, 0);

console.log("\n=== changes are collected, not written ===");
reset();
gui.editSetting("bFrames", "3");
gui.editSetting("gpuDecode", "false");
check("two changes pending          ", Object.keys(gui.changedValues()).length, 2);
check("save button is enabled       ", element("btn-settings-save").disabled, false);
check("save button counts them      ", element("btn-settings-save").textContent, "Save 2 change(s)");
check("state says so                ", element("settings-state").textContent, "unsaved changes");
check("navigation is marked too     ", element('.nav-item[data-page="settings"]').textContent, "Settings ●");
gui.editSetting("gpuDecode", "true");
check("undoing one by hand          ", Object.keys(gui.changedValues()).length, 1);

console.log("\n=== the undo arrow goes back to the default ===");
reset();
gui.editSetting("casStrength", "0.9");
check("changed to                   ", gui.changedValues().casStrength, "0.9");
gui.revertSetting("casStrength");
check("back to the INI default      ", element("set-casStrength").value, "0.4");
check("  and counts as a change     ", gui.changedValues().casStrength, "0.4");

console.log("\n=== the card has the last word on its own limits ===");
reset();
// Exactly what the converter prints, colour codes included.
gui.log({ text: "\u001b[33m Set 'bFrames=4' in NVENCForge_Config.ini to make this permanent.\u001b[0m", back: 0 });
check("advice was picked up         ", gui.state.gpuAdvice.bFrames, "4");
check("default follows the card     ", gui.defaultFor(settings.find((e) => e.key === "bFrames")), "4");
gui.revertSetting("bFrames");
check("undo uses the card's value   ", element("set-bFrames").value, "4");
contains("the bubble mentions it       ", gui.settingHelp("bFrames").now, "your graphics card asked for 4");
gui.log({ text: "NVENCForge finished.", back: 0 });
check("ordinary lines change nothing", Object.keys(gui.state.gpuAdvice).length, 1);

console.log("\n=== restore defaults touches every setting ===");
reset();
gui.editSetting("bFrames", "0");
gui.editSetting("targetCQ", "40");
gui.restoreDefaults();
const afterRestore = gui.changedValues();
check("bFrames back to default      ", element("set-bFrames").value, "5");
check("targetCQ back to default     ", element("set-targetCQ").value, "26");
// casStrength stands at 0 in this file while its default is 0.4, so restoring
// really has to change it — that is the case a naive "reset" would miss.
check("casStrength was 0, now 0.4   ", afterRestore.casStrength, "0.4");
check("only real differences count  ", afterRestore.bFrames, undefined);

console.log("\n=== help texts ===");
reset();
contains("comes out of the INI         ", gui.settingHelp("maxResolution").text, "scaled down");
contains("names the allowed range      ", gui.settingHelp("targetCQ").now, "Allowed: 1 to 51");
contains("names the default            ", gui.settingHelp("targetCQ").now, "Default: 26");
check("aqStrength without measurements", gui.settingHelp("aqStrength").text.includes("Measured"), false);
check("cpuPreset without measurements ", gui.settingHelp("cpuPreset").text.includes("Measured"), false);
contains("aqStrength still explains it ", gui.settingHelp("aqStrength").text, "blocky patches");
check("an empty default reads 'empty' ", gui.settingHelp("extraFilenameChars").now.includes("Default: empty"), true);

console.log("\n=== switching pages ===");
gui.showPage("settings");
check("settings visible             ", element("page-settings").hidden, false);
check("convert hidden               ", element("page-convert").hidden, true);
gui.showPage("convert");
check("back to convert              ", element("page-convert").hidden, false);

console.log("\n=== finding a setting among twenty-seven ===");
// Searched are the name and the section — not the explanations. A word like
// "file" appears in half of those, and a filter that answers with half the
// page has filtered nothing.
reset();
const filter = (text) => { element("settings-filter").value = text; gui.applySettingsFilter(); };
const rowHidden = (key) => element("row-" + key).hidden;

filter("cq");
check("a CQ setting stays          ", rowHidden("targetCQ"), false);
check("and the other one too       ", rowHidden("autoCQTolerance"), false);
check("something else goes         ", rowHidden("gpuDecode"), true);
contains("and it says how many       ", element("settings-filter-note").textContent, "of 13 shown");

// The section a setting stands in counts as its name too: someone looking
// for "encoder" means the whole group, not one line.
filter("encoder internals");
check("a whole section can be found", rowHidden("bFrames"), false);
check("and its neighbours come too ", rowHidden("nvencPreset"), false);
check("the rest is out             ", rowHidden("retireMode"), true);

// A heading with nothing under it would stand over an empty space.
filter("cpupreset");
check("the section with the hit stays", element("sec-expert-cpu-mode").hidden, false);
check("an empty section is hidden  ", element("sec-expert-speed").hidden, true);

/* A hit among the expert settings sits inside a closed box. Without opening
   it the search would look as if it had found nothing at all — the worst
   kind of wrong answer, because it reads like the setting does not exist. */
element("settings-expert-box").open = false;
filter("bframes");
check("the expert box opens itself ", element("settings-expert-box").open, true);

// Nothing found is said in words. An empty page with no explanation looks
// like the settings failed to load.
filter("gibtesnicht");
contains("nothing found is said      ", element("settings-filter-note").textContent, "nothing is called that");

// An empty box shows everything again — nothing is hidden for good.
filter("");
check("everything is back          ", rowHidden("gpuDecode"), false);
check("headings too                ", element("sec-expert-speed").hidden, false);
check("and the note is quiet       ", element("settings-filter-note").textContent, "");

// The two columns are asked for by minimum width, so a narrow window falls
// back to one on its own. Checked in the markup, since there is no layout
// engine here to measure.
const { html } = require("./check_harness").loadGui();
contains("the settings stand in a grid", html, 'class="settings-grid" id="settings-common"');
contains("the expert ones as well    ", html, 'class="settings-grid" id="settings-expert"');
contains("two columns where it fits  ", html, "repeat(auto-fit, minmax(400px, 1fr))");
contains("a heading spans them both  ", html, ".settings-grid > .sec-title { grid-column: 1 / -1; }");

finish();
