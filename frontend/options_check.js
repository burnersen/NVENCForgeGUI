// options_check.js — checks what the option fields tell the user.
//
// Run it with:  node frontend\options_check.js
//
// The window must never show a number of its own: the bitrate cap, the target
// resolution and the CQ all come out of NVENCForge_Config.ini, and which cap
// applies depends on codec and resolution. That decision is mirrored from the
// converter (main.go, parseArgs) and is exactly what can silently drift apart.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element } = loadGui();
const { check, contains, finish } = createChecker();

// The values the user's own INI holds, so the expected numbers below are the
// real ones rather than invented examples.
const sampleConfig = {
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
check("bitrate placeholder       ", element("opt-bitrate").placeholder, "as configured");
check("resolution label          ", element("opt-resolution-default").textContent, "Downscale if needed (default)");
contains("bitrate bubble says so    ", gui.HELP.bitrate().now, "could not be read");
contains("quality bubble says so    ", gui.HELP.quality().now, "could not be read");

console.log("\n=== the cap follows codec and resolution ===");
gui.applyConfig(sampleConfig);
choose("", "");
check("H.265, downscaled         ", element("opt-bitrate").placeholder, "8000 (from your INI)");
check("  cap key                 ", gui.bitrateCapKey(), "maxBitrate1080p");
choose("", "original");
check("H.265, original size      ", element("opt-bitrate").placeholder, "22000 (from your INI)");
check("  cap key                 ", gui.bitrateCapKey(), "maxBitrateOriginal");
choose("av1", "");
check("AV1, downscaled           ", element("opt-bitrate").placeholder, "6000 (from your INI)");
check("  cap key                 ", gui.bitrateCapKey(), "av1MaxBitrate1080p");
choose("av1", "original");
check("AV1, original size        ", element("opt-bitrate").placeholder, "13000 (from your INI)");
check("  cap key                 ", gui.bitrateCapKey(), "av1MaxBitrateOriginal");
contains("bubble names the live cap ", gui.HELP.bitrate().now, "13000 kbit/s");

console.log("\n=== the resolution entry names the configured height ===");
choose("", "");
check("label from the INI        ", element("opt-resolution-default").textContent, "Downscale to max 1080p (default)");
gui.applyConfig(Object.assign({}, sampleConfig, { maxResolution: 2160 }));
check("label follows the INI     ", element("opt-resolution-default").textContent, "Downscale to max 2160p (default)");
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

console.log("\n=== the folded options say what they add up to ===");
// The fold hides ten fields. If the line written on it were wrong or stale,
// the window would quietly promise one run and start another — and nobody
// opens a fold to check, which is the entire point of having the line.
function setOptions(values) {
  const fields = {
    "opt-codec": "", "opt-encoder": "", "opt-container": "", "opt-resolution": "",
    "opt-audio": "", "opt-bitdepth": "", "opt-quality": "auto", "opt-cq": "26",
    "opt-bitrate": "", "opt-parallel": "2"
  };
  Object.assign(fields, values);
  Object.keys(fields).forEach((id) => { element(id).value = fields[id]; });
  element("opt-keep").checked = !!values.keep;
  element("opt-shutdown").checked = !!values.shutdown;
  gui.updateOptionsLine();
  return element("options-line").textContent;
}

check("the defaults, in one line ", setOptions({}), "H.265 · GPU · MKV · Auto CQ · 2 at a time");
check("every choice is carried   ",
  setOptions({ "opt-codec": "av1", "opt-encoder": "cpu", "opt-container": "mp4", "opt-parallel": "1" }),
  "AV1 · CPU · MP4 · Auto CQ · 1 at a time");
check("a fixed CQ shows its number",
  setOptions({ "opt-quality": "fixed", "opt-cq": "30" }),
  "H.265 · GPU · MKV · CQ 30 · 2 at a time");

// Everything below only turns up when it departs from the default. A line that
// always listed all of it would be as long as the panel it stands in for.
contains("8 bit is called out       ", setOptions({ "opt-bitdepth": "8" }), "8 bit");
contains("keeping the size too      ", setOptions({ "opt-resolution": "original" }), "original size");
contains("untouched audio too       ", setOptions({ "opt-audio": "copy" }), "audio 1:1");
contains("a typed bitrate too       ", setOptions({ "opt-bitrate": "6000" }), "max 6000 kbit/s");
contains("kept sources too          ", setOptions({ keep: true }), "sources kept");
contains("and the shutdown, loudest ", setOptions({ shutdown: true }), "shuts down when done");
check("but silent when they are off", setOptions({}).includes("8 bit"), false);

finish();
