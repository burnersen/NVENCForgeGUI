// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// savings_check.js — the savings bar along the bottom edge.
//
// It is the one figure in this window that is meant to be looked at rather
// than worked with, so a wrong one would go unnoticed for weeks. Two things
// can go wrong quietly: showing "0.0 MB" where nothing has been converted yet
// (which reads like a fault), and a reset button that wipes months of counting
// on a single click.
//
// The counting itself belongs to the Go side and is checked in savings_test.go.
// What is checked here is only what the window makes of the figures it is
// handed — above all the time, which is new and which nobody would notice
// being wrong until it had been wrong for weeks.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls, setSavingsReply } = loadGui();
const checker = createChecker();

console.log("\nThe bar is part of the window, not of a page");
// Inside #shell but outside #body: it has to stay in view whichever page is
// open, and it must not scroll away with the content.
const shell = html.slice(html.indexOf('<div id="shell">'), html.indexOf('<div id="tip">'));
checker.check("the bar exists", shell.includes('<footer id="savings">'), true);
// It stands after the whole page area, so no page can hide it and it does not
// scroll away with the content.
checker.check("it stands outside the pages", shell.indexOf("</main>") < shell.indexOf('<footer id="savings">'), true);
checker.check("both figures have a place", shell.includes('id="savings-total"') && shell.includes('id="savings-time"'), true);
// Week and month were dropped at the user's request: two figures that run from
// the first ever use need no boundaries and cannot drift apart.
checker.check("and no week or month is left", shell.includes('id="savings-week"') || shell.includes('id="savings-month"'), false);
// Resetting lives on the settings page, at the user's own request.
const settingsPage = html.slice(html.indexOf('<div id="page-settings"'), html.indexOf("</main>"));
checker.check("the reset button is in the settings", settingsPage.includes('id="btn-savings-reset"'), true);
checker.check("and not in the bar itself", shell.slice(shell.indexOf('<footer id="savings">')).includes("btn-savings-reset"), false);

console.log("\nNothing converted yet is said in words, not as a zero");
gui.showSavings({ totalMB: 0, totalFiles: 0, totalSeconds: 0 });
checker.check("the space saved", element("savings-total").textContent, "nothing yet");
checker.check("no file count is claimed", element("savings-files").textContent, "");
checker.check("and no time either     ", element("savings-time").textContent, "—");

console.log("\nReal figures are shown in the units people read");
gui.showSavings({ totalMB: 4310.4, totalFiles: 12, totalSeconds: 7830 });
checker.check("gigabytes above 1024 MB", element("savings-total").textContent, "4.21 GB");
checker.check("with the file count    ", element("savings-files").textContent, "  (12 files)");
checker.check("hours and minutes      ", element("savings-time").textContent, "2 h 10 min");

gui.showSavings({ totalMB: 250.5, totalFiles: 1, totalSeconds: 600 });
checker.check("megabytes below 1024 MB", element("savings-total").textContent, "250.5 MB");
checker.check("and one file is one    ", element("savings-files").textContent, "  (1 file)");
checker.check("under an hour: minutes ", element("savings-time").textContent, "10 min");

console.log("\nThe time reads sensibly at both ends");
// The very first short file must not read as "0 min" — that looks like the
// clock is not running at all.
gui.showSavings({ totalMB: 5, totalFiles: 1, totalSeconds: 42 });
checker.check("well under a minute", element("savings-time").textContent, "42 s");
// And it has to keep working once it grows past a day, which it will.
gui.showSavings({ totalMB: 500000, totalFiles: 900, totalSeconds: 183600 });
checker.check("past a full day    ", element("savings-time").textContent, "51 h 0 min");

console.log("\nBefore the Go side has answered, the bar says nothing");
// It must not invent a zero either: not asked yet is not the same as nothing
// saved. This is the state the window opens in.
gui.state.savings = null;
gui.showSavings(null);
checker.check("a dash, not a number", element("savings-total").textContent, "—");
checker.check("and no time either  ", element("savings-time").textContent, "—");

console.log("\nThe counter is only wiped on the second click");
setSavingsReply({ totalMB: 900, totalFiles: 3, totalSeconds: 1200 });
calls.savingsResets.length = 0;
gui.state.savingsResetAsked = false;

gui.resetSavings();
checker.check("the first click asks     ", calls.savingsResets.length, 0);
checker.contains("and says why            ", element("savings-reset-note").textContent, "cannot be undone");
checker.contains("the button changes      ", element("btn-savings-reset").textContent, "Click again");

const done = gui.resetSavings();
checker.check("the second click does it ", calls.savingsResets.length, 1);

Promise.resolve(done).then(() => {
  checker.check("the bar goes back to zero", element("savings-total").textContent, "nothing yet");
  checker.check("the time goes with it    ", element("savings-time").textContent, "—");
  checker.contains("the button reads normally", element("btn-savings-reset").textContent, "Reset");
  checker.finish();
});
