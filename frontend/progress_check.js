// progress_check.js — checks the progress display without converting anything.
//
// Run it with:  node frontend\progress_check.js
//
// The display logic lives inside index.html and only ever runs in the web view,
// so a wrong bar used to cost a full conversion to notice. This script loads the
// script block straight out of the shipped index.html, hands it a stand-in for
// the browser and feeds it the same events the converter sends. Exit code 0
// means every case matched.
//
// It deliberately reads the shipped file, not a copy: a copy would drift and
// then prove nothing.
const fs = require("fs");
const path = require("path");

const htmlPath = path.join(__dirname, "dist", "index.html");
const html = fs.readFileSync(htmlPath, "utf8");
const scriptText = html.slice(
  html.lastIndexOf("<script>") + "<script>".length,
  html.lastIndexOf("</script>")
);

// A Proxy saves rebuilding the DOM: known properties come from the object,
// anything else turns into a harmless function returning another stand-in.
function fakeElement(id) {
  const store = {
    id, textContent: "", innerHTML: "", value: "", checked: false, hidden: false,
    className: "", disabled: false, max: 0, style: {}, dataset: {}, children: [],
    scrollTop: 0, scrollHeight: 0,
    classList: { add() {}, remove() {}, toggle() {}, contains: () => false }
  };
  return new Proxy(store, {
    get: (target, prop) => (prop in target ? target[prop] : () => fakeElement("child")),
    set: (target, prop, value) => { target[prop] = value; return true; }
  });
}
const elements = new Map();
const documentStub = {
  getElementById(id) {
    if (!elements.has(id)) elements.set(id, fakeElement(id));
    return elements.get(id);
  },
  createElement: () => fakeElement("new"),
  addEventListener() {},
  body: fakeElement("body")
};
// boot() hangs on DOMContentLoaded, which never fires here — so evaluating the
// script only defines things instead of trying to talk to a converter.
const windowStub = { addEventListener() {}, runtime: { OnFileDrop() {}, EventsOn() {} } };

const gui = new Function(
  "window", "document",
  scriptText + "\n;return { onConverterEvent, state };"
)(windowStub, documentStub);

const fileBar = () => documentStub.getElementById("pct-file").textContent;
const overallBar = () => documentStub.getElementById("pct-all").textContent;

let failed = 0;
function check(what, got, want) {
  const ok = got === want;
  if (!ok) failed++;
  console.log((ok ? "  ok   " : "  FAIL ") + what + ": " + got + (ok ? "" : "  (expected " + want + ")"));
}

const entry = (name, sizeMB) =>
  ({ path: "X:\\test\\" + name, name, folder: "X:\\test", sizeMB, status: "", note: "" });

// The run event clears the queue notes, so the queue is restored afterwards —
// in the real window the queue is filled by the user, not by an event.
function startRun(queue) {
  gui.state.queue = queue;
  gui.onConverterEvent({ ev: "run", mode: "convert", codec: "h265", encoder: "nvenc", files: queue.length, version: "test" });
  gui.state.queue = queue;
  gui.state.totalMB = queue.reduce((sum, e) => sum + e.sizeMB, 0);
}
const fileEvent = (index, total, e) =>
  ({ ev: "file", index, total, name: e.name, path: e.path, in_mb: e.sizeMB });
const resultEvent = (index, status, e, extra = {}) =>
  Object.assign({ ev: "result", index, status, name: e.name, in_mb: e.sizeMB, out_mb: 40, saved_mb: 60, saved_pct: 60 }, extra);

console.log("\n=== one file whose progress stops at 99.1 % ===");
let queue = [entry("film.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent({ ev: "progress", pct: 50, eta: "0:30", speed: "2.0x", fps: "120", bitrate: "4000k", est_mb: 40 });
check("half way                  ", fileBar(), "50.0 %");
gui.onConverterEvent({ ev: "progress", pct: 99.1, eta: "0:00", speed: "2.0x", fps: "120", bitrate: "4000k", est_mb: 40 });
check("last progress event       ", fileBar(), "99.1 %");
gui.onConverterEvent(resultEvent(1, "success", queue[0]));
check("file bar after the result ", fileBar(), "100.0 %");
check("overall after the result  ", overallBar(), "100.0 %");
gui.onConverterEvent({ ev: "summary", files: 1, success: 1, skipped: 0, failed: 0, saved_mb: 60, elapsed_sec: 65 });
check("file bar after the summary", fileBar(), "100.0 %");

console.log("\n=== two files: the overall bar must not count a file twice ===");
queue = [entry("a.mkv", 100), entry("b.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 2, queue[0]));
gui.onConverterEvent({ ev: "progress", pct: 99.2, est_mb: 40 });
gui.onConverterEvent(resultEvent(1, "success", queue[0]));
check("file 1 done -> file bar   ", fileBar(), "100.0 %");
check("file 1 done -> overall    ", overallBar(), "50.0 %");
gui.onConverterEvent(fileEvent(2, 2, queue[1]));
check("file 2 starts -> file bar ", fileBar(), "0.0 %");
check("file 2 starts -> overall  ", overallBar(), "50.0 %");
gui.onConverterEvent({ ev: "progress", pct: 50, est_mb: 40 });
check("file 2 half -> overall    ", overallBar(), "75.0 %");
gui.onConverterEvent(resultEvent(2, "success", queue[1]));
check("file 2 done -> overall    ", overallBar(), "100.0 %");

console.log("\n=== failed and stopped runs must NOT jump to 100 % ===");
queue = [entry("broken.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent({ ev: "progress", pct: 42, est_mb: 20 });
gui.onConverterEvent(resultEvent(1, "failed", queue[0], { out_mb: 0, saved_mb: 0, saved_pct: 0, message: "encoder error" }));
check("failed -> bar stays       ", fileBar(), "42.0 %");

queue = [entry("stopped.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent({ ev: "progress", pct: 27.3, est_mb: 12 });
gui.onConverterEvent(resultEvent(1, "preview", queue[0], { out_mb: 0, saved_mb: 0, saved_pct: 0 }));
check("stopped -> bar stays      ", fileBar(), "27.3 %");

console.log("\n=== a skipped file counts as done ===");
queue = [entry("already-h265.mkv", 100)];
startRun(queue);
gui.onConverterEvent(fileEvent(1, 1, queue[0]));
gui.onConverterEvent(resultEvent(1, "skipped", queue[0], { in_mb: 0, out_mb: 0, saved_mb: 0, saved_pct: 0 }));
check("skipped -> file bar       ", fileBar(), "100.0 %");
check("skipped -> overall        ", overallBar(), "100.0 %");

console.log("\n" + (failed === 0 ? "all checks passed" : failed + " check(s) FAILED"));
process.exit(failed === 0 ? 0 : 1);
