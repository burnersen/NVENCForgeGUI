// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// controls_check.js — does every control in the window actually do something?
//
// Run it with:  node frontend\controls_check.js
//
// The other checks each look at one corner of the window. This one asks the
// blunt question instead: take every button, box and drop-down that the shipped
// index.html has, and see whether it is connected at all — and, for the fields
// that decide how a file is encoded, whether the choice really reaches the
// converter.
//
// It exists because of a user report: AV1 and 8-bit were picked in the window
// and the file came out H.265 and 10-bit anyway. A check that only looked at
// one field at a time would not have caught it.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, calls, element } = loadGui();
const { check, contains, finish } = createChecker();

// Die Programmdatei nebenan kann alles, was es gibt. So prüft dieser Lauf die
// Oberfläche und nicht die Rückfallwege für alte Ausgaben.
const fullConfig = {
  found: true,
  path: "X:\\tools\\NVENCForge_Config.ini",
  maxResolution: 1080,
  maxBitrate1080p: 8000, maxBitrateOriginal: 22000,
  av1MaxBitrate1080p: 6000, av1MaxBitrateOriginal: 13000,
  targetCQ: 26, av1TargetCQ: 32, autoCQTargetVMAF: 97,
  autoCQ: true, autoCQKnown: true,
  retireMode: "folder",
  codec: "h265", codecKnown: true,
  container: "mkv", containerKnown: true,
  audioMode: "aac", audioModeKnown: true,
  bitDepth: 10, bitDepthKnown: true,
  keepResolution: false, keepResolutionKnown: true,
  keepSource: false, keepSourceKnown: true,
  encoder: "nvidia", encoderKnown: true,
  autoCrop: false, autoCropKnown: true,
  autoShutdown: false, autoShutdownKnown: true
};

// ---------------------------------------------------------------------------
// Teil 1 — ist überhaupt etwas angeschlossen?
//
// wire() ist die eine Stelle, an der das Fenster seine Bedienelemente mit
// Verhalten verbindet. Danach muss jeder Knopf einen Handler tragen. Einer ohne
// ist im laufenden Programm nicht von einem kaputten zu unterscheiden: er sieht
// normal aus, lässt sich drücken und tut nichts.
// ---------------------------------------------------------------------------
gui.applyConfig(fullConfig);
gui.wire();

// Die Liste kommt aus der ausgelieferten Datei, nicht aus einer Abschrift.
// Ein neu eingebauter Knopf steht damit von selbst mit auf dem Prüfstand.
function controlsOfKind(kind) {
  const found = [];
  const re = new RegExp("<" + kind + "\\b([^>]*)>", "g");
  let m;
  while ((m = re.exec(html)) !== null) {
    const id = (m[1].match(/id="([^"]+)"/) || [])[1];
    if (id) found.push({ id, attrs: m[1] });
  }
  return found;
}

function hasHandler(id) {
  const el = element(id);
  const own = ["onclick", "onchange", "oninput", "onkeydown"].some(
    (name) => typeof el[name] === "function" && el[name].name !== ""
      || (Object.prototype.hasOwnProperty.call(el, name) && typeof el[name] === "function")
  );
  return own || (el.listeners || []).length > 0;
}

console.log("\n=== every button is connected ===");
const buttons = controlsOfKind("button");
const deadButtons = buttons.filter((b) => !hasHandler(b.id));
check("buttons in the window     ", buttons.length > 0, true);
check("buttons without a handler ", deadButtons.map((b) => b.id).join(", ") || "none", "none");

