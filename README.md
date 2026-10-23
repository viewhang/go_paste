# GoPaste

English | [简体中文](README.zh-CN.md)

A clipboard manager for macOS and Windows, built with Go and [MyGo](https://github.com/egoist/mygo).

## Features

- Clipboard history for text, images, links, colors and files, with source apps and timestamps
- Search, manual lists and rule-based smart lists
- Previews, drag and drop, formatted paste and plain-text paste
- Exclusion rules for apps, confidential content, whitespace-only content and image size
- A bottom bar or a floating panel near the cursor, both resizable
- Classic and Liquid Glass themes, with English and Simplified Chinese

## MyGo

GoPaste uses [MyGo](https://github.com/egoist/mygo)'s native UI:

- GPU-rendered UI written in Go, with no WebView, HTML or JavaScript
- No cgo required; the same codebase cross-compiles to macOS and Windows
- Desktop APIs for the clipboard, tray, global shortcuts, drag and drop, and signed updates
- UI tests and screenshot rendering that run without opening a window

## Memory usage

### 220 records

Measured locally on October 23, 2026: GoPaste 0.1.0 arm64, macOS 27.0.1, Apple M1 Pro, 32 GB RAM. The history contained 220 records, including 42 image records.

| Metric | Physical memory footprint |
| --- | --- |
| Current sample | About 63 MB |
| Peak during this session | About 122 MB |

Measured using the Physical footprint metric from `vmmap -summary`. Usage varies with images, previews and workload.

### 2,000 records

Measured on October 23, 2026 with a local development build (arm64), macOS 27.0.1, Apple M1 Pro and 32 GB RAM.

The test used 2,000 generated records: 800 text, 500 image, 400 file, 200 link and 100 color records. Samples included 4K images, roughly 1 MiB of text in one record and 600 files in another.

| Scenario | Physical memory footprint, median of three runs |
| --- | --- |
| All records loaded | 123.4 MiB |
| Panel open | 133.2 MiB |
| Scrolled to record 2,000 | 108.0 MiB |
| Bottom bar resized repeatedly | 131.5 MiB |
| Panel hidden after testing | 133.0 MiB |
| Peak during each run | 199.3 MiB |

Measured using `vmmap -summary` after three seconds idle at each stage, without forcing garbage collection. Peaks ranged from 199.0 to 207.4 MiB across the three runs.

The test exercised storage and UI in a separate native window, without clipboard monitoring, the tray or the updater. These figures are not a full-app benchmark. Windows has not been measured yet.

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

Requires Go 1.27.1.

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
