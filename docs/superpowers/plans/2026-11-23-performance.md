# Issue 8 Performance Implementation Plan

**Goal:** Reduce idle memory and editing lag, keep imports responsive, and let users download original images.

**Architecture:** Keep a bounded display bitmap for each image and preserve its exact original on disk. Run one import at a time off the UI thread. Apply completed imports on the existing UI timer, then persist the board. Download copies originals through an edit-mode context menu.

**Tech stack:** Go 1.26.4, Win32, GDI+, and existing dependencies only.

**Spec:** [Performance requirements](../../PRD-performance.md). Read the spec before changing code. This guide records the final implementation, including the AVIF fallback chosen under the owner's delegated completion instruction.

## Workspace and safety

- Use the Orca worktree on `nekoium/lunaria-issue-8`, based on `origin/main`. Do not modify the main checkout's product code.
- Keep temporary files under `.tmp/issue-8/`; keep local measurement logs under `docs/verification/issue-8/`. Both are ignored.
- Never read, change, or delete the user's saved `%LOCALAPPDATA%\Lunaria` board. Set a temporary `LOCALAPPDATA` for app checks.
- Leave an existing Lunaria process alone. Close only the test process, through its own tray-host `WM_CLOSE` message or tray Quit command.
- Run background PowerShell with `Start-Process powershell -WindowStyle Hidden`. Keep native visible checks brief and announced. Do not rearrange the user's applications.
- Add no dependencies, codec installation requirement, or unrelated refactor. Do not push, merge, or publish.
- Resolve recoverable fixture, tool-path, and test-harness mistakes directly. Record genuine product limitations rather than hiding them or blocking unrelated tasks.

## Constraints and review focus

- Stable memory goal: under 100 MB; prefer under 50 MB. Measure Task Manager's active private working set after 30 seconds idle. Distinguish this from total working set and private committed bytes.
- Keep originals byte for byte, with persisted transforms independent of display resolution.
- Keep all nine supported formats. Preserve premultiplied BGRA alpha and existing first-frame/page semantics.
- Adding should take under one second. Large AVIF is the documented exception with the bundled decoder; a warning and background loading are the fallback.
- A 20-image drawing pass at 1920 × 1080 averages under 16 ms. This does not by itself establish full interactive frame rate.
- Queue, complete, fail, and quit imports without racing UI state, retaining clipboard handles, reopening editing, losing placement, or leaving an unsaved copied original.

## Task 1: Reproducible measurements

**Files:** `performance_windows_test.go`, `CONTRIBUTING.md`.

- [x] Add gated `TestIssue8Measure`, enabled only by `LUNARIA_MEASURE=1`.
- [x] Verify the issue #4 photo is AVIF at 4000 × 6000. Generate equivalent-sized gradient JPEG and PNG fixtures locally.
- [x] Load 7 AVIF, 7 JPEG, and 6 PNG images. `LUNARIA_MEASURE_COUNT` can restrict the baseline to three images.
- [x] Log load times, process peak and settled memory, average GDI+ drawing time, and ten dispose/reload rounds. Native API errors must fail rather than log zero memory.
- [x] Record the baseline before changing the loader. The baseline is expected to fail targets.

The GitHub attachment in issue #4 is an error screenshot. Download an AVIF rendition of the Pexels photo linked in the issue instead:

```powershell
New-Item -ItemType Directory -Force .tmp/issue-8, docs/verification/issue-8
Invoke-WebRequest 'https://images.pexels.com/photos/19428921/pexels-photo-19428921.jpeg?fm=avif' -OutFile .tmp/issue-8/issue4.avif
$env:LUNARIA_MEASURE='1'
$env:LUNARIA_MEASURE_COUNT='3'
go test -run '^TestIssue8Measure$' -v -timeout 20m .
Remove-Item Env:LUNARIA_MEASURE, Env:LUNARIA_MEASURE_COUNT
```

This is a current rendition of the reported photo, not proof of byte equality with the original historical download. Do not commit the photograph.

## Task 2: Bound retained image pixels

**Files:** `geometry.go`, `geometry_test.go`, `render_windows.go`, `image_windows_test.go`.