console.log("\n=== every tick box is connected or read ===");
// Ein Kästchen braucht keinen Handler, wenn sein Stand beim Start abgeholt
// wird. Ungelesen UND ohne Handler ist es hingegen reine Zierde.
const script = html.slice(html.lastIndexOf("<script>"));
const boxes = controlsOfKind("input").filter((c) => /type="checkbox"/.test(c.attrs));
// Gesucht wird auch nach dem KURZNAMEN ohne Bereich: Ein Kästchen, das über
// el(area, "…") angesprochen wird, steht im Skript nie unter seinem vollen
// Namen — eine Suche nur nach der ganzen id würde es als tot melden. Das traf
// früher die vier "Follow"-Kästchen; die gibt es seit dem Mitrollen mit
// Ruhepause nicht mehr, die Regel bleibt als Vorsorge.
const shortName = (id) => id.replace(/^(convert|split|join|watch)-/, "");
const idleBoxes = boxes.filter((b) => !hasHandler(b.id)
  && !script.includes('"' + b.id + '"')
  && !script.includes('"' + shortName(b.id) + '"'));
check("tick boxes neither read nor wired", idleBoxes.map((b) => b.id).join(", ") || "none", "none");

console.log("\n=== every drop-down is read somewhere ===");
const selects = controlsOfKind("select");
const idleSelects = selects.filter((s) => !script.includes('"' + s.id + '"') && !hasHandler(s.id));
check("drop-downs never read     ", idleSelects.map((s) => s.id).join(", ") || "none", "none");

// ---------------------------------------------------------------------------
// Teil 2 — kommt die Wahl auch an?
//
// Hier wird der Start-Knopf wirklich gedrückt. Was dabei zur Go-Seite geht,
// steht in calls.runs — genau das, was der Konverter später zu sehen bekommt.
// ---------------------------------------------------------------------------
const convert = gui.areaOf("convert");

// setzt die Konvertieren-Seite auf einen bekannten Stand zurück.
function resetConvertPage() {
  element("opt-codec").value = "h265";
  element("opt-encoder").value = "nvidia";
  element("opt-container").value = "mkv";
  element("opt-resolution").value = "downscale";
  element("opt-audio").value = "aac";
  element("opt-bitdepth").value = "10";
  element("opt-quality").value = "auto";
  element("opt-cq").value = "26";
  element("opt-bitrate").value = "";
  element("opt-parallel").value = "1";
  element("opt-keep").checked = false;
  element("opt-crop").checked = false;
  element("opt-shutdown").checked = false;
}

// pressStart drückt den echten Start-Knopf und liefert, was dabei losging.
async function pressStart(prepare) {
  resetConvertPage();
  convert.queue = [{ path: "X:\\video.mkv", name: "video.mkv" }];
  if (prepare) prepare();
  calls.runs.length = 0;
  await element("btn-convert-start").onclick();
  return calls.runs[calls.runs.length - 1] || {};
}

