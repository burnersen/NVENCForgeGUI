// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// mirror_check.js — the window and NVENCForge_Config.ini must say the same thing.
//
// Run it with:  node frontend\mirror_check.js
//
// The bug this guards against is the one that started the whole rebuild: with
// encoder=cpu in the INI the window still said "GPU / NVENC", and with
// autoCQ=false it still said "Auto CQ on". Everything looked normal and the
// converter did something else. Three properties keep that from coming back:
//
//   1. at start-up the controls are filled FROM the file,
//   2. a change goes straight back INTO the file — under the right key,
//   3. nothing is written for a key the file does not have.
const { loadGui, createChecker } = require("./check_harness");

const { gui, element, calls, setSettingsFileReply } = loadGui();
const { check, finish } = createChecker();

// An INI that has everything, with values that differ from the window's own
// defaults on purpose — otherwise a control that was never filled in would
// look just as right.
const fullConfig = {
  found: true,
  path: "X:\\tools\\NVENCForge_Config.ini",
  maxResolution: 1080,
  maxBitrate1080p: 8000,
  maxBitrateOriginal: 22000,
  av1MaxBitrate1080p: 6000,
  av1MaxBitrateOriginal: 13000,
  targetCQ: 26,
  av1TargetCQ: 32,
  autoCQTargetVMAF: 96,
  autoCQ: false, autoCQKnown: true,
  codec: "av1", codecKnown: true,
  container: "mp4", containerKnown: true,
  audioMode: "copy", audioModeKnown: true,
  bitDepth: 8, bitDepthKnown: true,
  keepResolution: true, keepResolutionKnown: true,
  keepSource: true, keepSourceKnown: true,
  encoder: "cpu", encoderKnown: true,
  autoCrop: true, autoCropKnown: true,
  autoShutdown: false, autoShutdownKnown: true,
  retireMode: "folder"
};

console.log("\n=== the file fills the controls ===");
// The converter must count as new enough, otherwise the black-bar box is
// unticked again on purpose (applyCropCapability).
gui.state.cropCapable = true;
gui.applyConfig(fullConfig);

check("codec                     ", element("opt-codec").value, "av1");
check("encoder                   ", element("opt-encoder").value, "cpu");
check("container                 ", element("opt-container").value, "mp4");
check("audio                     ", element("opt-audio").value, "copy");
check("bit depth                 ", element("opt-bitdepth").value, "8");
check("resolution                ", element("opt-resolution").value, "original");
check("quality                   ", element("opt-quality").value, "fixed");
check("keep source               ", element("opt-keep").checked, true);
check("cut black bars            ", element("opt-crop").checked, true);
check("the watched folder too    ", element("wopt-codec").value, "av1");
// The shutdown box belongs to the Convert page alone: a standing order that
// switches the machine off would never convert the next file to arrive.
check("no shutdown box on watch  ", element("wopt-shutdown").checked, false);

console.log("\n=== a change goes back into the file ===");
calls.settingSaves.length = 0;
element("opt-codec").value = "h265";
gui.rememberOption("opt", gui.INI_MIRROR.find((e) => e.id === "codec"));
check("one write                 ", calls.settingSaves.length, 1);
check("under the key codec       ", JSON.stringify(calls.settingSaves[0]), '{"codec":"h265"}');

calls.settingSaves.length = 0;
element("opt-resolution").value = "downscale";
gui.rememberOption("opt", gui.INI_MIRROR.find((e) => e.id === "resolution"));
// The window asks "downscale?", the file asks "keep the resolution?" — the
// answer has to be turned around on the way, or it means the opposite.
check("keepResolution turns round", JSON.stringify(calls.settingSaves[0]), '{"keepResolution":"false"}');

calls.settingSaves.length = 0;
element("opt-quality").value = "auto";
gui.rememberOption("opt", gui.INI_MIRROR.find((e) => e.id === "quality"));
check("auto CQ back on           ", JSON.stringify(calls.settingSaves[0]), '{"autoCQ":"true"}');

