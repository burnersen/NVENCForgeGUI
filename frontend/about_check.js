// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// about_check.js — checks the About page.
//
// Run it with:  node frontend\about_check.js
//
// The one thing here that can really go wrong is a link. A plain <a href> in a
// WebView2 window does not open a browser: it loads the page INTO this window,
// and the program is gone with no way back. So the page must carry no href at
// all, and every link button must go through BrowserOpenURL.
//
// The second thing is the version. It has to come from the same report the
// header uses — two places asking separately would sooner or later name two
// different versions.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls } = loadGui();
const { check, contains, finish } = createChecker();

const aboutPage = () => {
  const from = html.indexOf('<div id="page-about" hidden>');
  const to = html.indexOf('</main>', from);
  if (from < 0 || to < 0) throw new Error("the About page is not in the markup");
  return html.slice(from, to);
};

console.log("\n=== the page is there and reachable ===");
check("the page exists           ", html.includes('<div id="page-about" hidden>'), true);
check("a nav button leads to it  ", /nav-item[^>]*data-page="about"/.test(html), true);
gui.showPage("about");
check("showPage opens it          ", element("page-about").hidden, false);
check("and hides the Convert page", element("page-convert").hidden, true);

console.log("\n=== no link may swallow the window ===");
check("no href on the page       ", /<a\s[^>]*href/i.test(aboutPage()), false);

console.log("\n=== the buttons hand the address to the browser ===");
gui.wire();
element("btn-about-gui-repo").onclick();
element("btn-about-converter-repo").onclick();
element("btn-about-licence").onclick();
check("three addresses opened    ", calls.opened.length, 3);
contains("the window's own repo     ", calls.opened[0], "github.com/burnersen/NVENCForgeGUI");
contains("the converter's repo      ", calls.opened[1], "github.com/burnersen/NVENCForge");
contains("and the licence           ", calls.opened[2], "polyformproject.org");

console.log("\n=== the versions come from the startup report ===");
gui.showAbout({
  guiVersion: "9.9.9",
  converter: { found: true, version: "v1.18.0", eventChannel: true, path: "X:\\tools\\NVENCForge.exe" }
});
contains("this window               ", element("about-gui").textContent, "9.9.9");
contains("the converter             ", element("about-converter").textContent, "v1.18.0");
check("and where it sits         ", element("about-converter-path").textContent, "X:\\tools\\NVENCForge.exe");

console.log("\n=== with no converter it says so ===");
gui.showAbout({ guiVersion: "9.9.9", converter: { found: false } });
contains("not found                 ", element("about-converter").textContent, "not found");
check("and no stale path is left ", element("about-converter-path").textContent, "—");

console.log("\n=== what the page promises about the disk ===");
// These three names are the promise "delete the folder and it is gone". If a
// fourth file is ever written next to the exe, this check has to grow with it.
["NVENCForgeGUI.window", "NVENCForgeGUI.savings", "NVENCForgeGUI.profiles"].forEach((name) => {
  check("names " + name, aboutPage().includes(name), true);
});

finish();
