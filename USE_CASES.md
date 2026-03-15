# Canopy Use Cases

Canopy is a multiplexer for Claude Code agents — one dashboard, many parallel worktrees. This document covers how it fits into different workflows, with a focus on developers already comfortable in the terminal.

---

## For tmux users

You already live in tmux. You know the value of persistent sessions and parallel panes. Canopy extends that mental model to AI agents.

**Without canopy** your flow might look like:
- Open a new tmux window
- `cd` to a worktree or create one with `git worktree add`
- Start `claude` manually
- Switch windows to check progress
- Repeat for each task

**With canopy** that becomes:
- `n` to create the worktree and branch
- `r` to launch claude in it
- Arrow keys to scan all agents at a glance
- `a` to drop into any session when needed, `Ctrl+B D` to come back

The agents are still just tmux sessions (`canopy_<hash>_<branch>`). You can manage them directly with tmux if you want — canopy is the dashboard on top, not a cage.

---

## For Neovim users

You probably already have a terminal split or a floating terminal plugin (toggleterm, lazygit, etc). Canopy fits naturally alongside your editor workflow.

**Scenario: working on a feature while canopy handles the boring parts**

You are writing the core logic of a feature in Neovim. Meanwhile:
- Canopy is running an agent on `chore/update-deps` updating your dependencies
- Another agent on `fix/lint-errors` is clearing the lint backlog
- A third on `docs/api-reference` is generating documentation from your code

None of these block you. You check canopy when you want a status update, attach to one if it needs input, and keep editing.

**Scenario: reviewing agent output without leaving your editor**

Run canopy in a split terminal inside Neovim (`:terminal` or toggleterm). The output panel shows what each agent is doing. No context switching to a browser or separate window.

---

## Parallel feature development

**Problem**: You have a large feature that can be broken into independent sub-tasks — API layer, UI components, tests, migrations. Doing them sequentially in one claude session is slow and creates a long, tangled conversation.

**With canopy**:
- `feat/api-layer` → agent writing the API endpoints
- `feat/ui-components` → agent building the React components
- `feat/db-migrations` → agent writing the migration files
- `feat/tests` → agent writing the test suite

All four run simultaneously. Each agent has its own isolated worktree so there are no file conflicts. You review and merge when they're done.

---

## Exploration vs. implementation

You want to try two different approaches to the same problem without committing to either.

- `spike/approach-a` → agent tries solution A
- `spike/approach-b` → agent tries solution B

Review both diffs with `d`, pick the winner, delete the other with `D`.

---

## Long-running tasks while you do other work

Some claude tasks take time — refactoring a large module, generating boilerplate, writing documentation. With canopy you fire them off and forget them until they're done.

The `✓ done` status tells you when to look. The `⚠ waiting` status tells you when the agent needs a decision. You don't need to babysit the terminal.

---

## Bug fixing across multiple issues

You have a backlog of small bugs. Instead of fixing them one by one in a single session:

- `fix/issue-123` → agent fixing bug 123
- `fix/issue-124` → agent fixing bug 124
- `fix/issue-125` → agent fixing bug 125

Each fix is isolated on its own branch. Each diff is clean and reviewable on its own.

---

## Working across a monorepo

Your monorepo has a frontend, backend, and infra directory. Changes often need to happen in all three.

- `feat/auth-backend` → agent updating the Go auth service
- `feat/auth-frontend` → agent updating the React login flow
- `feat/auth-infra` → agent updating the Terraform IAM rules

Same feature, three worktrees, three agents, all running in parallel.

---

## Keeping the main branch clean

Every agent works on its own branch in its own worktree. Your main branch is never touched. You review the diff (`d`), merge what looks good, and delete the worktree (`D`) when done. The same workflow you already use for PRs, but the branch was written by an agent.

---

## Session persistence across interruptions

You close your laptop. The tmux sessions keep running on a remote machine or your desktop. When you reopen canopy, the agents are still there — their status, output snapshot, everything. Canopy reconnects to the existing sessions automatically on the next poll cycle.

---

## Quick reference: when to use which key

| Situation | Key |
|-----------|-----|
| Start a new isolated task | `n` then `r` |
| Check what an agent is doing | Arrow keys, read output panel |
| Agent needs a yes/no decision | `a` to attach, answer, `Ctrl+B D` to return |
| Agent is stuck or went wrong | `x` to kill, `r` to restart |
| Review what the agent changed | `d` for the diff |
| Task is done, clean up | `D` to delete the worktree |
| Launched too many agents | `x` on idle ones to free resources |
