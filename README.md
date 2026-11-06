# Lunaria

A native Windows vision board. Keep images visible on your desktop, then switch into edit mode to arrange them.

## Run

Download **Lunaria.exe** from [GitHub Releases](https://github.com/nekoium/Lunaria/releases), then open it. Lunaria stays in the notification area; Windows may place its icon in the overflow menu. No additional application runtime is required.

- Press **Ctrl+Alt+L**, or click the notification-area icon, to toggle edit mode.
- Right-click the icon and choose **Add Image…** to select an image. PNG, JPEG, GIF, WebP, AVIF, BMP, and TIFF are supported; animated or multipage images display their first frame or page. Lunaria detects the format from the file contents, including AVIF images downloaded with a `.jpg` name.
- In edit mode, you can also drop image files onto the board or press **Ctrl+V** to paste copied image files, screenshots, or clipboard images.
- Click an image to select it. Drag its body to move it, a corner to resize it, or the handle extending from its top edge to rotate it. Rotation snaps near quarter turns.
- Press **Delete** to remove the selected image.
- Press **Escape**, **Ctrl+Tab**, or **Ctrl+Alt+L** to finish editing. Activating a maximized application also ends editing. Switching to non-maximized Explorer or another non-maximized application keeps editing open.
- Right-click the notification-area icon and choose **Quit** to close Lunaria.

Desktop icons work normally outside edit mode. During editing, the board receives desktop clicks. Ordinary application windows and the taskbar remain above the board. Calling the editor gives it keyboard focus; during an active file drag it waits until the drop completes.

## Board storage

Lunaria automatically saves the board in `%LOCALAPPDATA%\Lunaria\board.json` after adding or deleting images and after completing a move, resize, or rotation. Imported images are copied into the adjacent `images` folder, so moving or deleting an original does not change the board. These copies use additional disk space; deleting an image from the board removes its copy after a successful save.

Board order, positions, sizes, and rotations are restored on launch. Saves replace the JSON atomically. If a save fails, Lunaria reports the error and keeps the current session in memory; the previous saved board remains intact. An unreadable board or a missing/damaged stored image prevents startup rather than silently replacing saved data. Keep the JSON and image folder together when backing up the board.

## Current limits

This prototype supports Windows and the primary monitor only. It has no settings, automatic startup, or configurable shortcuts. Images larger than 40 megapixels are rejected.

Images stay local. Lunaria does not upload them or connect to a service. Large AVIF images can take several seconds to import and use substantial temporary memory.

Passive images are placed below desktop icons and pass clicks through. On newer Windows desktop layouts, partially transparent image areas can appear darker in normal mode, and exact #010203 pixels are treated as transparent. Edit mode preserves per-pixel transparency. Desktop attachment depends on Explorer; if no suitable desktop host is available, the board stays hidden rather than moving above icons. Ctrl+Alt+L must be available; Lunaria reports a conflict if another application has registered it.

## Build

Install Go 1.25 or later, then run from this folder on Windows:

```powershell
go build -ldflags "-H windowsgui -s -w" -o Lunaria.exe .
```

## License

Lunaria is available under the [MIT License](LICENSE). Bundled decoder and dependency licenses are included in [third-party notices](THIRD_PARTY_NOTICES.txt).
