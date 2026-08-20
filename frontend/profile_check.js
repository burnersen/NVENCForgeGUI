// NVENCForgeGUI — Required Notice: Copyright (c) 2026 burnersen — NVENCForgeGUI
// Licensed under the PolyForm Noncommercial License 1.0.0 (non-commercial use only).
// Full terms: LICENSE.md · https://polyformproject.org/licenses/noncommercial/1.0.0

// profile_check.js — checks the saved option sets without saving anything.
//
// Run it with:  node frontend\profile_check.js
//
// What makes this worth checking: a profile writes into the very controls that
// decide how a video is encoded. If one field is forgotten while loading, the
// run quietly uses whatever stood there before — and the file is already
// converted by the time anybody notices.
//
// The other half is what leaves the window: "shut down the PC when finished"
// must never travel inside a profile.
const { loadGui, createChecker } = require("./check_harness");

const { gui, element, calls, setProfiles } = loadGui();
const { check, contains, finish } = createChecker();

const value = (id) => element(id).value;
const lastSave = () => calls.profileSaves[calls.profileSaves.length - 1] || {};

// Two sets, deliberately not in alphabetical order: the list has to come back
// sorted, or a growing list turns into a lucky dip.
const sets = [
  {
    name: "Serien", codec: "av1", encoder: "", container: "mp4", resolution: "original",
    audio: "copy", bitDepth: "8", quality: "fixed", fixedCQ: 34, maxBitrate: 6000,
    keepSource: true, parallel: 3
  },
  {
    name: "Filme", codec: "", encoder: "cpu", container: "", resolution: "",
    audio: "", bitDepth: "", quality: "auto", fixedCQ: 0, maxBitrate: 0,
    keepSource: false, parallel: 1
  }
];

console.log("\n=== nothing saved yet ===");
setProfiles([]);
let appendedBefore = element("opt-profile").appended;
(async () => {
  await gui.loadProfiles();
  check("only the empty entry      ", element("opt-profile").appended - appendedBefore, 1);
  check("nothing to delete         ", element("btn-profile-delete").disabled, true);

  console.log("\n=== two saved sets ===");
  setProfiles(sets);
  appendedBefore = element("opt-profile").appended;
  await gui.loadProfiles();
  check("empty entry plus two sets ", element("opt-profile").appended - appendedBefore, 3);
  check("sorted by name            ", gui.state.profiles.map((p) => p.name).join(", "), "Filme, Serien");

  console.log("\n=== loading one on the Convert page ===");
  element("opt-profile").value = "Serien";
  gui.chooseProfile("opt");
  check("codec                     ", value("opt-codec"), "av1");
  check("encoder                   ", value("opt-encoder"), "");
  check("container                 ", value("opt-container"), "mp4");
  check("resolution                ", value("opt-resolution"), "original");
  check("audio                     ", value("opt-audio"), "copy");
  check("bit depth                 ", value("opt-bitdepth"), "8");
  check("quality                   ", value("opt-quality"), "fixed");
  check("fixed CQ                  ", value("opt-cq"), 34);
  check("max bitrate               ", value("opt-bitrate"), 6000);
  check("keep the source           ", element("opt-keep").checked, true);
  check("how many at a time        ", value("opt-parallel"), "3");
  // The CQ box only means anything with a fixed quality — a loaded profile has
  // to open and close it just like a hand-made choice does.
  check("the CQ box is shown       ", element("field-cq").hidden, false);
  check("the name is ready to save ", value("opt-profile-name"), "Serien");

  console.log("\n=== the watched folder loads the same set ===");
  element("wopt-profile").value = "Filme";
  gui.chooseProfile("wopt");
  check("its own codec             ", value("wopt-codec"), "");
  check("its own encoder           ", value("wopt-encoder"), "cpu");
  check("its own CQ box is hidden  ", element("wfield-cq").hidden, true);
  // The two pages are separate on purpose: the watched folder must not pull
  // the batch you are setting up out from under you.
  check("Convert page untouched    ", value("opt-codec"), "av1");

  console.log("\n=== saving what stands on the page ===");
  element("opt-shutdown").checked = true;   // must NOT travel with the profile
  element("opt-profile-name").value = "Nachts";
  await gui.saveProfile();
  check("sent under the typed name ", lastSave().name, "Nachts");
  check("with the codec on show    ", lastSave().codec, "av1");
  check("and the fixed CQ          ", lastSave().fixedCQ, 34);
  check("shutdown stays out of it  ", "shutdown" in lastSave(), false);
  check("files stay out of it      ", "files" in lastSave(), false);

  console.log("\n=== saving without a name ===");
  const savesSoFar = calls.profileSaves.length;
  element("opt-profile-name").value = "   ";
  element("opt-profile").value = "";
  await gui.saveProfile();
  check("nothing was sent          ", calls.profileSaves.length, savesSoFar);
  contains("and it says why           ", element("profile-note").textContent, "name");

  console.log("\n=== deleting takes two clicks ===");
  element("opt-profile").value = "Serien";
  await gui.deleteProfile();
  check("first click deletes nothing", calls.profileDeletes.length, 0);
  contains("but warns                 ", element("profile-note").textContent, "gone for good");
  await gui.deleteProfile();
  check("second click deletes      ", calls.profileDeletes.join(), "Serien");
  check("and the choice is cleared ", value("opt-profile"), "");
  check("the button reads Delete   ", element("btn-profile-delete").textContent, "Delete");

  finish();
})();
