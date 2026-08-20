// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// shutdown_check.js — "shut down when finished", without a browser.
//
// This is the option with the worst failure: a machine that switches itself
// off in the middle of a batch takes the rest of the night's work with it, and
// nobody is sitting there to see it happen. Until v1.0.1 that is exactly what
// it did — the option travelled to the converter, and since this window hands
// the converter one file at a time, the FIRST finished file pulled the plug.
//
// The decision now belongs to the Go side (shutdown.go), so what is left to
// check here is the window's half of the deal: that the box is passed on the
// moment it is clicked, that the window takes Go's word for the state, that
// the warning really comes up while Windows counts down, and that the cancel
// button asks for the cancel.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls } = loadGui();
const checker = createChecker();

const state = (armed, counting) => ({ armed, counting, seconds: 60, note: "" });

function reset() {
  calls.shutdownWishes.length = 0;
  calls.shutdownCancels.length = 0;
  gui.state.watch = { folder: "", active: false };
  gui.state.converterFound = true;
  gui.onShutdownState(state(false, false));
}

console.log("\nThe box and the warning are on the page");
checker.contains("the box itself", html, 'id="opt-shutdown"');
checker.contains("a warning box", html, 'id="shutdown-alert"');
checker.contains("with the seconds", html, 'id="shutdown-seconds"');
checker.contains("and a way out", html, 'id="btn-shutdown-cancel"');

console.log("\nClicking the box tells the Go side right away");
reset();
element("opt-shutdown").checked = true;
gui.setShutdownWish();
checker.check("the wish went out    ", calls.shutdownWishes.length, 1);
checker.check("and it says 'on'     ", calls.shutdownWishes[0], true);
element("opt-shutdown").checked = false;
gui.setShutdownWish();
checker.check("taking it back too   ", calls.shutdownWishes[1], false);

// The whole point of sending it at once: the batch is usually already running
// when somebody decides to go to bed. A wish that only travelled with the next
// StartRun would do nothing at all for the batch on screen.
console.log("\nIt does not wait for the next run");
reset();
gui.areaOf("convert").running = true;
element("opt-shutdown").checked = true;
gui.setShutdownWish();
checker.check("sent while running   ", calls.shutdownWishes.length, 1);
checker.check("no run was started   ", calls.runs.length, 0);
gui.areaOf("convert").running = false;

console.log("\nThe window takes the Go side's word for the state");
reset();
element("opt-shutdown").checked = true;
// Go dropped the wish - a stopped batch, a watched folder, another converter.
// A tick left standing here would promise a shutdown that will not come.
gui.onShutdownState(state(false, false));
checker.check("the box follows      ", element("opt-shutdown").checked, false);

console.log("\nWhile Windows counts down, the warning is up");
reset();
gui.onShutdownState(state(true, true));
checker.check("the warning shows    ", element("shutdown-alert").hidden, false);
checker.contains("with the seconds on it", element("shutdown-seconds").textContent, "60");
// And the box is locked: only the button can call it off now, so a box that
// still looked clickable would promise something it cannot do.
checker.check("the box is locked    ", element("opt-shutdown").disabled, true);

console.log("\nThe cancel button asks Windows to drop it");
gui.cancelShutdown();
checker.check("one cancel went out  ", calls.shutdownCancels.length, 1);

console.log("\nAnd afterwards the warning is gone again");
gui.onShutdownState(state(false, false));
checker.check("the warning is hidden", element("shutdown-alert").hidden, true);
checker.check("the box is free again", element("opt-shutdown").disabled, false);
checker.check("and unticked         ", element("opt-shutdown").checked, false);

// A watched folder never finishes, so there is nothing to shut down after.
console.log("\nA watched folder keeps the box out of reach");
reset();
gui.state.watch = { folder: "D:\\Downloads", active: true };
gui.updateButtons();
checker.check("locked               ", element("opt-shutdown").disabled, true);
checker.check("and cleared          ", element("opt-shutdown").checked, false);

// The run request still carries the wish, so a batch started with the box
// already ticked arms the Go side even if nobody clicked it this session.
console.log("\nStarting a batch carries the wish along");
reset();
element("opt-shutdown").checked = true;
const request = gui.collectRequest(gui.areaOf("convert"));
checker.check("the request says so  ", request.shutdown, true);

checker.finish();