async function main() {
  console.log("\n=== the Convert page: every field reaches the run ===");

  let run = await pressStart();
  check("default codec            ", run.codec, "h265");
  check("default container        ", run.container, "mkv");
  check("default bit depth        ", run.bitDepth, "10");

  run = await pressStart(() => { element("opt-codec").value = "av1"; });
  check("codec AV1                ", run.codec, "av1");

  run = await pressStart(() => { element("opt-bitdepth").value = "8"; });
  check("bit depth 8              ", run.bitDepth, "8");

  run = await pressStart(() => { element("opt-container").value = "mp4"; });
  check("container MP4            ", run.container, "mp4");

  run = await pressStart(() => { element("opt-encoder").value = "cpu"; });
  check("encoder CPU              ", run.encoder, "cpu");

  run = await pressStart(() => { element("opt-resolution").value = "original"; });
  check("keep resolution          ", run.resolution, "original");

  run = await pressStart(() => { element("opt-audio").value = "copy"; });
  check("audio copied 1:1         ", run.audio, "copy");

  run = await pressStart(() => {
    element("opt-quality").value = "fixed";
    element("opt-cq").value = "30";
  });
  check("fixed quality            ", run.quality, "fixed");
  check("  the CQ itself          ", run.fixedCQ, 30);

  run = await pressStart(() => { element("opt-bitrate").value = "9000"; });
  check("bitrate cap              ", run.maxBitrate, 9000);

  run = await pressStart(() => { element("opt-keep").checked = true; });
  check("keep the original        ", run.keepSource, true);

  run = await pressStart(() => { element("opt-crop").checked = true; });
  check("cut off black bars       ", run.crop, "on");

  run = await pressStart(() => { element("opt-parallel").value = "3"; });
  check("three at a time          ", run.parallel, 3);

  console.log("\n=== the user's case from the issue: AV1 + 8 bit + MP4 ===");
  run = await pressStart(() => {
    element("opt-codec").value = "av1";
    element("opt-bitdepth").value = "8";
    element("opt-container").value = "mp4";
  });
  check("codec                    ", run.codec, "av1");
  check("bit depth                ", run.bitDepth, "8");
  check("container                ", run.container, "mp4");

  console.log("\n=== the two look-only runs keep the same choices ===");
  run = await pressStart(() => {
    element("opt-codec").value = "av1";
    calls.runs.length = 0;
  });
  // Der Prüflauf geht über einen eigenen Knopf, nicht über Start.
  convert.queue = [{ path: "X:\\video.mkv", name: "video.mkv" }];
  element("opt-codec").value = "av1";
  calls.runs.length = 0;
  await element("btn-convert-cqcheck").onclick();
  const cqRun = calls.runs[calls.runs.length - 1] || {};
  check("quality check keeps codec", cqRun.codec, "av1");
  check("  and asks for the check ", cqRun.quality, "check");

  convert.queue = [{ path: "X:\\video.mkv", name: "video.mkv" }];
  element("opt-codec").value = "av1";
  calls.runs.length = 0;
  await element("btn-convert-cropcheck").onclick();
  const cropRun = calls.runs[calls.runs.length - 1] || {};
  check("bar check keeps codec    ", cropRun.codec, "av1");
  check("  and asks for the check ", cropRun.crop, "check");

  console.log("\n=== the watched folder has its own set of fields ===");
  element("wopt-codec").value = "av1";
  element("wopt-bitdepth").value = "8";
  element("wopt-container").value = "mp4";
  element("wopt-encoder").value = "cpu";
  element("wopt-resolution").value = "original";
  element("wopt-audio").value = "copy";
  element("wopt-quality").value = "fixed";
  element("wopt-cq").value = "40";
  element("wopt-bitrate").value = "7000";
  element("wopt-keep").checked = true;
  element("wopt-crop").checked = true;
  const watch = gui.collectWatchRequest(["X:\\watched.mkv"]);
  check("watch codec              ", watch.codec, "av1");
  check("watch bit depth          ", watch.bitDepth, "8");
  check("watch container          ", watch.container, "mp4");
  check("watch encoder            ", watch.encoder, "cpu");
  check("watch resolution         ", watch.resolution, "original");
  check("watch audio              ", watch.audio, "copy");
  check("watch quality            ", watch.quality, "fixed");
  check("watch CQ                 ", watch.fixedCQ, 40);
  check("watch bitrate            ", watch.maxBitrate, 7000);
  check("watch keep               ", watch.keepSource, true);
  check("watch crop               ", watch.crop, "on");

  console.log("\n=== AV1 in an MP4 says so, before the run starts ===");
  // Der Hinweis haengt an addEventListener, nicht an onchange — geprueft wird
  // deshalb ueber dispatch("change"), also genau den Weg, den ein echter Klick
  // im Fenster nimmt. Ein Aufruf der Funktion von Hand wuerde die Verdrahtung
  // ueberspringen und einen abgehaengten Hinweis nicht bemerken.
  function pickCodecContainer(prefix, codec, container) {
    element(prefix + "-codec").value = codec;
    element(prefix + "-container").value = container;
    element(prefix + "-container").dispatch("change");
    return element(prefix + "-reach-note").textContent;
  }
  for (const prefix of ["opt", "wopt"]) {
    const where = prefix === "opt" ? "Convert" : "watched folder";
    contains(where + ": AV1 + MP4 warns   ", pickCodecContainer(prefix, "av1", "mp4"), "older ones");
    check(where + ": AV1 + MKV silent  ", pickCodecContainer(prefix, "av1", "mkv"), "");
    check(where + ": H.265 + MP4 silent", pickCodecContainer(prefix, "h265", "mp4"), "");
    check(where + ": H.265 + MKV silent", pickCodecContainer(prefix, "h265", "mkv"), "");
    // Auch der Weg ueber das Codec-Feld muss ihn ausloesen, nicht nur der
    // ueber den Container: sonst haengt der Hinweis nur an einer der zwei
    // Wahlmoeglichkeiten, die ihn gemeinsam ergeben.
    element(prefix + "-container").value = "mp4";
    element(prefix + "-codec").value = "av1";
    element(prefix + "-codec").dispatch("change");
    contains(where + ": codec field too   ", element(prefix + "-reach-note").textContent, "older ones");
    pickCodecContainer(prefix, "h265", "mkv");
  }
  // Und die Vorbelegung aus der INI muss ihn ebenfalls setzen: Wer beides in
  // der Datei stehen hat, klickt hier nie und saehe sonst gar nichts.
  gui.applyConfig({ ...fullConfig, codec: "av1", container: "mp4" });
  gui.seedOptionsFromConfig("opt");
  contains("straight from the INI    ", element("opt-reach-note").textContent, "older ones");
  gui.applyConfig(fullConfig);

  console.log("\n=== Split and Join: the mode drop-down decides the job ===");
  // Beide Seiten bieten denselben Vorgang zweimal an: einmal roh und einmal
  // fuer DaVinci Resolve. Waehlt das Fenster den falschen, sieht der Lauf normal
  // aus und liefert Ton, den Resolve nicht oeffnen kann.
  const split = gui.areaOf("split");
  for (const [choice, wanted] of [["split", "split"], ["davinci", "davinci"]]) {
    split.queue = [{ path: "X:\\film.mkv", name: "film.mkv" }];
    element("split-mode").value = choice;
    gui.applySplitMode();
    calls.runs.length = 0;
    await element("btn-split-start").onclick();
    const job = calls.runs[calls.runs.length - 1] || {};
    check(("split mode \"" + choice + "\"").padEnd(25), job.mode, wanted);
  }

  const join = gui.areaOf("join");
  for (const [choice, wanted] of [["join", "join"], ["joindavinci", "joindavinci"]]) {
    // Zusammenfuegen braucht genau ein Bild plus mindestens eine Tonspur.
    gui.state.joinFiles = [
      { path: "X:\\film.mkv", kind: "video" },
      { path: "X:\\film.ac3", kind: "audio" }
    ];
    join.queue = gui.state.joinFiles.map((f) => ({ path: f.path, name: f.path }));
    element("join-mode").value = choice;
    gui.applyJoinMode();
    calls.runs.length = 0;
    await element("btn-join-start").onclick();
    const job = calls.runs[calls.runs.length - 1] || {};
    check(("join mode \"" + choice + "\"").padEnd(25), job.mode, wanted);
  }

  console.log("\n=== the INI fills the fields in, both pages ===");
  gui.applyConfig({ ...fullConfig, codec: "av1", bitDepth: 8, container: "mp4", encoder: "cpu",
    audioMode: "copy", keepResolution: true, keepSource: true, autoCrop: true, autoCQ: false });
  resetConvertPage();
  gui.seedOptionsFromConfig("opt");
  check("INI sets the codec       ", element("opt-codec").value, "av1");
  check("INI sets the bit depth   ", element("opt-bitdepth").value, "8");
  check("INI sets the container   ", element("opt-container").value, "mp4");
  check("INI sets the encoder     ", element("opt-encoder").value, "cpu");
  check("INI sets the audio       ", element("opt-audio").value, "copy");
  check("INI sets the resolution  ", element("opt-resolution").value, "original");
  check("INI sets the quality     ", element("opt-quality").value, "fixed");
  check("INI ticks keep           ", element("opt-keep").checked, true);
  check("INI ticks crop           ", element("opt-crop").checked, true);

  gui.seedOptionsFromConfig("wopt");
  check("INI sets watch codec     ", element("wopt-codec").value, "av1");
  check("INI sets watch bit depth ", element("wopt-bitdepth").value, "8");
  check("INI leaves watch shutdown", element("wopt-shutdown").value, "");

  finish();
}

main();
