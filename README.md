# GoPaste

English | [简体中文](README.zh-CN.md)

A clipboard manager for macOS and Windows, built with Go and [MyGo](https://github.com/egoist/mygo).

[Screenshots](docs/screenshots.md)

## Features

- Keep a history of copied text, images, links, colors and files, with the source app and copy time
- Search your history, organize items in regular lists, or filter them with smart lists
- Preview and drag items, and paste with formatting or as plain text
- Choose which apps to exclude, and skip blank text, content marked confidential or images above a size limit
- Use a resizable bottom bar or a panel that opens beside the cursor
- Choose Classic or Liquid Glass, in English or Simplified Chinese

## MyGo

GoPaste uses [MyGo](https://github.com/egoist/mygo)'s native UI:

- GPU-rendered UI written in Go, with no WebView, HTML or JavaScript
- No cgo required; the same codebase cross-compiles to macOS and Windows
- Desktop APIs for the clipboard, tray, global shortcuts, drag and drop, and signed updates
- UI tests and screenshot rendering that run without opening a window

## Memory usage

Measured on October 23, 2026 with a local arm64 development build on macOS 27.0.1, Apple M1 Pro and 32 GB RAM.

The test used 2,000 records: 800 text, 500 image, 400 file, 200 link and 100 color records. Samples included 4K images, roughly 1 MiB of text in one record and 600 files in another.

| Scenario | Memory usage |
| --- | --- |
| History loaded | 46.2 MiB |
| Panel open | 68.1 MiB |
| Scrolled to record 2,000 | 183.8 MiB |
| Bottom bar resized repeatedly | 171.5 MiB |
| Panel hidden after testing | 104.8 MiB |
| Peak during each run | 202.1 MiB |

Values are medians of three runs, using the Physical footprint metric from `vmmap -summary` after three seconds idle at each stage, without forcing garbage collection.

The test covered storage and UI in a native window, without clipboard monitoring, the tray or the updater. Windows has not been measured yet.

## Install and use

Download from [Releases](https://github.com/viewhang/go_paste/releases):

- macOS: choose Apple Silicon (arm64) or Intel (x64), extract the archive and move `GoPaste.app` to Applications
- Windows: run the x64 Setup installer

Press `Command+Shift+V` on macOS or `Ctrl+Shift+V` on Windows to show or hide the panel. Panel layout, shortcuts and exclusion rules are configurable in Settings.

On macOS, automatic paste requires permission in **System Settings → Privacy & Security → Accessibility**.

| Shortcut | Action |
| --- | --- |
| `⌘⇧V` / `Ctrl+Shift+V` | Show or hide the panel |
| `↑` `↓` | Select a record; use `←` `→` in the bottom bar |
| `Enter` / `Shift+Enter` | Paste / paste as plain text |
| `⌘1–0` / `Ctrl+1–0` | Paste a visible item, up to ten items |
| `Space` | Preview |
| `⌘F` / `Ctrl+F` | Search |
| `⌘C` / `Ctrl+C` | Copy the selected record |
| `⌘,` / `Ctrl+,` | Open Settings |
| `Backspace` | Delete the selected record |
| `Esc` | Close the preview, clear search or hide the panel |

Hold Command on macOS or Ctrl on Windows to show quick-paste numbers. Check for updates and configure automatic checks or installation in **Settings → About**.

## Development

Requires Go 1.27.3.

```bash
CGO_ENABLED=0 go run .
CGO_ENABLED=0 go test ./...
```

UI translations are in `internal/locales/`. After editing them, run:

```bash
go tool mygo generate
go tool mygo i18n check
```

## Build and release

```bash
# macOS Apple Silicon
go run ./tools/package -arch arm64 -o build/darwin-arm64 -release

# macOS Intel x64
go run ./tools/package -arch amd64 -o build/darwin-amd64 -release

# Windows x64
go tool mygo build -platform windows/amd64 -o build
```

Binaries, installers and update files are written to `build/<platform>-<architecture>/`.

Before releasing, update `version` in `mygo.json` and rebuild. Create a release tagged `v<version>`, then upload the `.tar.gz` archives, `update-*.json` manifests and Windows Setup installer from the three output directories.
