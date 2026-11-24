# Lunaria

A native Windows vision board. Keep images on your desktop or float them above applications, then switch into edit mode to arrange them.

## Run

Download **Lunaria.exe** from [GitHub Releases](https://github.com/nekoium/Lunaria/releases), then open it. Lunaria stays in the notification area; Windows may place its icon in the overflow menu. No additional application runtime is required.

- Press **Ctrl+Alt+L**, or click the notification-area icon, to toggle edit mode.
- Press **Ctrl+Alt+Shift+L** to switch between desktop placement and floating above apps. You can also choose **Float above apps** or **Return to desktop** from the tray menu. Switching placement preserves whether you are editing, except that returning to desktop behind a maximized app ends editing.
- Right-click the icon and choose **Add Image…** to select an image. PNG, JPEG, GIF, WebP, AVIF, HEIC, HEIF, BMP, and TIFF are supported; animated or multipage images display their first frame or page. Lunaria detects the format from the file contents, including AVIF images downloaded with a `.jpg` name.
- In edit mode, you can also drop image files onto the board or press **Ctrl+V** to paste copied image files, screenshots, or clipboard images.
- Left-click an image to select it and bring it to the front. Drag its body to move it, a corner to resize it, or the handle extending from its top edge to rotate it. Rotation snaps near quarter turns.
- Press **Delete** to remove the selected image.
- In edit mode, right-click an image and choose **Download** to save its original file in your Downloads folder. Existing files are never replaced.
- Press **Escape**, **Ctrl+Tab**, or **Ctrl+Alt+L** to finish editing. In desktop placement, activating a maximized application also ends editing. Floating edit mode stays open when switching applications.
- Right-click the notification-area icon and choose **Quit** to close Lunaria.

Outside edit mode, clicks pass through the images to the desktop or application underneath. Floating the board in normal mode keeps focus in your current application. Leaving floating edit mode returns focus to the previous app if it is still open and Lunaria still has focus. In desktop placement, ordinary application windows and the taskbar remain above the board. Floating placement uses Windows' standard always-on-top behavior, including over fullscreen apps where Windows permits it; visibility over exclusive fullscreen apps, secure desktops, or other topmost windows is not guaranteed. Editing receives clicks and keyboard focus; during an active file drag it waits until the drop completes. Lunaria starts in desktop normal mode.

## Board storage

Lunaria automatically saves the board in `%LOCALAPPDATA%\Lunaria\board.json` after adding or deleting images and after completing a move, resize, or rotation. Imported images are copied into the adjacent `images` folder, so moving or deleting an original does not change the board. These copies use additional disk space; deleting an image from the board removes its copy after a successful save.

Board order, positions, sizes, and rotations are restored on launch. Saves replace the JSON atomically. If a save fails, Lunaria reports the error and keeps the current session in memory; the previous saved board remains intact. An unreadable board or a missing/damaged stored image prevents startup rather than silently replacing saved data. Keep the JSON and image folder together when backing up the board.

## Current limits

This prototype supports Windows and the primary monitor only. It has no settings, automatic startup, or configurable shortcuts. Images larger than 40 megapixels are rejected.

Images stay local. Lunaria does not upload them or connect to a service. HEIC/HEIF import uses a bundled pure Go HEVC decoder, applies container rotation, and displays the primary image or first sequence frame. No Windows codec installation is needed. HEIF files using other codecs may be unsupported. HEIC colors remain in the source color space; ICC profiles and HDR are not converted for display. Lunaria keeps smaller display copies in memory, capped to a 1080p screen budget. Enlarging an image can make it softer; **Download** saves the unchanged original. Images load one at a time in the background so you can keep editing. Large AVIF files can still take several seconds; Lunaria shows a loading hint and a notification for a slow import. Temporary memory use can rise while decoding.

In desktop placement, passive images appear below desktop icons. On newer Windows desktop layouts, partially transparent image areas can appear darker in desktop normal mode, and exact #010203 pixels are treated as transparent. Floating placement and edit mode preserve per-pixel transparency. Desktop attachment depends on Explorer; if no suitable desktop host is available, the board stays hidden rather than moving above icons. Both shortcuts must be available; Lunaria reports a conflict if another application has registered either one.

## Build

Install Go 1.26.4 or later, then run from this folder on Windows:

```powershell
go build -ldflags "-H windowsgui -s -w" -o Lunaria.exe .
```

## License

Lunaria is available under the [MIT License](LICENSE). Bundled decoder and dependency licenses are included in [third-party notices](THIRD_PARTY_NOTICES.txt).
