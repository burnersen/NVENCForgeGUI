// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// update_check.js — checks the update panel on the About page.
//
// Run it with:  node frontend\update_check.js
//
// Three things here would be bad in different ways:
//
//   1. An install button that is offered when there is nothing newer. It would
//      replace the running program for no reason at all.
//   2. A window that asks GitHub on its own. The user asked for the opposite:
//      it looks only when the button is pressed.
//   3. A button that stays dead after a failed attempt. The next try would be
//      impossible without restarting the program.
const { loadGui, createChecker } = require("./check_harness");

const { gui, html, element, calls, setUpdateCheckReply, setUpdateInstallReply } = loadGui();
const { check, contains, finish } = createChecker();

const settle = () => new Promise((resolve) => setImmediate(resolve));

async function run() {
  gui.wire();

  console.log("\n=== the panel is on the About page ===");
  check("check button exists       ", html.includes('id="btn-update-check"'), true);
  check("install button exists     ", html.includes('id="btn-update-install"'), true);
  check("install button starts hidden", /id="btn-update-install"[^>]*hidden/.test(html), true);

  console.log("\n=== nothing is asked before the button is pressed ===");
  // wire() runs at startup. If a single check had gone out by now, the promise
  // "it asks GitHub only when you press the button" would already be broken.
  check("no check on startup       ", calls.updateChecks.length, 0);
  check("no install on startup     ", calls.updateInstalls.length, 0);

  console.log("\n=== nothing newer: no install button ===");
  setUpdateCheckReply({
    newer: false, current: "1.1.0", latest: "v1.1.0",
    note: "This is the newest release (v1.1.0)."
  });
  await gui.checkUpdate();
  await settle();
  check("GitHub was asked once     ", calls.updateChecks.length, 1);
  contains("and the answer is shown   ", element("update-note").textContent, "newest release");
  check("install stays hidden      ", element("btn-update-install").hidden, true);

  console.log("\n=== something newer: the button appears and names the version ===");
  setUpdateCheckReply({
    newer: true, current: "1.1.0", latest: "v1.2.0", sizeBytes: 11789824,
    note: "NVENCForgeGUI v1.2.0 is available."
  });
  await gui.checkUpdate();
  await settle();
  check("install is offered        ", element("btn-update-install").hidden, false);
  contains("and names the version     ", element("btn-update-install").textContent, "v1.2.0");
  check("still nothing installed   ", calls.updateInstalls.length, 0);

  console.log("\n=== installing tells the Go side and says the window is going ===");
  setUpdateInstallReply({
    installed: true, restarting: true, version: "v1.2.0",
    message: "NVENCForgeGUI v1.2.0 installed. Restarting now."
  });
  await gui.installUpdate();
  await settle();
  check("install was sent          ", calls.updateInstalls.length, 1);
  contains("the message is shown      ", element("update-note").textContent, "installed");
  contains("the button says it is going", element("btn-update-install").textContent, "restarting");

  console.log("\n=== a refused update leaves both buttons usable ===");
  // This is the case where a conversion is still running: the Go side refuses,
  // and the user has to be able to try again once it has finished.
  setUpdateCheckReply({ newer: true, current: "1.1.0", latest: "v1.2.0", note: "v1.2.0 is available." });
  await gui.checkUpdate();
  await settle();
  setUpdateInstallReply(new Error("a conversion is still running — let it finish or stop it first"));
  await gui.installUpdate();
  await settle();
  contains("the reason is shown       ", element("update-note").textContent, "still running");
  check("install can be tried again", element("btn-update-install").disabled, false);
  check("checking can be tried again", element("btn-update-check").disabled, false);
  contains("the label is back         ", element("btn-update-install").textContent, "v1.2.0");

  console.log("\n=== GitHub not reachable: the check button comes back ===");
  setUpdateCheckReply(new Error("no network"));
  await gui.checkUpdate();
  await settle();
  contains("the failure is shown      ", element("update-note").textContent, "Could not ask GitHub");
  check("and the button works again", element("btn-update-check").disabled, false);
  check("no install button is left  ", element("btn-update-install").hidden, true);

  finish();
}

run();
