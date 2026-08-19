// wiring_check.js — does the window's script talk to elements that exist?
//
// This check exists because of a real failure, and because none of the other
// nine could have caught it. The stand-in document in check_harness.js hands
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

// Only literal ids can be checked. Something like $("field-" + key) is built at
// runtime and is left alone rather than guessed at.
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

// The four ids that keep the window usable at all. Named one by one, so that
// removing one of them fails here with its own name rather than as a number.
console.log("\nThe pieces the window cannot work without");
for (const id of ["btn-start", "btn-stop", "logbox", "queue", "watch-logbox",
                  "watch-lanes", "btn-watch-start", "btn-watch-stop", "lanes"]) {
  checker.check("  " + id.padEnd(18), defined.has(id), true);
}

// The handover that stops the web view from opening dropped files itself. It
// is not a button, so nothing else here would miss it - and when it is gone,
// dropping a video plays it instead of queueing it.
console.log("\nFile dropping is taken over from the web view");
checker.check("OnFileDrop is registered ", /runtime\.OnFileDrop\(/.test(script), true);
checker.check("and drops reach the queue", /OnFileDrop\(\s*\(/.test(script) || script.includes("OnFileDrop("), true);

// The watched folder has a twin of every display the Convert page has. Each
// twin needs the same styling, and CSS says nothing when it does not: the rule
// simply does not apply. That is how its log first appeared as a white,
// sizeless box with unreadable terminal colours in it.
console.log("\nEvery twin display is styled, not just the original");
const css = markup.slice(markup.indexOf("<style>"), markup.indexOf("</style>"));
const countOf = (name) => css.split("#" + name).length - 1;
for (const [one, twin] of [["logbox", "watch-logbox"], ["lanes", "watch-lanes"], ["summary", "watch-summary"]]) {
  // Counted, not merely looked for: naming the twin in one rule and forgetting
  // it in the next is the same bug in a smaller size, and a plain "is it
  // mentioned anywhere" would sail straight past it.
  const forOne = countOf(one);
  const forTwin = countOf(twin);
  if (forOne !== forTwin) {
    console.log("  #" + one + " is styled " + forOne + " time(s), #" + twin + " " + forTwin);
  }
  checker.check("  #" + twin.padEnd(16), forTwin, forOne);
}

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
