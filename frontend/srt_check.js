// srt_check.js — checks the subtitle cleaner's phrase list on the settings page.
//
// Run it with:  node frontend\srt_check.js
//
// What matters here is that the list on screen and the list in the file stay
// the same thing. A phrase that changes on its way out would strip something
// other than what the user typed, and that only shows up much later in a
// finished subtitle file - which is exactly why it is checked here.
const { loadGui, createChecker } = require("./check_harness");

const { gui, calls, setSRTReply, element } = loadGui();
const { check, contains, finish } = createChecker();

const fileView = {
  found: true,
  path: "X:\\tools\\SRTCleaner_config.txt",
  note: "",
  phrases: [
    { text: "untertitel", exact: false },
    { text: "2017", exact: true }
  ]
};

async function main() {
  console.log("\n=== the list is read as it is in the file ===");
  setSRTReply(fileView);
  await gui.loadSRTCleaner();
  check("two phrases                  ", gui.state.srt.phrases.length, 2);
  check("plain one                    ", gui.state.srt.phrases[0].text, "untertitel");
  check("  is not exact               ", gui.state.srt.phrases[0].exact, false);
  check("the exact one                ", gui.state.srt.phrases[1].text, "2017");
  check("  keeps its flag             ", gui.state.srt.phrases[1].exact, true);

  console.log("\n=== nothing typed yet, nothing to save ===");
  check("Save is off                  ", element("btn-srt-save").disabled, true);
  check("the path is shown instead    ", element("srt-state").textContent, fileView.path);

  console.log("\n=== adding a phrase ===");
  element("srt-new").value = "  werbung  ";
  element("srt-new-exact").checked = false;
  gui.addSRTPhrase();
  check("three phrases now            ", gui.state.srt.phrases.length, 3);
  check("spaces are trimmed           ", gui.state.srt.phrases[2].text, "werbung");
  check("the box is empty again       ", element("srt-new").value, "");
  check("Save is on                   ", element("btn-srt-save").disabled, false);
  check("and says so                  ", element("srt-state").textContent, "unsaved changes");

  console.log("\n=== a ticked box travels with the new phrase ===");
  element("srt-new").value = "ende der vorstellung";
  element("srt-new-exact").checked = true;
  gui.addSRTPhrase();
  check("four phrases now             ", gui.state.srt.phrases.length, 4);
  check("the new one is exact         ", gui.state.srt.phrases[3].exact, true);
  check("and the tick is cleared      ", element("srt-new-exact").checked, false);

  console.log("\n=== an empty box adds nothing ===");
  const before = gui.state.srt.phrases.length;
  element("srt-new").value = "   ";
  gui.addSRTPhrase();
  check("still the same count         ", gui.state.srt.phrases.length, before);

  console.log("\n=== a phrase must not start with # ===");
  element("srt-new").value = "# not a phrase";
  gui.addSRTPhrase();
  check("it was refused               ", gui.state.srt.phrases.length, before);
  contains("and the reason is shown  ", element("srt-state").textContent, "comment");

  console.log("\n=== what is sent is what is on screen ===");
  await gui.saveSRTPhrases();
  check("one save call                ", calls.srtSaves.length, 1);
  const sent = calls.srtSaves[0];
  check("  four phrases sent          ", sent.length, 4);
  check("  the exact flag rides along ", sent[1].exact, true);
  check("  the added plain one        ", sent[2].text, "werbung");
  check("  the added exact one        ", sent[3].exact, true);

  console.log("\n=== a missing file says so instead of showing an empty list ===");
  setSRTReply({ found: false, path: "", note: "SRTCleaner_config.txt is not there yet.", phrases: [] });
  await gui.loadSRTCleaner();
  check("no phrases                   ", gui.state.srt.phrases.length, 0);
  check("Save stays off               ", element("btn-srt-save").disabled, true);
  check("Add stays off                ", element("btn-srt-add").disabled, true);
  check("the box is locked            ", element("srt-new").disabled, true);

  console.log("\n=== the signature tells a real change from a round trip ===");
  setSRTReply(fileView);
  await gui.loadSRTCleaner();
  check("untouched -> Save off        ", element("btn-srt-save").disabled, true);
  gui.state.srt.phrases[0].exact = true;
  gui.renderSRTCleaner();
  check("a ticked box is a change     ", element("btn-srt-save").disabled, false);
  gui.state.srt.phrases[0].exact = false;
  gui.renderSRTCleaner();
  check("and back again is not        ", element("btn-srt-save").disabled, true);

  finish();
}

main();
