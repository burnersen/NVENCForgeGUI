# NVENCForgeGUI

[![CI](https://github.com/burnersen/NVENCForgeGUI/actions/workflows/ci.yml/badge.svg)](https://github.com/burnersen/NVENCForgeGUI/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-PolyForm%20Noncommercial-blue)](LICENSE.md)

**A window for [NVENCForge](https://github.com/burnersen/NVENCForge) — for everyone who would rather click than type.**

Drop your videos in, press Start, watch it happen. Same converter, same
results, same settings file — just visible.

![NVENCForgeGUI](docs/screenshot.png)

---

## What you get

- **Drag and drop** — files or whole folders, subfolders included.
- **Live progress** — one line per running converter: file, bar, speed, ETA, bitrate, output size so far.
- **Up to three videos at once.** Measured here with four 90-second clips: two at a time took **52 seconds instead of 72**. A third brought nothing more — the graphics card is busy by then. You pick 1, 2 or 3.
- **Watch a folder.** Point it at your downloads folder and forget about it: every video that lands there is converted on its own. A file is only picked up once it has stopped growing for 30 seconds, so a download in progress is left alone.
- **The lossless tools, with buttons:** take a video apart into its tracks, or put them back together. Both directions ask the same question: bit-for-bit, or Resolve-ready with AAC audio and cleaned subtitles.
- **The track chooser** — when a file has several audio tracks or subtitles, you tick what you want instead of typing numbers.
- **Every setting explained.** The Settings page is built from `NVENCForge_Config.ini` itself: hover over anything and it tells you what it does, what is allowed, and what your file currently says. The subtitle cleaner's phrase list is editable there too, instead of in a text editor.
- **Stop one, or stop all.** Each converter has its own ✕; what is already encoded is kept as a playable preview.
- **Day or night.** A switch at the bottom left turns the window light or dark, and it remembers which you chose. The log keeps its dark ground either way — the colours in there come from the converter itself and are meant for one.
- **It stays where you put it.** Size and place are remembered between starts, and starting it a second time brings the open window forward instead of opening a rival that fights over the same files.

## Getting started

1. Download `NVENCForgeGUI.exe` from the [releases](../../releases) and put it wherever you like.
2. Start it. If `NVENCForge.exe` is missing, one click fetches the latest one from its own repository — and it then sets itself up (configuration file and FFmpeg) without you doing anything else.
3. Drop a video in and press Start.

Nothing is installed, nothing is written to the registry. Delete the folder and
it is gone.

**You need:** Windows 10 or 11 and an NVIDIA graphics card (GTX 10 series or
newer). Without one, NVENCForge can still convert on the processor — slower,
but it works.

## The window never converts anything itself

It starts the unchanged `NVENCForge.exe` as a separate process, hands it the
options on the command line and reads its machine-readable event channel
(`-json`). Quality decisions, bitrate caps, the GPU capability probe — all of
that stays exactly where it was. If a converter is already sitting in `tools\`,
it is used as it is.

That is the design rule throughout: **no button may lie.** What is not built is
greyed out rather than pretending to work, and nothing on screen claims
anything the converter did not report.

<details>
<summary><b>How several converters share the work</b></summary>

Each file gets its own process, and up to three run side by side.

The obvious alternative — handing the same file list to every instance and
letting the converter's own `.lock` files sort it out — was measured and
dropped. It is a race: when one instance moves a finished original into
`originals` while another is still probing that same file, the second one
reports *"No such file or directory"* for a file that is in fact finished. One
process per file is just as fast (53 seconds against 52) and reports honestly.

The tool modes (Split, Join, DaVinci) always run one at a time. They copy
instead of encoding — the disk is the limit there, not the card — and they ask
about tracks, where two dialogs at once would be a nuisance.

</details>

<details>
<summary><b>Building it yourself</b></summary>

Needs [Go](https://go.dev/) and [Wails v2](https://wails.io/).

```
wails build
```

**Without `-clean`** — that would delete `build\bin\tools\` and with it the
converter sitting next to the window.

The interface is a single HTML file with no build step: no npm, no bundler, no
node_modules.

</details>

<details>
<summary><b>Tests</b></summary>

```
go test ./...
node frontend\progress_check.js
node frontend\options_check.js
node frontend\settings_check.js
node frontend\davinci_check.js
node frontend\extract_check.js
node frontend\join_check.js
node frontend\watch_check.js
node frontend\parallel_check.js
node frontend\srt_check.js
node frontend\theme_check.js
```

The `*_check.js` files run the window's own script without a browser and check
what the display really does: that a bar never reports more than the converter
said, that the start button carries the mode of the page you are on, that an
answer goes back to the converter that asked. Every check is written so that
breaking the code on purpose makes it fail — a check that stays green when the
code is wrong is worth nothing.

The live tests (`NVENCFORGEGUI_LIVE=1`) go further and start the real converter
on a real file.

Everything above except the live tests runs on GitHub for every push, on a
Windows machine — see the CI badge at the top. It cannot convert anything up
there, having neither a graphics card nor FFmpeg, but it does prove the
window still builds and still behaves.

</details>

## License

NVENCForgeGUI is source-available under the [PolyForm Noncommercial License 1.0.0](LICENSE.md).
Free to use, study, modify and share for any **noncommercial** purpose: personal use, hobby, education, research. **Commercial use, resale or bundling into paid products is not permitted** without a separate license from the author. Want a commercial license? Open an issue or reach out.

### What it is built on

The window is built with [Wails v2](https://wails.io/) (MIT-licensed), which pairs a Go program with the WebView2 runtime Windows already ships. Wails stays under its own license; the license above covers the code in this repository.

The converter is a separate program: [NVENCForge](https://github.com/burnersen/NVENCForge), same author, same PolyForm Noncommercial license. The window looks for `NVENCForge.exe` in its `tools` folder and downloads it from GitHub if it is missing — it is not part of this repository. NVENCForge in turn fetches FFmpeg (GPL) on its own first run. Each step only handles what it knows about.

---

## Feedback

Something unclear, something missing, something behaving oddly? Open an issue —
a short one is fine. Nobody has ever complained too much.

---

Free for personal use. Built for my own media library, shared because it might
save you the evening it saved me.
