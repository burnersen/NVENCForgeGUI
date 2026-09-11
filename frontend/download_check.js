// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// download_check.js — checks what the window does after installing a converter.
//
// Run it with:  node frontend\download_check.js
//
// The point of this file is one promise that is easy to lose in a refactor:
// after a new NVENCForge has been installed, the window must READ THE INI
// AGAIN. The Go side primes the fresh exe so it writes its new settings into
// the file — but the settings page is built from that file, so without a
// re-read the new entries exist on disk and stay invisible on screen. That was
// the actual bug report: "I install the update and cannot use the new stuff."
//
// The other two cases matter because they are the ones where a re-read would
// be wrong or pointless: nothing was replaced (the second click is still
// pending), and the call failed altogether.

const path = require("path");
const { loadGui, createChecker } = require(path.join(__dirname, "check_harness.js"));

const { gui, calls, element, setDownloadReply } = loadGui();
const { check, finish } = createChecker();

const settle = () => new Promise((resolve) => setImmediate(resolve));

async function run() {
  gui.wire();

  console.log("\n=== installing a new converter re-reads the INI ===");
  const before = calls.configViews.length;
  await gui.download();
  await settle();
  check("the download was sent     ", calls.converterDownloads.length, 1);
  check("and the INI was read again", calls.configViews.length > before, true);

  console.log("\n=== nothing replaced: no re-read, and the button offers the retry ===");
  // This is the "release has no event channel" case: the Go side downloaded
  // but deliberately kept the installed converter. The file on disk did not
  // change, so re-reading it would only be noise.
  setDownloadReply({ replaced: false, tag: "v1.9.0", message: "Release v1.9.0 has no event channel." });
  const beforeSecond = calls.configViews.length;
  await gui.download();
  await settle();
  check("the download was sent     ", calls.converterDownloads.length, 2);
  check("no pointless re-read      ", calls.configViews.length, beforeSecond);
  check("the retry is offered      ", element("btn-download").textContent.includes("anyway"), true);

  console.log("\n=== a failed download leaves the button usable ===");
  setDownloadReply(new Error("GitHub is not reachable"));
  const beforeThird = calls.configViews.length;
  await gui.download();
  await settle();
  check("no re-read after a failure", calls.configViews.length, beforeThird);
  check("the button works again    ", element("btn-download").disabled, false);

  finish();
}

run();
