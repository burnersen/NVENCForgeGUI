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
    classList: { add() {}, remove() {}, toggle() {}, contains: () => false },
    // Taking lines back is counted: that is how a check can see whether a
    // redraw really deleted something — the log's own way of overwriting
    // itself, and the one thing two converters can ruin for each other.
    removed: 0,
    removeChild() { store.removed++; },
    // Attributes are kept rather than swallowed: whether a button says it is
    // switched on is the only thing that tells the user which theme is
    // running, and a stand-in that forgets it could not check that.
    attrs: {},
    setAttribute(name, value) { store.attrs[name] = value; },
    getAttribute(name) { return store.attrs[name]; }
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
    // Erzeugte Elemente werden mitgeschrieben: Nur so lässt sich prüfen, wie
    // eine Liste ANKOMMT, die das Fenster erst zur Laufzeit zusammenbaut.
    createElement: (tag) => {
      const created = fakeElement("new-" + tag);
      createdElements.push(created);
      return created;
    },
    createDocumentFragment: () => fakeElement("fragment"),
    createTextNode: (text) => fakeElement("text-" + text),
    // Same stand-in for the same selector, so a check can look at what the
    // code did to it.
    querySelector(selector) {
      if (!elements.has(selector)) elements.set(selector, fakeElement(selector));
      return elements.get(selector);
    },
    // Lists can be prepared per selector (see setQueryAll below). Anything not
    // prepared stays empty, which is what the older checks rely on.
    querySelectorAll: (selector) => selectorLists.get(selector) || [],
    addEventListener() {},
    body: fakeElement("body"),
    // applyTheme writes the chosen theme onto <html>. It goes through
    // getElementById so that a check asking for "html" gets the very same
    // stand-in — two separate ones would look fine and prove nothing.
    get documentElement() { return documentStub.getElementById("html"); }
  };
  const selectorLists = new Map();
  const createdElements = [];

  // Everything the window would hand to the Go side is recorded instead. That
  // is how a check can see WHICH answer a button really sends — the one thing
  // that decides whether the user gets the tracks they picked.
  const calls = { answers: [], answerSlots: [], runs: [], joinSorts: [], stops: [], srtSaves: [], themes: [] };
  // What GetSRTCleaner should answer; set per check with setSRTReply.
  let srtReply = { found: false, path: "", note: "not set", phrases: [] };
  // Sorting the join list happens in Go (joinfiles.go) and is tested there.
  // Here only the answer is staged, so a check can see what the WINDOW does
  // with it — and, just as important, which paths it hands over.
  let joinReply = [];
  const windowStub = {
    addEventListener() {},
    runtime: { OnFileDrop() {}, EventsOn() {} },
    go: {
      main: {
        App: {
          // The slot says WHICH converter asked — with several running, an
          // answer sent to the wrong one would pull the wrong tracks.
          AnswerQuestion(slot, text) { calls.answers.push(text); calls.answerSlots.push(slot); return Promise.resolve(); },
          StartRun(request) { calls.runs.push(request); return Promise.resolve(); },
          StopRun() { calls.stops.push("all"); return Promise.resolve(); },
          StopSlot(slot) { calls.stops.push(slot); return Promise.resolve(); },
          SortJoinFiles(paths) { calls.joinSorts.push(paths); return Promise.resolve(joinReply); },
          PickJoinFiles() { return Promise.resolve(joinReply); },
          StartWatching(folder) { return Promise.resolve({ watching: true, folder }); },
          StopWatching() { return Promise.resolve({ watching: false, folder: "" }); },
          PickWatchFolder() { return Promise.resolve(""); },
          // The phrase list of the subtitle cleaner. What the window SENDS is
          // recorded: a phrase that changes on its way to the file would strip
          // something other than the list on screen promises.
          // Which theme is written down decides what the window looks like on
          // the NEXT start — a wrong value here is invisible until then.
          GetTheme() { return Promise.resolve("dark"); },
          SaveTheme(theme) { calls.themes.push(theme); return Promise.resolve(); },
          GetSRTCleaner() { return Promise.resolve(srtReply); },
          SaveSRTCleaner(phrases) {
            calls.srtSaves.push(phrases);
            return Promise.resolve({ written: phrases.length });
          }
        }
      }
    }
  };

  const exported = new Function(
    "window", "document",
    scriptText + "\n;return { onConverterEvent, state, applyConfig, refreshFromConfig, bitrateCapKey, HELP," +
    " settingModel, looksInvalid, settingHelp, editSetting, revertSetting, restoreDefaults," +
    " changedValues, defaultFor, noteGPUAdvice, renderSettings, showPage, log," +
    " onQuestion, sendAnswer, askSelection, isExtraOption, isToolRun, collectRequest, resetProgress," +
    " runUsesQueue, updateButtons, afterJoinChange, addJoinPaths, joinBase, joinOfKind," +
    " joinReady, joinRunFiles, onWatchFiles, maybeStartWatchRun, showWatch, toggleWatch, onQueueState," +
    " startBatch, clearProgress, updateOverall, stopSlot, stop, renderLanes," +
    " loadSRTCleaner, renderSRTCleaner, addSRTPhrase, saveSRTPhrases, srtSignature," +
    " joinMode, isJoinMode, applyJoinMode, JOIN_MODES, applyTheme, chooseTheme, THEMES," +
    " onRunState };"
  )(windowStub, documentStub);

  return {
    gui: exported,
    html,
    calls,
    created: createdElements,
    element: (id) => documentStub.getElementById(id),
    // setQueryAll stellt die Ticks, die askSelection einsammelt.
    setQueryAll: (selector, list) => selectorLists.set(selector, list),
    // setJoinReply legt fest, was die Go-Seite auf SortJoinFiles antworten soll.
    setJoinReply: (files) => { joinReply = files; },
    // setSRTReply legt fest, was die Go-Seite auf GetSRTCleaner antworten soll.
    setSRTReply: (view) => { srtReply = view; }
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
