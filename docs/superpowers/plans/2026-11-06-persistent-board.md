# Persistent board implementation plan

**Goal:** Restore the user's board after restart without depending on original image files.

**Approved design:** The user approved `%LOCALAPPDATA%\Lunaria\board.json`, managed image copies in `images`, and automatic saves after add/delete/completed transforms. JSON replacement must be atomic, save errors visible, and unreadable existing board data preserved. Source-path references and exit-only saves were considered and rejected because originals can move and interrupted sessions can lose edits.

**Architecture:** Add a Windows persistence file using standard-library JSON/file operations and Windows atomic replacement. Store versioned ordered records with managed filenames and geometry, never native bitmap handles. Restore through the existing decoder; refuse startup on invalid metadata or missing/corrupt copies rather than silently overwriting a partial board. Smoke mode bypasses user storage.

**Scope:** Medium because this introduces durable user data and local copies. Keep baseline and changed runnable Windows binaries for user testing; do not commit/push/integrate until testing approval.

## Work

- Add `storage_windows.go` and tests covering ordered geometry round trips, original source deletion, empty/deleted boards, corrupt/unsupported metadata, missing images, unsafe filenames, failed saves preserving existing data, and smoke isolation.
- Connect startup load, imports, deletion, drag completion/capture loss, and final drop coordinates in `main_windows.go`; add only a managed filename field to `boardImage`.
- Update README and product design to reflect user-approved persistence; no settings, migrations, cloud storage, or UI additions.
- Run `go test ./...`, `go vet ./...`, and a Windows build. Do not run desktop smoke while an existing user instance could be affected.
- Preserve baseline revision `4d40df077e2c7d67f082bd46dfa893bbdf241374` and an exact source patch/revision for the changed binary under `.tmp/issue-7`; save a Rim state and report artifacts for user review.

## Data protection

Validate version, finite positive geometry, and managed filenames before loading. Atomically replace JSON only after fully writing and syncing a sibling temporary file. Remove deleted image copies only after successful metadata save; preserve copies when saving fails. Do not scan or change existing user data during tests; all storage tests use temporary directories inside this worktree.
