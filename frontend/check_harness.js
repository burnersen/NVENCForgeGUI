// check_harness.js — runs the window's own script without a browser.
//
// Shared by progress_check.js and options_check.js. The logic under test lives
// inside index.html and normally only runs inside the web view, so without this
// every wrong label or bar would cost a full conversion to notice.
//
// It deliberately loads the shipped index.html, not a copy: a copy would drift
// and then prove nothing.
const fs = require("fs");
const path = require("path");

// A Proxy saves rebuilding the DOM: known properties come from the object,
// anything else turns into a harmless function returning another stand-in.
function fakeElement(id) {
  const store = {
    id, textContent: "", innerHTML: "", value: "", checked: false, hidden: false,
    className: "", disabled: false, max: 0, placeholder: "", style: {}, dataset: {},
    children: [], scrollTop: 0, scrollHeight: 0,
    classList: { add() {}, remove() {}, toggle() {}, contains: () => false }
  };
  return new Proxy(store, {
    get: (target, prop) => (prop in target ? target[prop] : () => fakeElement("child")),
    set: (target, prop, value) => { target[prop] = value; return true; }
  });
}

// loadGui evaluates the script block and hands back what the checks need.
// boot() hangs on DOMContentLoaded, which never fires here, so evaluating only
// defines things instead of trying to talk to a converter that is not running.
function loadGui() {
  const htmlPath = path.join(__dirname, "dist", "index.html");
  const html = fs.readFileSync(htmlPath, "utf8");
  const scriptText = html.slice(
    html.lastIndexOf("<script>") + "<script>".length,
    html.lastIndexOf("</script>")
  );

  const elements = new Map();
  const documentStub = {
    getElementById(id) {
      if (!elements.has(id)) elements.set(id, fakeElement(id));
      return elements.get(id);
    },
    createElement: () => fakeElement("new"),
    createDocumentFragment: () => fakeElement("fragment"),
    createTextNode: (text) => fakeElement("text-" + text),
    // Same stand-in for the same selector, so a check can look at what the
    // code did to it.
    querySelector(selector) {
      if (!elements.has(selector)) elements.set(selector, fakeElement(selector));
      return elements.get(selector);
    },
    querySelectorAll: () => [],
    addEventListener() {},
    body: fakeElement("body")
  };
  const windowStub = { addEventListener() {}, runtime: { OnFileDrop() {}, EventsOn() {} } };

  const exported = new Function(
    "window", "document",
    scriptText + "\n;return { onConverterEvent, state, applyConfig, refreshFromConfig, bitrateCapKey, HELP," +
    " settingModel, looksInvalid, settingHelp, editSetting, revertSetting, restoreDefaults," +
    " changedValues, defaultFor, noteGPUAdvice, renderSettings, showPage, log };"
  )(windowStub, documentStub);

  return {
    gui: exported,
    html,
    element: (id) => documentStub.getElementById(id)
  };
}

// createChecker keeps the score. Every check prints one line, so a failure is
// readable without a test framework.
function createChecker() {
  let failed = 0;
  return {
    check(what, got, want) {
      const ok = got === want;
      if (!ok) failed++;
      console.log((ok ? "  ok   " : "  FAIL ") + what + ": " + got + (ok ? "" : "  (expected " + want + ")"));
    },
    contains(what, haystack, needle) {
      const ok = String(haystack).includes(needle);
      if (!ok) failed++;
      console.log((ok ? "  ok   " : "  FAIL ") + what + (ok ? "" : ": " + haystack + "  (should contain " + needle + ")"));
    },
    finish() {
      console.log("\n" + (failed === 0 ? "all checks passed" : failed + " check(s) FAILED"));
      process.exit(failed === 0 ? 0 : 1);
    }
  };
}

module.exports = { loadGui, createChecker };
