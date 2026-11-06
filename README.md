# Lunaria

A native Windows vision board. Keep images visible on your desktop, then switch into edit mode to arrange them.

## Run

Download **Lunaria.exe** from [GitHub Releases](https://github.com/nekoium/Lunaria/releases), then open it. Lunaria stays in the notification area; Windows may place its icon in the overflow menu. No additional application runtime is required.

- Press **Ctrl+Alt+L**, or click the notification-area icon, to toggle edit mode.
- Right-click the icon and choose **Add Image…** to select an image. PNG, JPEG, and GIF are supported; GIF displays its first frame.
- In edit mode, you can also drop image files onto the board.
- Click an image to select it. Drag its body to move it, a corner to resize it, or the handle extending from its top edge to rotate it. Rotation snaps near quarter turns.
- Press **Delete** to remove the selected image.
- Press **Escape**, **Ctrl+Tab**, or **Ctrl+Alt+L** to finish editing. Activating a maximized application also ends editing. Switching to non-maximized Explorer or another non-maximized application keeps editing open.
- Right-click the notification-area icon and choose **Quit** to close Lunaria.

Desktop icons work normally outside edit mode. During editing, the board receives desktop clicks. Ordinary application windows and the taskbar remain above the board. Calling the editor gives it keyboard focus; during an active file drag it waits until the drop completes.

## Current limits

This prototype supports Windows and the primary monitor only. It does not save the board between launches. It has no settings, automatic startup, or configurable shortcuts. Images larger than 40 megapixels are rejected.

Images stay local. Lunaria does not upload them or connect to a service.

Passive images are placed below desktop icons and pass clicks through. On newer Windows desktop layouts, partially transparent image areas can appear darker in normal mode, and exact #010203 pixels are treated as transparent. Edit mode preserves per-pixel transparency. Desktop attachment depends on Explorer; if no suitable desktop host is available, the board stays hidden rather than moving above icons. Ctrl+Alt+L must be available; Lunaria reports a conflict if another application has registered it.

## Build

Install Go 1.25 or later, then run from this folder on Windows:

```powershell
go build -ldflags "-H windowsgui -s -w" -o Lunaria.exe .
```

## License

Lunaria is available under the [MIT License](LICENSE).
