# Hide Lunaria Implementation Plan

> **For agentic workers:** Execute this small change directly in the current session under the user's authorization to finalize automatically.

**Goal:** Add a checked tray toggle that temporarily hides the board.

**Architecture:** Keep one session-only `application.hidden` flag. Centralize concealment in `showBoard`, stop edit transitions in `setHidden(bool)`, and guard rendering and interaction entry while hidden.

**Tech Stack:** Go 1.26.4, existing Win32 APIs.

**Spec:** `docs/superpowers/specs/2026-11-25-hide-lunaria-design.md`

## Global Constraints

No new dependencies, shortcuts, timers, or storage changes. Preserve saved images and transforms. Tests must avoid the user's board and keyboard focus.

## Review Focus

- Hidden surfaces remain hidden after Explorer/display recovery or delayed fade messages.
- Completed background imports do not restore visibility.
- Editing, fading, dragging, and floating retain correct state after hiding.
- Native menu check state matches visibility and Quit remains enabled.
- Unhiding after surface destruction recreates the correct placement.

### Task 1: Add and verify temporary hiding

**Files:** Modify `main_windows.go`, `floating_windows.go`, `render_windows.go`, `README.md`, `docs/DESIGN.md`, and `CONTRIBUTING.md`. Add `visibility_windows_test.go`.

**Interfaces:** Add `(*application).setHidden(bool)` and `(*application).createTrayMenu() uintptr`; reuse `showBoard() error` and existing native-window test conventions.

- [x] Write a native regression test covering menu flags, both placements, immediate fade/drag cleanup, hidden shortcut behavior, recovery, import completion, and content preservation.
- [x] Run the targeted test and confirm it fails before implementation.
- [x] Add the flag and setter, guard edit/placement entry and hidden rendering, and add the native checkable menu command.
- [x] Run the targeted test, full `go test ./...`, `go vet ./...`, GUI build, and `git diff --check`.
- [x] Document the tray control and session behavior, and review the connected code and diff.

Finalize by committing, opening and merging a PR closing #14 under the user's authorization.