- [x] Add `fitSize(w, h, maxW, maxH int) (int, int)`: retain aspect ratio, never upscale, round dimensions, and keep thin images at least one pixel wide/high.
- [x] Add `defaultImageBounds() (int, int)`: 55% of screen width/height, capped to a 1920 × 1080 screen budget. Maximum retained size is 1056 × 594.
- [x] In `readImage`, keep existing source-dimension validation. Scale decoded pixels before allocating the retained GDI+ bitmap. Use existing `golang.org/x/image/draw.BiLinear` for reductions and the existing direct draw for small images.
- [x] Preserve premultiplication, red/blue swap, the Scan0 buffer lifetime comment, and `runtime.KeepAlive`.
- [x] Return free decoder memory to Windows in `loadImage` with `debug.FreeOSMemory()` after the decoded source is no longer needed.
- [x] Test small, portrait, landscape, thin, and partially transparent images. Existing saved-board tests must continue to pass.
- [x] Run the full measurement. Keep an AVIF timing failure visible in the results and continue the remaining tasks with the documented fallback.

## Task 3: Download originals

**Files:** `download_windows.go`, `download_windows_test.go`, `main_windows.go`, `README.md`, `docs/DESIGN.md`.

- [x] Implement `downloadOriginal(storage, file, folder string) (string, error)`. Validate stored names, detect the real format, choose an extension, and stream the original bytes unchanged.
- [x] Create the output with `O_EXCL`; try numbered suffixes through 1000 attempts. Never truncate an existing file. Remove a newly created incomplete file on copy, sync, or close failure.
- [x] Implement `downloadsFolder() (string, error)` with `SHGetKnownFolderPath` and `CoTaskMemFree`. The Downloads GUID is `374de290-123f-4565-9164-39c4925e467b`.
- [x] Implement `(a *application) imageMenu(p vector)` and route board `WM_RBUTTONUP` to it. Hit-test the top image, finish any drag, select it, then offer Download only in edit mode.
- [x] Test exact bytes, AVIF detection independent of filename, duplicate output preservation, bad source/destination paths, and known-folder lookup without writing to the real Downloads folder.

## Task 4: Responsive imports

**Files:** `import_windows.go`, `import_windows_test.go`, `main_windows.go`, `clipboard_windows.go`, `clipboard_windows_test.go`, `render_windows.go`.

- [x] Keep the queue and active-request metadata on the UI thread. Give one worker a storage-path snapshot and private input data. Transfer the completed bitmap through a buffered channel.
- [x] Poll results from the existing 150 ms UI timer. Apply results in order. Delay insertion during a drag or modal dialog. Preserve the user's current edit-mode state.
- [x] Queue file-selection, file-drop, clipboard-file, PNG, and bitmap imports. Copy clipboard data while it is owned; do not decode or PNG-encode clipboard bitmaps on the UI thread. Keep test and smoke low-level helpers synchronous.
- [x] Preserve paste offsets and clamped drop positions. Save completed imports through the existing atomic board persistence path.
- [x] Add an importing hint and a nonblocking notification after one second, once per session. Do not claim a preview or notification satisfies the timing goal.
- [x] Before GDI+ shutdown, wait for the active worker, dispose its uncommitted bitmap, remove its unsaved original copy, and discard queued requests.
- [x] Test queue serialization, clipboard replacement after return, drag protection, closed edit mode, invalid imports, original bytes, and shutdown cleanup.
- [x] Add gated `TestIssue8ImportResponsiveness` to measure queue latency and repeated hidden-window dragging during the large AVIF decode.

## Task 5: Final validation and documentation

**Files:** `CONTRIBUTING.md`, `README.md`, `docs/DESIGN.md`, `docs/PRD-performance.md`, this guide, and `docs/PERFORMANCE.md`.

- [x] Run `go test ./...`, `go vet ./...`, and a stripped Windows build.
- [x] Run the responsiveness measurement with the large AVIF.
- [x] Start the built app with 20 original files in temporary storage. Wait 30 seconds after startup. Log monitor dimensions, active private working set, total working set, private committed bytes, peak working set, and native message responsiveness. Off-screen test geometry avoids covering the user's desktop; disclose that limitation.
- [x] Run the existing native smoke test and inspect its result. Do not equate native messages with subjective visual approval.
- [x] Document results, their limits, the retained-image quality trade-off, and slow AVIF behavior. Keep test reports and engineering detail out of the public README.
- [x] Review the final diff, commit the work, and report the branch, build artifact, checks, and remaining limitations. Do not push, merge, or publish.

## Commands

```powershell
go test ./...
go vet ./...
go build -ldflags '-H windowsgui -s -w' -o .tmp/issue-8/Lunaria.exe .
$env:LUNARIA_MEASURE='1'
go test -run '^TestIssue8ImportResponsiveness$' -v -timeout 1m .
Remove-Item Env:LUNARIA_MEASURE
```

For the full loader measurement, use `go test -run '^TestIssue8Measure$' -v -timeout 20m .` with `LUNARIA_MEASURE=1`. It intentionally reports failure when AVIF exceeds one second. Record that gap; do not weaken the assertion to make the report green.
