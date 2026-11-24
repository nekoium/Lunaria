# Floating Board Implementation Plan

> **For agentic workers:** Execute the tasks below in order within the isolated issue #12 worktree.

**Goal:** Add independent desktop/top placement and normal/edit interaction states.

**Architecture:** Reuse the desktop display and top-level editor windows. Add a floating flag, native topmost placement, and shared editor setup for interactive and passive top display.

**Tech Stack:** Go 1.26.4, Win32, GDI+; no added dependencies.

**Spec:** `docs/superpowers/specs/2026-11-24-floating-board-design.md`

## Global Constraints

- Ctrl+Alt+Shift+L toggles placement, preserving editing state.
- Ctrl+Alt+L toggles editing, preserving placement.
- Passive top mode passes clicks through without taking focus.
- Standard Windows topmost ordering; no custom taskbar/fullscreen policy.
- Preserve desktop attachment, persistence, clipboard handling, and edit fades.
- Work only in this isolated worktree.

## Review Focus

- Rapid switching during an edit fade must not restore the wrong surface.
- Returning from top edit to desktop edit must remove topmost ordering.
- Explorer restart must not reattach the top overlay as a desktop child.
- Display changes must resize the passive overlay while preserving top placement.
- A conflicting overlay shortcut must be reported with normal cleanup.

### Task 1: Four-state window behavior

**Files:** `main_windows.go`, `editor_windows.go`, `render_windows.go`, `win32.go`, new `floating_windows.go`, new `floating_windows_test.go`.

- [x] Add native window tests for all four states, placement-preserving editing, click-through/no-activation styles, topmost removal, fade reversal, display recovery, and focus preservation. Check the top-edit foreground-maximization guard in code review; verify real fullscreen behavior manually.
- [x] Register the second hotkey and add tray placement toggle.
- [x] Share top-level surface creation and rendering; keep desktop-specific ordering and paint paths restricted to desktop placement.
- [x] Run focused native tests, then the existing Go test suite, vet, and build.

### Task 2: Product and contributor documentation

**Files:** `README.md`, `docs/DESIGN.md`, `CONTRIBUTING.md`.

- [x] Document both shortcuts and all four modes, including native topmost limitations.
- [x] Record native verification coverage and manual checks in contributor documentation.
- [x] Review the complete diff and confirm the shared checkout and other worktrees remain untouched.
