# NVENCForgeGUI

A desktop window for [NVENCForge](https://github.com/burnersen/NVENCForge).

The window never re-implements the converter. It starts the unchanged
`NVENCForge.exe` as a separate process, passes the chosen options on the
command line and reads its machine-readable event channel (`-json`). Whatever
the converter decides — quality, bitrate caps, GPU capability probing — stays
exactly as it is on the command line.

## The download chain

You download the window. The window downloads `NVENCForge.exe` from its latest
GitHub release into `tools\`. On its first run `NVENCForge.exe` downloads
FFmpeg by itself. Nothing else has to be installed.

If a `tools\NVENCForge.exe` is already there it is used as it is — that is how
a locally built converter can be tested before it is released.

## Layout

| Path | What it is |
|---|---|
| `main.go` | window, size, drag and drop |
| `app.go` | every method the window may call |
| `runner.go` | starts the converter, reads its two channels |
| `wincon.go` | hidden console and the clean stop |
| `converter.go` | finding, checking and downloading `NVENCForge.exe` |
| `queue.go` | dropped files and folders become a queue |
| `runargs.go` | the options become a command line |
| `frontend/dist/index.html` | the whole window: HTML, CSS and JavaScript, no build step |

## Building

```
wails build
```

Put `NVENCForge.exe` (and, if you want to skip the first download, `ffmpeg.exe`
and `ffprobe.exe`) into a `tools\` folder next to the built exe.

## Tests

```
go test .
```

The two live checks in `runner_live_test.go` run a real conversion and are
skipped unless they are asked for:

```
$env:NVENCFORGEGUI_LIVE = "1"
$env:NVENCFORGEGUI_LIVE_SHORT = "C:\path\to\a\short.mkv"
$env:NVENCFORGEGUI_LIVE_LONG  = "C:\path\to\a\long.mkv"
go test -run TestLive -v .
```

## Status

Stage 3 of 8: the Convert area works — queue, options, real progress, clean
stop. Settings, the tool modes with their track chooser, parallel runs and
folder watching are not built yet and are shown as disabled in the window.