calls.settingSaves.length = 0;
element("opt-crop").checked = false;
gui.rememberOption("opt", gui.INI_MIRROR.find((e) => e.id === "crop"));
check("black bars off            ", JSON.stringify(calls.settingSaves[0]), '{"autoCrop":"false"}');

console.log("\n=== the CQ and the cap know which key they belong to ===");
calls.settingSaves.length = 0;
element("opt-codec").value = "av1";
element("opt-cq").value = "30";
gui.rememberCQ();
check("AV1 writes av1TargetCQ    ", JSON.stringify(calls.settingSaves[0]), '{"av1TargetCQ":"30"}');

calls.settingSaves.length = 0;
element("opt-codec").value = "h265";
element("opt-cq").value = "24";
gui.rememberCQ();
check("H.265 writes targetCQ     ", JSON.stringify(calls.settingSaves[0]), '{"targetCQ":"24"}');

calls.settingSaves.length = 0;
element("opt-resolution").value = "original";
element("opt-bitrate").value = "15000";
gui.rememberBitrate();
check("cap follows the mode      ", JSON.stringify(calls.settingSaves[0]), '{"maxBitrateOriginal":"15000"}');

calls.settingSaves.length = 0;
element("opt-bitrate").value = "";
gui.rememberBitrate();
// Empty is not a value: it means "whatever the file says". Writing a zero
// there would wreck the cap for every later run.
check("empty writes nothing      ", calls.settingSaves.length, 0);

console.log("\n=== an older INI is left alone ===");
// Same window, but a file from an older NVENCForge: it simply has no such
// lines. Writing them would invent settings in someone else's file — and the
// converter next door would not understand them either.
const oldConfig = { found: true, path: "X:\\tools\\NVENCForge_Config.ini", maxResolution: 1080, targetCQ: 26 };
gui.state.optionsSeeded = false;
gui.applyConfig(oldConfig);
calls.settingSaves.length = 0;
gui.INI_MIRROR.forEach((entry) => gui.rememberOption("opt", entry));
check("nothing written           ", calls.settingSaves.length, 0);

console.log("\n=== the watched folder never writes ===");
gui.state.optionsSeeded = false;
gui.applyConfig(fullConfig);
calls.settingSaves.length = 0;
gui.INI_MIRROR.forEach((entry) => gui.rememberOption("wopt", entry));
check("still nothing written     ", calls.settingSaves.length, 0);

console.log("\n=== a profile carries the whole file ===");
setSettingsFileReply({
  found: true,
  path: "X:\\tools\\NVENCForge_Config.ini",
  note: "",
  settings: [
    { key: "targetCQ", value: "26" },
    { key: "autoCQTargetVMAF", value: "96" },
    { key: "aqStrength", value: "2" }
  ]
});
gui.ensureSettingsFile().then(() => {
  const snapshot = gui.settingsSnapshot();
  check("snapshot has every key    ", Object.keys(snapshot).sort().join(","), "aqStrength,autoCQTargetVMAF,targetCQ");
  check("with its value            ", snapshot.aqStrength, "2");

  const profile = gui.profileFromOptions("Archiv");
  check("the profile carries it    ", profile.settings.autoCQTargetVMAF, "96");
  // Without this the black-bar tick was thrown away on the way to the file —
  // the window sent it, the Go side had no field for it.
  check("and the black-bar tick    ", typeof profile.crop, "boolean");

  return gui.applyProfileSettings({ name: "Archiv", settings: snapshot });
}).then(() => {
  check("loading applies the file  ", calls.profileApplies.join(","), "Archiv");
  // Older profiles have no copy of the file. Then nothing may happen — the
  // current settings are not to be wiped by an old profile.
  return gui.applyProfileSettings({ name: "Alt" });
}).then(() => {
  check("an old profile writes not ", calls.profileApplies.join(","), "Archiv");
  finish();
});
