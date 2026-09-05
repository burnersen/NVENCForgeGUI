// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

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
    // Rollmaße: ein Kasten im Fake hat keine Größe, also stehen sie hier als
    // echte Zahlen. Ohne sie rechnete das Mitrollen mit Stand-ins und käme auf
    // NaN — die Prüfung sähe grün aus und hätte nichts geprüft.
    offsetTop: 0, offsetHeight: 0, clientHeight: 0,
    // rectTop ist die Lage im Bild, die eine Prüfung stellen kann.
    // getBoundingClientRect gibt sie zurück, statt einen Stand-in zu
    // liefern: mit einer Funktion darin würde jede Rechnung zu NaN, und die
    // Prüfung sähe grün aus, ohne etwas geprüft zu haben.
    rectTop: 0,
    getBoundingClientRect() {
      const top = store.rectTop || 0;
      const height = store.offsetHeight || 0;
      return { top, height, bottom: top + height, left: 0, right: 0, width: 0 };
    },
    // querySelector sucht wirklich, statt einen Stand-in zurückzugeben: ob das
    // Mitrollen die LAUFENDE Zeile findet, ist genau die Frage. Gesucht wird
    // nach Klassen (".item.active"), mehr braucht das Fenster hier nicht.
    querySelector(selector) {
      const wanted = String(selector).split(".").filter(Boolean);
      const hit = store.children.find((child) => {
        const classes = String((child && child.className) || "").split(" ");
        return wanted.every((cls) => classes.includes(cls));
      });
      return hit || null;
    },
    classList: { add() {}, remove() {}, toggle() {}, contains: () => false },
    // Taking lines back is counted: that is how a check can see whether a
    // redraw really deleted something — the log's own way of overwriting
    // itself, and the one thing two converters can ruin for each other.
    removed: 0,
    removeChild() { store.removed++; store.children.pop(); },
    // Lines added are counted as well: with two logs in the window, WHICH box
    // a line landed in is the only way to see that they really are separate.
    appended: 0,
    appendChild(child) { store.appended++; store.children.push(child); return child; },
    // Attributes are kept rather than swallowed: whether a button says it is
    // switched on is the only thing that tells the user which theme is
    // running, and a stand-in that forgets it could not check that.
    attrs: {},
    setAttribute(name, value) { store.attrs[name] = value; },
    getAttribute(name) { return store.attrs[name]; },
    // Registrierte Ereignisse werden aufbewahrt statt verschluckt. Ein Teil der
    // Oberfläche hängt nämlich an addEventListener statt an onchange — etwa
    // alles, was eine Änderung in die INI zurückschreibt. Solange der Ersatz
    // hier nichts merkte, war genau dieser Teil ungeprüft, und ein Element ohne
    // jede Verdrahtung sah aus wie eines mit.
    listeners: [],
    addEventListener(name, fn) { store.listeners.push({ name, fn }); },
    // dispatch löst aus, was wirklich registriert wurde — das Gegenstück zum
    // Klick auf einen onclick-Knopf.
    dispatch(name, event) {
      store.listeners
        .filter((entry) => entry.name === name)
        .forEach((entry) => entry.fn(event || { target: store }));
    }
  };
  return new Proxy(store, {
    get: (target, prop) => (prop in target ? target[prop] : () => fakeElement("child")),
    set: (target, prop, value) => {
      target[prop] = value;
      // Ein geleerter Kasten verliert im Browser seine Rollstellung und
      // seine Kinder. Ohne das hier sähe eine Prüfung, die genau das
      // absichern soll, einen Schaden nie: der Wert bliebe einfach stehen.
      if (prop === "innerHTML" && value === "") {
        target.children = [];
        target.scrollTop = 0;
      }
      return true;
    }
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
  const calls = { answers: [], answerSlots: [], runs: [], joinSorts: [], stops: [], srtSaves: [], themes: [], clipboard: [], savingsResets: [], frame: [], profileSaves: [], profileDeletes: [], opened: [], shutdownWishes: [], shutdownCancels: [], updateChecks: [], updateInstalls: [], settingSaves: [], profileApplies: [], drops: [] };
  // Was die Go-Seite auf DropPendingFile antworten soll. Standard: die Datei
  // wartete noch und wurde gestrichen. Auf false gestellt heißt: zu spät, sie
  // läuft schon — der Fall, in dem die Zeile stehen bleiben MUSS.
  let dropReply = true;
  // Was die Go-Seite zum Selbst-Update antworten soll; je Prüfung gesetzt. Ein
  // Error steht für "der Aufruf scheitert" — der Fall, in dem das Fenster
  // seinen Knopf sonst für immer gesperrt ließe.
  let updateCheckReply = { newer: false, current: "1.1.0", latest: "v1.1.0", note: "This is the newest release (v1.1.0)." };
  let updateInstallReply = { installed: true, restarting: true, version: "v1.2.0", message: "installed." };
  // Was die Go-Seite über das Ausschalten antworten soll. Sie führt den Stand,
  // das Fenster zeigt ihn nur — eine Prüfung muss also beides trennen können.
  let shutdownReply = { armed: false, counting: false, seconds: 60, note: "" };
  // Die Profilablage. Sie führt wirklich Buch, statt nur zu bestätigen:
  // Eine Prüfung muss sehen, was NACH dem Speichern im Auswahlfeld steht.
  let profileList = [];
  // Was die Go-Seite über die INI antworten soll; je Prüfung gesetzt.
  // Getrennt gehalten: GetConfigView ist die kurze Sicht für die
  // Konvertieren-Seite, GetSettingsFile die vollständige Liste der
  // Einstellungsseite. Ein Profil nimmt die zweite mit.
  let configReply = { found: false, note: "not set" };
  let settingsFileReply = { found: false, path: "", note: "not set", settings: [] };
  const sortProfiles = (list) =>
    list.slice().sort((a, b) => a.name.toLowerCase() < b.name.toLowerCase() ? -1 : 1);

  // Was das Sparbuch der Go-Seite antworten soll; je Prüfung gesetzt.
  let savingsReply = { totalMB: 0, totalFiles: 0, totalSeconds: 0 };
  // What GetSRTCleaner should answer; set per check with setSRTReply.
  let srtReply = { found: false, path: "", note: "not set", phrases: [] };
  // Sorting the join list happens in Go (joinfiles.go) and is tested there.
  // Here only the answer is staged, so a check can see what the WINDOW does
  // with it — and, just as important, which paths it hands over.
  let joinReply = [];
  const windowStub = {
    addEventListener() {},
    runtime: {
      OnFileDrop() {}, EventsOn() {},
      // Links MUESSEN hier durch: ein <a href> wuerde die Seite IM Fenster
      // laden und das Programm waere weg.
      BrowserOpenURL(url) { calls.opened.push(url); },
      ClipboardSetText(text) { calls.clipboard.push(text); return Promise.resolve(); }
    },
    go: {
      main: {
        App: {
          // The slot says WHICH converter asked — with several running, an
          // answer sent to the wrong one would pull the wrong tracks.
          AnswerQuestion(slot, text) { calls.answers.push(text); calls.answerSlots.push(slot); return Promise.resolve(); },
          StartRun(request) { calls.runs.push(request); return Promise.resolve(); },
          // Welcher BEREICH angehalten wird, ist der ganze Punkt: ein Stapel
          // abbrechen darf den beobachteten Ordner nicht mitreißen.
          StopArea(area) { calls.stops.push(area); return Promise.resolve(); },
          StopSlot(slot) { calls.stops.push(slot); return Promise.resolve(); },
          DropPendingFile(area, path) {
            calls.drops.push({ area, path });
            return Promise.resolve(dropReply);
          },
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
          GetProfiles() { return Promise.resolve(sortProfiles(profileList)); },
          // Die INI. Was das Fenster hineinschreibt, wird mitgeschrieben:
          // Seit die Konvertieren-Seite jede Änderung zurückschreibt, ist
          // GENAU DAS die Frage — landet der richtige Schlüssel mit dem
          // richtigen Wert in der Datei, und wird nichts geschrieben, was die
          // Datei gar nicht kennt?
          GetConfigView() { return Promise.resolve(configReply); },
          GetSettingsFile() { return Promise.resolve(settingsFileReply); },
          SaveSettings(values) {
            calls.settingSaves.push(values);
            return Promise.resolve({ written: Object.keys(values).length, backupPath: "X:\\tools\\NVENCForge_Config.ini.bak" });
          },
          // Ein Profil setzt die ganze Datei. Der NAME reicht der Go-Seite —
          // sie hat den Abzug selbst gespeichert.
          ApplyProfile(name) {
            calls.profileApplies.push(name);
            return Promise.resolve({ written: 30, note: "" });
          },
          SaveProfile(profile) {
            calls.profileSaves.push(profile);
            profileList = profileList.filter((kept) => kept.name !== profile.name).concat([profile]);
            return Promise.resolve(sortProfiles(profileList));
          },
          DeleteProfile(name) {
            calls.profileDeletes.push(name);
            profileList = profileList.filter((kept) => kept.name !== name);
            return Promise.resolve(sortProfiles(profileList));
          },
          // Das Ausschalten des Rechners entscheidet die Go-Seite. Hier wird
          // festgehalten, WAS das Fenster ihr dazu schickt — ein Häkchen, das
          // nie ankommt, wäre sonst nicht von einem wirkenden zu unterscheiden.
          SetShutdownWhenDone(on) {
            calls.shutdownWishes.push(on);
            shutdownReply = { ...shutdownReply, armed: on };
            return Promise.resolve(shutdownReply);
          },
          CancelShutdown() {
            calls.shutdownCancels.push(true);
            shutdownReply = { ...shutdownReply, armed: false, counting: false };
            return Promise.resolve(shutdownReply);
          },
          ShutdownStatus() { return Promise.resolve(shutdownReply); },
          // Das Sparbuch führt die Go-Seite. Das Fenster zeigt nur an — und
          // WAS es beim Zurücksetzen wirklich losschickt, steht hier.
          GetSavings() { return Promise.resolve(savingsReply); },
          ResetSavings() {
            calls.savingsResets.push(true);
            savingsReply = { totalMB: 0, totalFiles: 0, totalSeconds: 0 };
            return Promise.resolve(savingsReply);
          },
          // Der Fensterrahmen: Prozent im Titel, Balken im Taskleisten-Knopf,
          // Blinken am Ende. Aufgezeichnet wird die REIHENFOLGE — dass am Ende
          // eines Stapels wirklich abgeräumt und gemeldet wird, ist der Punkt.
          ShowProgress(percent) { calls.frame.push("percent:" + percent); return Promise.resolve(); },
          ShowBusy() { calls.frame.push("busy"); return Promise.resolve(); },
          HideProgress() { calls.frame.push("idle"); return Promise.resolve(); },
          SignalDone() { calls.frame.push("done"); return Promise.resolve(); },
          SaveTheme(theme) { calls.themes.push(theme); return Promise.resolve(); },
          GetSRTCleaner() { return Promise.resolve(srtReply); },
          SaveSRTCleaner(phrases) {
            calls.srtSaves.push(phrases);
            return Promise.resolve({ written: phrases.length });
          },
          // Looking for a new version and installing one are two separate
          // calls on purpose, and they are counted separately here: an install
          // that goes off without a look before it would replace the running
          // program on nothing but an old button label.
          CheckForUpdate() {
            calls.updateChecks.push(true);
            if (updateCheckReply instanceof Error) return Promise.reject(updateCheckReply);
            return Promise.resolve(updateCheckReply);
          },
          InstallUpdate() {
            calls.updateInstalls.push(true);
            if (updateInstallReply instanceof Error) return Promise.reject(updateInstallReply);
            return Promise.resolve(updateInstallReply);
          }
        }
      }
    }
  };

  const exported = new Function(
    "window", "document",
    scriptText + "\n;return { wire, onConverterEvent, state, applyConfig, refreshFromConfig, bitrateCapKey, HELP," +
    " settingModel, looksInvalid, settingHelp, editSetting, revertSetting, restoreDefaults," +
    " changedValues, defaultFor, noteGPUAdvice, renderSettings, showPage, log, note," +
    " onQuestion, sendAnswer, askSelection, isExtraOption, isToolRun, collectRequest, resetProgress," +
    " readTrackLabel, trackFacts, buildTrackRule, applyTrackRule, describeTrackRule, setAllTicks," +
    " justTheName, joinJobs," +
    " updateButtons, afterJoinChange, addJoinPaths, joinOfKind," +
    " joinReady, joinRunFiles, applyCropCapability, showConverter, onWatchFiles, maybeStartWatchRun, showWatch, clearWatchArea, runWentThroughCleanly, clearFinishedList, stopWatching, stopWatchRun, collectWatchRequest, onQueueState, renderWatchSummary, isWatchSlot, WATCH_SLOT, limitParallelChoice, watchNote," +
    " startBatch, clearProgress, updateOverall, batchProgress, updateFrame, stopSlot, stop, start, renderLanes, copyAreaLog," +
    " loadSRTCleaner, renderSRTCleaner, addSRTPhrase, saveSRTPhrases, srtSignature," +
    " joinMode, applyJoinMode, JOIN_MODES, showAbout, openLink, LINKS, loadProfiles, renderProfiles, chooseProfile, saveProfile, deleteProfile, applyProfile, profileFromOptions, profileNamed, applyTheme, chooseTheme, THEMES," +
    " splitMode, applySplitMode, SPLIT_MODES," +
    " areaOf, areaOfSlot, areaNameOfSlot, anyRunning, addItems, afterQueueChange, dropFinishedEntries, removeFromQueue," +
    " watchScrolling, followsNow, followLog, followQueueBox, followEverything, listBoxOf, FOLLOW_PAUSE_MS," +
    " AREA_NAMES, AREA_SLOTS, finishArea, clearLanes, renderList, showFinalSummary, el," +
    " showSavings, resetSavings, applySettingsFilter, settingMatches, sectionId, onRunState," +
    " setShutdownWish, onShutdownState, cancelShutdown, showShutdownAlert," +
    " checkUpdate, installUpdate," +
    " INI_MIRROR, seedOptionsFromConfig, rememberOption, rememberCQ, rememberBitrate, saveOneSetting,"
    + " noteCodecReach, afterOptionsChanged," +
    " settingsSnapshot, ensureSettingsFile, applyProfileSettings, noteShutdownFromConfig, refreshConfig };"
  )(windowStub, documentStub);

  return {
    gui: exported,
    html,
    calls,
    created: createdElements,
    element: (id) => documentStub.getElementById(id),
    // setQueryAll stellt die Ticks, die askSelection einsammelt.
    setQueryAll: (selector, list) => selectorLists.set(selector, list),
    // setDropReply legt fest, ob eine Datei noch aus der Warteschlange kam.
    setDropReply: (ok) => { dropReply = ok; },
    // setJoinReply legt fest, was die Go-Seite auf SortJoinFiles antworten soll.
    setJoinReply: (files) => { joinReply = files; },
    // setSRTReply legt fest, was die Go-Seite auf GetSRTCleaner antworten soll.
    setSRTReply: (view) => { srtReply = view; },
    // setConfigReply legt fest, was GetConfigView melden soll.
    setConfigReply: (view) => { configReply = view; },
    // setSettingsFileReply legt die vollständige Einstellungsliste fest.
    setSettingsFileReply: (file) => { settingsFileReply = file; },
    // setSavingsReply legt fest, was die Go-Seite als Sparbuch meldet.
    setSavingsReply: (report) => { savingsReply = report; },
    // setProfiles legt fest, was die Go-Seite als gespeicherte Sätze meldet.
    setProfiles: (list) => { profileList = list.slice(); },
    // setUpdateCheckReply legt fest, was CheckForUpdate antworten soll.
    setUpdateCheckReply: (answer) => { updateCheckReply = answer; },
    // setUpdateInstallReply legt fest, was InstallUpdate antworten soll.
    setUpdateInstallReply: (answer) => { updateInstallReply = answer; }
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
