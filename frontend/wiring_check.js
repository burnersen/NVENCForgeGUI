// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// wiring_check.js — does the window's script talk to elements that exist?
//
// This check exists because of a real failure, and because none of the other
// checks could have caught it. The stand-in document in check_harness.js hands
// back an element for ANY id ever asked for; the real browser hands back null.
// So a button that was removed from the page while its wiring stayed behind
// looks perfectly fine in every other check here — and in the real window it
// throws on start-up, which stops wire() halfway through. Everything after the
// broken line is then never wired: buttons stay disabled, and file dropping is
// never taken over from the web view, so dropped videos simply start playing.
//
// The check is therefore deliberately static. It reads the shipped index.html
// as text and compares two lists: the ids the markup defines, and the ids the
// script reaches for. It needs no stand-ins at all, which is exactly why it
// can see what they hide.
const fs = require("fs");
const path = require("path");
const { createChecker } = require("./check_harness");

const checker = createChecker();
const html = fs.readFileSync(path.join(__dirname, "dist", "index.html"), "utf8");

// Everything before the last <script> is the page; everything after it is the
// code. Splitting there keeps an id mentioned in a string out of the markup
// list and vice versa.
const scriptAt = html.lastIndexOf("<script>");
if (scriptAt === -1) throw new Error("no script block in index.html");
const markup = html.slice(0, scriptAt);
const script = html.slice(scriptAt);

