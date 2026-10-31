# Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io),
certificate by [SignPath Foundation](https://signpath.org).

> **Status:** NVENCForgeGUI has applied to the SignPath Foundation. Until
> the signing is active, `NVENCForgeGUI.exe` is not signed. Every release
> says in its text whether its exe is signed.

## What is signed

Only `NVENCForgeGUI.exe`, built from the source code in this repository by
the [release workflow](.github/workflows/release.yml) on GitHub-hosted
runners. Nothing that was built elsewhere is signed with this certificate.
Every signing request is approved by hand, release by release.

Not part of this project's releases: the converter `NVENCForge.exe`, which
the window fetches from the releases of
[NVENCForge](https://github.com/burnersen/NVENCForge) (a project of its own
with its own [code signing policy](https://github.com/burnersen/NVENCForge/blob/main/CODE_SIGNING.md)),
and FFmpeg, which the converter downloads on its first run.

The file properties of the signed exe name the product `NVENCForgeGUI` and
the version of the release; SignPath refuses to sign a file that does not
match.

## Team roles

| Role | Who |
| ---- | --- |
| Committers and reviewers | [burnersen](https://github.com/burnersen) |
| Approvers | [burnersen](https://github.com/burnersen) |

Changes from anybody else (pull requests) are reviewed by a committer before
they are merged. All team members use multi-factor authentication for GitHub
and for SignPath.

## Privacy policy

This program will not transfer any information to other networked systems
unless specifically requested by the user or the person installing or
operating it.

What NVENCForgeGUI does on the network, and only when you click for it:

- **Fetching the converter:** if `NVENCForge.exe` is missing, or when you
  update it, the window asks the GitHub API for the newest NVENCForge
  release and downloads `NVENCForge.exe` from GitHub. The converter then
  fetches FFmpeg itself (see its
  [privacy policy](https://github.com/burnersen/NVENCForge/blob/main/CODE_SIGNING.md#privacy-policy)).
- **Updating the window:** on the About page, and only when you press the
  button, it asks the GitHub API for the newest NVENCForgeGUI release and,
  on a second click, downloads it from GitHub.

Like any network request, these reveal your IP address to GitHub. Your
videos are converted on your own PC and never leave it. There is no
telemetry, no automatic update check, no account and no advertising.

## Installing and uninstalling

There is no installer. NVENCForgeGUI keeps everything (the converter,
FFmpeg, its settings, profiles and window position) in the `tools` folder
next to the exe and writes nothing to the registry. To uninstall, delete the
folder.

## Verifying a download

- **Signature:** right-click `NVENCForgeGUI.exe` > *Properties* > *Digital
  Signatures*. A signed release shows *SignPath Foundation*.
- **Build it yourself:** see [Building it yourself](README.md#building-it-yourself).

## Reporting a problem

If you think a signed file of this project does something it should not,
please open an [issue](https://github.com/burnersen/NVENCForgeGUI/issues).
Signed files that break these rules can also be reported to the
[SignPath Foundation](https://signpath.org).
