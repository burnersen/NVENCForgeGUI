// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// options_check.js — checks what the option fields tell the user.
//
// Run it with:  node frontend\options_check.js
//
// The window must never show a number of its own: the target resolution and
// the CQ come out of NVENCForge_Config.ini, and which CQ applies depends on the
// codec. That decision is mirrored from the converter and is exactly what can
// silently drift apart. (Bitrate caps are gone since NVENCForge 2.0.0.)
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element } = loadGui();
const { check, contains, finish } = createChecker();

// The values the user's own INI holds, so the expected numbers below are the
// real ones rather than invented examples.
const sampleConfig = {
  found: true,
  path: "X:\\tools\\NVENCForge_Config.ini",
  maxResolution: 1080,
  targetCQ: 26,
  av1TargetCQ: 32,
  autoCQTargetVMAF: 96,
  autoCQ: true,
  autoCQKnown: true
};

function choose(codec, resolution) {
  element("opt-codec").value = codec;
  element("opt-resolution").value = resolution;
  gui.refreshFromConfig();
}

console.log("\n=== without a readable INI the window states nothing ===");
gui.applyConfig({ found: false, note: "not there yet" });
check("resolution label          ", element("opt-resolution-default").textContent, "Downscale if needed");
contains("quality bubble says so    ", gui.HELP.quality().now, "could not be read");

console.log("\n=== no bitrate cap any more (NVENCForge 2.0.0) ===");
gui.applyConfig(sampleConfig);
check("no bitrate help bubble    ", gui.HELP.bitrate, undefined);
check("no cap key mirror         ", html.includes("function bitrateCapKey"), false);

console.log("\n=== the resolution entry names the configured height ===");
choose("", "");
check("label from the INI        ", element("opt-resolution-default").textContent, "Downscale to max 1080p");
gui.applyConfig(Object.assign({}, sampleConfig, { maxResolution: 2160 }));
check("label follows the INI     ", element("opt-resolution-default").textContent, "Downscale to max 2160p");
gui.applyConfig(sampleConfig);

console.log("\n=== the fixed CQ starts at the value the INI uses ===");
gui.state.cqTouched = false;
choose("", "");
check("H.265 CQ                  ", element("opt-cq").value, 26);
choose("av1", "");
check("AV1 CQ                    ", element("opt-cq").value, 32);
// A number the user typed must survive a codec change — otherwise the window
// would quietly overwrite a deliberate choice.
gui.state.cqTouched = true;
element("opt-cq").value = 20;
choose("", "");
check("typed value is kept       ", element("opt-cq").value, 20);
gui.state.cqTouched = false;

console.log("\n=== every field with a bubble really has a text ===");
const keys = [...html.matchAll(/data-help="([a-z]+)"/g)].map((match) => match[1]);
check("fields marked in the HTML ", keys.length > 0, true);
keys.forEach((key) => {
  const build = gui.HELP[key];
  const help = typeof build === "function" ? build() : null;
  check("  " + key.padEnd(24), Boolean(help && help.title && help.text), true);
});

console.log("\n=== the quality bubble reports the INI state ===");
contains("auto CQ on                ", gui.HELP.quality().now, "Auto CQ on");
contains("with the quality target   ", gui.HELP.quality().now, "96");
gui.applyConfig(Object.assign({}, sampleConfig, { autoCQ: false }));
contains("auto CQ off               ", gui.HELP.quality().now, "Auto CQ off");

finish();
