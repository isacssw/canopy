# canopy

**canopy** is a terminal UI for developers running multiple AI coding agents in parallel. One view. All your agents.

![Go version](https://img.shields.io/github/go-mod/go-version/isacssw/canopy)
[![Go Report Card](https://goreportcard.com/badge/github.com/isacssw/canopy)](https://goreportcard.com/report/github.com/isacssw/canopy)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux-blue)

## Table of Contents

- [Why canopy?](#why-canopy)
- [Demo](#demo)
- [Requirements](#requirements)
- [Install](#install)
- [Usage](#usage)
- [Keybinds](#keybinds)
- [How it works](#how-it-works)
- [Config](#config)
- [Contributing](#contributing)
- [Roadmap](#roadmap)
- [License](#license)

```
┌──────────────────────────────┬─────────────────────────────────────┐
│  worktrees                   │  agent output  ← feat/auth-flow     │
│                              │                                     │
│  ● feat/auth-flow            │  ✻ Thinking...                      │
│    main ← feat/auth-flow     │  ⎯ Writing AuthProvider.tsx         │
│    running                   │  ✔ Done. 3 files changed.           │
│                              │                                     │
│  ⚠ feat/onboarding-ui        │                                     │
│    main ← feat/onboarding-ui │                                     │
│    waiting                   │                                     │
│                              │                                     │
│  ✓ fix/email-capture         │                                     │
│    main ← fix/email-capture  │                                     │
│    done                      │                                     │
├──────────────────────────────┴─────────────────────────────────────┤
│ n new  r run  a attach  x kill  d diff  D delete  i send  R refresh│
└────────────────────────────────────────────────────────────────────┘
```

Each worktree gets a dedicated **tmux session** so Claude Code runs in a real terminal with full PTY support. Canopy is the view from above — monitor all your agents at a glance, attach to any one when you need to interact directly.

## Why canopy?

Managing 3–5 parallel AI agents across separate terminals is chaotic — you have no overview of what's running, you're constantly switching windows, and there's no signal for when an agent needs your input. Canopy is the persistent view above all of them: one place to see every agent's state, catch anything waiting for you, and drop in exactly when needed.

## Demo

> 🎥 GIF coming soon — [watch the demo video](#)

## Requirements

- [tmux](https://github.com/tmux/tmux) (any modern version)
- [Claude Code](https://claude.ai/code) (`claude` on your PATH)
- A git repository

## Install

**With Go (recommended):**
```bash
go install github.com/isacssw/canopy/cmd/canopy@latest
```

**From source:**
```bash
git clone https://github.com/isacssw/canopy
cd canopy
go build -o canopy ./cmd/canopy
mv canopy ~/.local/bin/   # or any directory on your PATH
```

## Usage

Run from anywhere inside your git repo:

```bash
canopy
```

First run will prompt for your agent command (default: `claude`).

```bash
canopy --help      # show keybinds and usage
canopy --version   # print version
```

Config is saved to `~/.config/canopy/config.json`.

## Keybinds

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate worktrees |
| `n` | Create new worktree (prompts for branch + base) |
| `r` | Run agent in selected worktree |
| `a` | Attach to the live tmux session (full interactive Claude) |
| `x` | Kill running agent |
| `d` | View git diff for selected worktree |
| `D` | Delete worktree (with confirmation) |
| `i` | Send input to agent when it's waiting |
| `R` | Refresh worktree list |
| `q` | Quit |

### Attaching to an agent

Press `a` to drop into the agent's tmux session and interact with Claude directly. Canopy suspends while you're attached. Press `Ctrl+b d` to detach and return to canopy.

## Agent states

| Icon | State | Meaning |
|------|-------|---------|
| `○` | idle | No agent running |
| `●` | running | Agent active |
| `⚠` | waiting | Agent needs your input — press `i` or `a` |
| `✓` | done | Agent finished cleanly |
| `✗` | error | Agent exited with error |

## How it works

When you press `r`, canopy creates a detached tmux session named `canopy_<repo-hash>_<branch>` and launches your agent command inside it. The output panel shows a live snapshot of the tmux pane, refreshed every 500ms.

Session names include a short hash of the repo root, so branches with the same name across different repos never collide.

Agents keep running after you quit canopy — they're just tmux sessions. You can reattach at any time with `tmux attach -t <session-name>` or by reopening canopy and pressing `a`.

## Config

`~/.config/canopy/config.json`:

```json
{
  "agent_command": "claude"
}
```

You can use any command here, e.g. `claude --dangerously-skip-permissions` or a custom wrapper script.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) — contributions, bug reports, and feature requests are welcome.

## Roadmap

- [ ] Output search / filter
- [ ] Per-agent task label (shown in list)
- [ ] AI summary of last agent run
- [ ] Multi-repo support
- [ ] Export session log

## License

MIT — see [LICENSE](LICENSE)
