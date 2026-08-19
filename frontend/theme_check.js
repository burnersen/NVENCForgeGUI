// theme_check.js — checks the day/night switch.
//
// Run it with:  node frontend\theme_check.js
//
// Two things can go wrong here and neither shows up while you look at the
// window: a theme that is put on screen but never written down (gone on the
// next start), and a colour name that the day palette forgot to answer — the
// window then quietly keeps the night value for that one thing, which is how
// unreadable text happens on a white panel.
const { loadGui, createChecker } = require("./check_harness");

const { gui, calls, html, element } = loadGui();
const { check, finish } = createChecker();

// paletteOf pulls the colour names out of one CSS block, e.g. ":root {…}".
function paletteOf(selector) {
  const start = html.indexOf(selector + " {");
  if (start === -1) return null;
  const block = html.slice(start, html.indexOf("}", start));
  const names = new Set();
  for (const line of block.split("\n")) {
    const match = line.match(/^\s*(--[a-z0-9-]+)\s*:/i);
    if (match) names.add(match[1]);
  }
  return names;
}

console.log("\n=== a theme goes on screen and says so ===");
gui.applyTheme("light");
check("html wears the theme         ", element("html").dataset.theme, "light");
check("day button is pressed        ", element("theme-light").getAttribute("aria-pressed"), "true");
check("night button is not          ", element("theme-dark").getAttribute("aria-pressed"), "false");

gui.applyTheme("dark");
check("and back again               ", element("html").dataset.theme, "dark");
check("night button is pressed      ", element("theme-dark").getAttribute("aria-pressed"), "true");

console.log("\n=== nonsense never reaches the screen ===");
// A hand-edited file, a typo, a future theme that was dropped again: the
// window has to land on something it can actually draw.
gui.applyTheme("neon");
check("unknown theme falls back     ", element("html").dataset.theme, "dark");
gui.applyTheme(undefined);
check("nothing at all falls back    ", element("html").dataset.theme, "dark");

console.log("\n=== the choice is written down ===");
calls.themes.length = 0;
gui.applyTheme("dark");
gui.chooseTheme("light");
check("one save                     ", calls.themes.length, 1);
check("  and it is the right theme  ", calls.themes[0], "light");
check("screen changed too           ", element("html").dataset.theme, "light");

// Clicking the half that is already on must not write anything: the file
// would be rewritten for nothing, and on a slow disk that is a stutter for
// no gain.
gui.chooseTheme("light");
check("no save for the same theme   ", calls.themes.length, 1);

console.log("\n=== both palettes answer the same names ===");
const night = paletteOf(":root");
const day = paletteOf('html[data-theme="light"]');
check("night palette found          ", night !== null && night.size > 10, true);
check("day palette found            ", day !== null && day.size > 10, true);

// --radius is not a colour and has no business being repeated.
const missing = [...night].filter((name) => name !== "--radius" && !day.has(name));
const extra = [...day].filter((name) => !night.has(name));
if (missing.length) console.log("  missing in the day palette: " + missing.join(", "));
if (extra.length) console.log("  only in the day palette: " + extra.join(", "));
check("day palette answers all      ", missing.length, 0);
check("and invents none             ", extra.length, 0);

finish();
