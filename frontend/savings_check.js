// savings_check.js — the savings bar along the bottom edge.
//
// It is the one figure in this window that is meant to be looked at rather
// than worked with, so a wrong one would go unnoticed for weeks. Two things
// can go wrong quietly: showing "0.0 MB" where nothing has been converted yet
// (which reads like a fault), and a reset button that wipes months of counting
// on a single click.
//
// The counting itself belongs to the Go side and is checked in savings_test.go
// — including the week and month boundaries. What is checked here is only what
// the window makes of the figures it is handed.
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
checker.check("both figures have a place", shell.includes('id="savings-week"') && shell.includes('id="savings-month"'), true);
// Resetting lives on the settings page, at the user's own request.
const settingsPage = html.slice(html.indexOf('<div id="page-settings"'), html.indexOf("</main>"));
checker.check("the reset button is in the settings", settingsPage.includes('id="btn-savings-reset"'), true);
checker.check("and not in the bar itself", shell.slice(shell.indexOf('<footer id="savings">')).includes("btn-savings-reset"), false);

console.log("\nNothing converted yet is said in words, not as a zero");
gui.showSavings({ weekMB: 0, weekFiles: 0, monthMB: 0, monthFiles: 0 });
checker.check("this week ", element("savings-week").textContent, "nothing yet");
checker.check("this month", element("savings-month").textContent, "nothing yet");
checker.check("and no file count is claimed", element("savings-week-files").textContent, "");

console.log("\nReal figures are shown in the units people read");
gui.showSavings({ weekMB: 4310.4, weekFiles: 12, monthMB: 250.5, monthFiles: 1 });
checker.check("gigabytes above 1024 MB", element("savings-week").textContent, "4.21 GB");
checker.check("megabytes below it     ", element("savings-month").textContent, "250.5 MB");
checker.check("with the file count    ", element("savings-week-files").textContent, "  (12 files)");
checker.check("and one file is one    ", element("savings-month-files").textContent, "  (1 file)");

console.log("\nBefore the Go side has answered, the bar says nothing");
// It must not invent a zero either: not asked yet is not the same as nothing
// saved. This is the state the window opens in.
gui.state.savings = null;
gui.showSavings(null);
checker.check("a dash, not a number", element("savings-week").textContent, "—");

console.log("\nThe counter is only wiped on the second click");
setSavingsReply({ weekMB: 900, weekFiles: 3, monthMB: 900, monthFiles: 3 });
calls.savingsResets.length = 0;
gui.state.savingsResetAsked = false;

gui.resetSavings();
checker.check("the first click asks     ", calls.savingsResets.length, 0);
checker.contains("and says why            ", element("savings-reset-note").textContent, "cannot be undone");
checker.contains("the button changes      ", element("btn-savings-reset").textContent, "Click again");

const done = gui.resetSavings();
checker.check("the second click does it ", calls.savingsResets.length, 1);

Promise.resolve(done).then(() => {
  checker.check("the bar goes back to zero", element("savings-week").textContent, "nothing yet");
  checker.contains("the button reads normally", element("btn-savings-reset").textContent, "Reset");
  checker.finish();
});
