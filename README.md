# claude-boot

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

An interactive launcher for [Claude Code](https://claude.ai/code). Before starting a Claude session, it lets you:

- Toggle installed **plugins** and **skills** on/off
- Pick the **model** and **thinking effort** level
- Apply saved **profiles** (preset combinations of the above)

All choices are persisted to your project's `.claude/settings.json` — including model and thinking effort — so they're remembered the next time you run `claude-boot` in that repo, and plain `claude` inherits the plugin/skill toggles too.

## Install

Download the binary for your platform from the [latest release](../../releases/latest):

| Platform              | File                            |
|-----------------------|---------------------------------|
| macOS (Apple Silicon) | `claude-boot-darwin-arm64`      |
| Linux x86-64          | `claude-boot-linux-amd64`       |
| Windows x86-64        | `claude-boot-windows-amd64.exe` |

Then make it executable and move it onto your PATH (macOS/Linux):

```bash
chmod +x claude-boot-darwin-arm64
mv claude-boot-darwin-arm64 /usr/local/bin/claude-boot
```

Verify the download with the provided `checksums.txt`:

```bash
sha256sum -c checksums.txt
```

## Usage

```bash
claude-boot                  # open TUI, then launch claude
claude-boot -p <profile>     # apply a saved profile and launch immediately
claude-boot --restore        # remove claude-boot settings from project .claude/settings.json
```

## Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/akang1108/claude-boot.git
cd claude-boot
make install   # runs tests and installs to $GOPATH/bin
```
