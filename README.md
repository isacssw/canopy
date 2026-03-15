# canopy

A terminal UI orchestrator for running multiple Claude Code agents across git worktrees.

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

## Requirements

- [tmux](https://github.com/tmux/tmux) (any modern version)
- [Claude Code](https://claude.ai/code) (`claude` on your PATH)
- A git repository

## Install

```bash
git clone https://github.com/you/canopy
cd canopy
go build -o canopy ./cmd/canopy
sudo mv canopy /usr/local/bin/
```

## Usage

Run from anywhere inside your git repo:

```bash
canopy
```

First run will prompt for your agent command (default: `claude`).

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

Press `a` to drop into the agent's tmux session and interact with Claude directly. Canopy suspends while you're attached. Press `Ctrl+B D` to detach and return to canopy.

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

## Roadmap

- [ ] Output search / filter
- [ ] Per-agent task label (shown in list)
- [ ] AI summary of last agent run
- [ ] Multi-repo support
- [ ] Export session log
