# Contributing

Build on Windows with Go 1.25 or later. The prototype uses Go's standard library and Windows system DLLs, with no third-party Go dependencies.

```powershell
go test ./...
go vet ./...
go build -ldflags "-H windowsgui -s -w" -o Lunaria.exe .
```

For native API smoke checks, quit any running instance and run from this folder:

```powershell
Start-Process .\Lunaria.exe -ArgumentList '--smoke-test', 'testdata/sample.png' -Wait
Get-Content .\smoke-result.json
```

These checks exercise window styles, geometry, edit operations, synthetic Shell drop payloads, and surface recreation. Smoke mode does not request keyboard focus. They are not proof of usable desktop behavior.

The checked-in `lunaria_windows_amd64.syso` embeds the Windows compatibility manifest needed for layered child windows. After editing `lunaria.manifest`, regenerate it with `go generate ./...`. This downloads the pinned resource compiler as a build tool, not an application dependency.

Actual computer-use tests must verify the exposed desktop with a non-maximized Explorer window: invoke editing, drop a file, move, resize, rotate, remove, switch back to Explorer without closing editing, maximize Explorer to finish editing, and confirm that passive images remain visible. Inspect screenshots after actions; successful input delivery alone is not a pass.

Rendering uses Windows GDI+ with layered windows. Passive display and editing use separate native windows. The editor remains below ordinary windows and the taskbar, takes focus on invocation unless a drag is active, and exits when a maximized application becomes foreground. On raised Explorer desktops (`Progman` has `WS_EX_NOREDIRECTIONBITMAP`), the passive board is parented to `Progman` between `SHELLDLL_DefView` and the wallpaper `WorkerW`. It uses `SetLayeredWindowAttributes` with GDI painting; per-pixel `UpdateLayeredWindow` content was invisible in this arrangement. The separate top-level editor retains per-pixel alpha. The smoke test checks parentage, sibling order, and the passive layered mode.

Do not accept a parent/style check or `PrintWindow` capture alone as visual proof. A short non-activating DWM thumbnail of `Progman` showed the passive fixture overlapping the Recycle Bin, with the icon and label above it and wallpaper behind it. A prior icon-view-parent experiment showed the image covering the icon despite correct sibling order; that path was rejected. Desktop captures and native smoke results remain local under `docs/verification/` and are excluded from Git. This compositor check does not establish real mouse interaction or Explorer restart recovery.

Keep visible checks brief and announced. Do not minimize or rearrange the user's applications merely to capture the desktop. Treat interference or occlusion as inconclusive, not a product failure or a pass. The passive color-key path flattens partial alpha against a near-black key and treats exact #010203 as transparent; it needs an alpha-capable composition path for fidelity-sensitive transparent images.

`docs/DESIGN.md` records product decisions only. Keep implementation notes out of it.