const defined = new Set([...markup.matchAll(/id="([^"]+)"/g)].map((m) => m[1]));

// Only literal ids can be checked here. Something like $("field-" + key) is
// built at runtime; the ids an area builds for itself are checked further
// down, by name.
const asked = [...new Set([...script.matchAll(/\$\("([^"]+)"\)/g)].map((m) => m[1]))];
const selected = [...new Set(
  [...script.matchAll(/querySelector\("#([A-Za-z0-9_-]+)"\)/g)].map((m) => m[1])
)];

console.log("\nThe script only reaches for elements the page really has");
console.log("  " + defined.size + " ids in the page, " + asked.length + " asked for by name");

const missing = asked.filter((id) => !defined.has(id));
if (missing.length) {
  console.log("  these do not exist: " + missing.join(", "));
  console.log("  (in the real window each of them is null, and the line throws)");
}
checker.check("every $(\"id\") exists      ", missing.length, 0);

const missingSelectors = selected.filter((id) => !defined.has(id));
if (missingSelectors.length) console.log("  missing selectors: " + missingSelectors.join(", "));
checker.check("every #selector exists    ", missingSelectors.length, 0);

/* Every area builds its own element ids: el(area, "lanes") asks for
   "convert-lanes", "split-lanes" and so on. Those never appear as a literal
   $("...") anywhere, so the list above cannot see them — this is the new blind
   spot, and it is the same kind of hole the whole file was written for. So the
   ids are spelled out here, area by area, and matched against the page.

   The lists mirror what the script really does: hasQueue decides whether an
   area has a list of files, hasBar whether it has an overall bar. */
const AREAS = {
  convert: { queue: true, bar: true, runs: true },
  split:   { queue: true, bar: true, runs: true },
  join:    { queue: false, bar: false, runs: true },
  watch:   { queue: false, bar: false, runs: false }
};

console.log("\nEvery area owns the elements its own code reaches for");
for (const [area, has] of Object.entries(AREAS)) {
  // The display each area needs whatever it does: its own log and its own
  // progress area, with a running total under it.
  const roles = ["logbox", "autoscroll", "log-copied", "error", "summary", "lanes"];
  const buttons = ["log-copy", "log-clear"];
  if (has.runs) {
    roles.push("stop-hint");
    buttons.push("start", "stop");
  }
  if (has.queue) {
    roles.push("queue", "queue-info", "dropzone");
    buttons.push("files", "folder", "clear");
  }
  if (has.bar) roles.push("bar", "pct", "barline");

  const wanted = roles.map((role) => area + "-" + role)
    .concat(buttons.map((name) => "btn-" + area + "-" + name));
  const absent = wanted.filter((id) => !defined.has(id));
  if (absent.length) console.log("  " + area + " is missing: " + absent.join(", "));
  checker.check("  " + area.padEnd(18), absent.length, 0);
}

// The join page keeps a list of a different shape, so it does not follow the
// naming above. Named separately rather than left out.
console.log("\nThe join page keeps its own kind of list");
for (const id of ["join-list", "join-dropzone", "join-info", "join-result",
                  "btn-join-files", "btn-join-clear"]) {
  checker.check("  " + id.padEnd(18), defined.has(id), true);
}

// The watched folder is worked by its own three buttons, not by a start button.
console.log("\nThe watched folder is switched on and off by hand");
for (const id of ["btn-watch-pick", "btn-watch-start", "btn-watch-stop", "btn-watch-stop-run"]) {
  checker.check("  " + id.padEnd(18), defined.has(id), true);
}

// The handover that stops the web view from opening dropped files itself. It
// is not a button, so nothing else here would miss it - and when it is gone,
// dropping a video plays it instead of queueing it.
console.log("\nFile dropping is taken over from the web view");
checker.check("OnFileDrop is registered ", /runtime\.OnFileDrop\(/.test(script), true);
checker.check("and drops reach a list   ", /addItems\(|addJoinPaths\(/.test(script), true);

/* Every display exists four times over now, and they are styled by class
   instead of by id — one rule for all of them. That moves the old danger
   rather than removing it: a copy that forgets its class is styled by nothing
   at all, and CSS says nothing when a rule does not apply. That is exactly how
   the watched folder's log first turned up as a white, sizeless box with
   unreadable terminal colours in it.

   So every copy is checked for its class, and every class for a rule that
   actually exists. */
console.log("\nEvery copy carries the class that styles it");
const css = markup.slice(markup.indexOf("<style>"), markup.indexOf("</style>"));

function tagOf(id) {
  const found = markup.match(new RegExp("<[^>]*\\sid=\"" + id + "\"[^>]*>"));
  return found ? found[0] : "";
}
function wears(id, className) {
  const attribute = tagOf(id).match(/class="([^"]*)"/);
  return !!attribute && attribute[1].split(/\s+/).includes(className);
}

const styled = [
  ["convert-logbox", "logbox"], ["split-logbox", "logbox"],
  ["join-logbox", "logbox"], ["watch-logbox", "logbox"],
  ["convert-lanes", "lanes"], ["split-lanes", "lanes"],
  ["join-lanes", "lanes"], ["watch-lanes", "lanes"],
  ["convert-summary", "summary"], ["split-summary", "summary"],
  ["join-summary", "summary"], ["watch-summary", "summary"],
  ["convert-queue", "filelist"], ["split-queue", "filelist"], ["join-list", "filelist"],
  ["convert-dropzone", "dropzone"], ["split-dropzone", "dropzone"], ["join-dropzone", "dropzone"]
];
const bare = styled.filter(([id, className]) => !wears(id, className));
if (bare.length) console.log("  without their class: " + bare.map(([id]) => id).join(", "));
checker.check("every copy is dressed    ", bare.length, 0);

const classes = [...new Set(styled.map(([, className]) => className))];
const unruled = classes.filter((className) => !css.includes("." + className));
if (unruled.length) console.log("  classes with no rule: " + unruled.join(", "));
checker.check("and every class has rules", unruled.length, 0);

// The result line is the one thing a finished run leaves behind, so the rule
// that makes it stand out has to exist — the script writes that class on.
checker.check("the result line has a look", css.includes(".summary.final"), true);
checker.check("and the script puts it on ", script.includes('"summary final"'), true);

// Second guard, and a different one: actually RUN wire(). The stand-in hands
// back an element for any id, so this cannot see a missing element - but it
// does see a handler pointing at a function that no longer exists, which is
// the same kind of half-finished rename and has the same consequence: every
// line after it is never wired.
console.log("\nwire() gets to the end without falling over");
let wireError = "";
try {
  const { loadGui } = require("./check_harness");
  loadGui().gui.wire();
} catch (err) {
  wireError = String(err);
  console.log("  " + wireError);
}
checker.check("no error while wiring    ", wireError, "");

checker.finish();
