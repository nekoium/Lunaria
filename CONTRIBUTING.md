# Contributing

Build on Windows with Go 1.26.4 or later. The prototype uses Windows system DLLs for its interface and rendering. BMP, TIFF, and WebP decoding use `golang.org/x/image`. AVIF decoding uses a bundled decoder through a Go WebAssembly runtime; users do not need to install an image codec.

```powershell
go test ./...
go vet ./...
go build -ldflags "-H windowsgui -s -w" -o Lunaria.exe .
```

`TestIssue8Measure` checks the performance goals in `docs/PRD-performance.md`. Download the issue #4 photo as described in `docs/superpowers/plans/2026-11-23-performance.md`; its GitHub attachment is only a screenshot. With the AVIF at `.tmp/issue-8/issue4.avif`, run `$env:LUNARIA_MEASURE='1'; go test -run '^TestIssue8Measure$' -v -timeout 20m .`. The test measures its own total process working set, not Task Manager's active private working set or the built app. Large AVIF currently exceeds one second, so this optional measurement intentionally reports failure. Keep that gap visible. Run `go test -run '^TestIssue8ImportResponsiveness$' -v -timeout 1m .` with the same environment to check dragging during AVIF loading. Remove `LUNARIA_MEASURE` afterward. See `docs/PERFORMANCE.md` for results and limitations.

Normal imports run one at a time off the UI thread. Queue and completion handling stay on the UI thread; workers never mutate board state. Clipboard tests use a private window station, and queue tests use hidden native windows. For built-app measurements, use temporary `LOCALAPPDATA` and launch any background PowerShell with `Start-Process powershell -WindowStyle Hidden` to avoid opening a console on the user's desktop.

HEIC/HEIF decoding uses `github.com/gen2brain/h265/heic`, with automatic container transforms, a 40-million-pixel coded-frame limit, and at most four decoding workers. It selects the primary image or first sequence frame. RGB output retains the source color space; no additional ICC conversion or HDR tone mapping is applied. Runtime files do not require Windows HEIF/HEVC extensions.

Include `THIRD_PARTY_NOTICES.txt` with binary distributions. Animated WebP import extracts and composites the first frame using the existing WebP decoder; it does not decode the remaining frames.

For native API smoke checks, quit any running instance and run from this folder:

```powershell
Start-Process .\Lunaria.exe -ArgumentList '--smoke-test', 'testdata/sample.png' -Wait
Get-Content .\smoke-result.json
```

These checks exercise window styles, geometry, edit operations, synthetic Shell drop payloads, and surface recreation. Smoke mode does not request keyboard focus and does not read or write the user's saved board. They are not proof of usable desktop behavior.

Board persistence tests use isolated temporary storage and cover restart restoration, source removal, deletion, completed drags, invalid metadata, failed atomic replacement, and smoke isolation. For manual restart testing, add images, move/resize/rotate them, quit through the tray, and relaunch; confirm image order and transforms persist even after moving the original files. Do not delete or edit the user's existing `%LOCALAPPDATA%\Lunaria` folder for testing.

The checked-in `lunaria_windows_amd64.syso` embeds the Windows compatibility manifest needed for layered child windows and the application icon. The approved seedless B1 overlapping-pods artwork is `assets/lunaria.png`. After editing the manifest or source artwork, run `go generate ./...` to regenerate `assets/lunaria.ico` and the Windows resources. The icon includes 16, 20, 24, 32, 40, 48, 64, 128, and 256 px sizes; the tray loads the matching small system icon size from those same resources. Generation downloads the pinned resource compiler as a build tool, not an application dependency.

Actual computer-use tests must verify the exposed desktop with a non-maximized Explorer window: invoke editing, drop a file, move, resize, rotate, remove, switch back to Explorer without closing editing, maximize Explorer to finish editing, and confirm that passive images remain visible. Inspect screenshots after actions; successful input delivery alone is not a pass.

Rendering uses Windows GDI+ with layered windows. Desktop display and editing use separate native windows. In desktop placement, the editor remains below ordinary windows and the taskbar, takes focus on invocation unless a drag is active, and exits when a maximized application becomes foreground. Floating placement reuses the top-level editor with native topmost ordering; normal mode adds click-through and no-activation styles and removes the editing background and controls. Both shortcuts toggle independent settings. `floating_windows_test.go` exercises native window styles, foreground preservation, selection/content preservation, fade reversal, return to desktop, and Explorer/display recovery without starting another Lunaria instance or accessing saved board data. It also checks immediate desktop auto-exit behind a maximized app, including when the editor or tray has focus, and preserves editing for non-maximized, hidden, and minimized apps. Foreground lookup is substituted during these regression checks so tests do not activate the user's applications. These API checks do not establish visual ordering over actual fullscreen applications or real mouse interaction. Manually verify all four states, clicks through images into another app, top editing over a maximized app, and both shortcuts while fullscreen. On raised Explorer desktops (`Progman` has `WS_EX_NOREDIRECTIONBITMAP`), the passive board is parented to `Progman` between `SHELLDLL_DefView` and the wallpaper `WorkerW`. It uses `SetLayeredWindowAttributes` with GDI painting; per-pixel `UpdateLayeredWindow` content was invisible in this arrangement. The separate top-level editor retains per-pixel alpha. The smoke test checks parentage, sibling order, and the passive layered mode.

Do not accept a parent/style check or `PrintWindow` capture alone as visual proof. A short non-activating DWM thumbnail of `Progman` showed the passive fixture overlapping the Recycle Bin, with the icon and label above it and wallpaper behind it. A prior icon-view-parent experiment showed the image covering the icon despite correct sibling order; that path was rejected. Desktop captures and native smoke results remain local under `docs/verification/` and are excluded from Git. This compositor check does not establish real mouse interaction or Explorer restart recovery.

Keep visible checks brief and announced. Do not minimize or rearrange the user's applications merely to capture the desktop. Treat interference or occlusion as inconclusive, not a product failure or a pass. The passive color-key path flattens partial alpha against a near-black key and treats exact #010203 as transparent; it needs an alpha-capable composition path for fidelity-sensitive transparent images.

## Release a portable package

Run the normal tests and vet checks above, then package on Windows x64:

```powershell
.\tools\Package.ps1 -Version 0.1.0
```

The script builds with `-trimpath` to omit local source paths, then writes a versioned ZIP and SHA-256 file under `.tmp/release-<version>/`. Use a fresh staging folder for each build. The ZIP includes the README, its icon asset, license, and third-party notices.

Before publishing, scan tracked files and Git history for secrets, verify package contents, and update `bucket/lunaria.json` with the version, permanent release URL, and ZIP hash. Record user-facing notes under `docs/releases/`. Commit and push, tag that commit with the version (for example, `0.1.0`), and publish the ZIP and checksum as GitHub release assets. Do not replace published ZIPs; Scoop relies on their hashes. Download the published ZIP to verify its checksum, then check installation through the project-owned Scoop bucket.

Scoop's `checkver` and `autoupdate` fields support maintainer updates; this repository does not update the bucket automatically. Board data remains outside the installation directory and must not be removed during upgrades or uninstalls.

`docs/DESIGN.md` records product decisions only. Keep implementation notes out of it.
