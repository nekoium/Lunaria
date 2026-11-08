# Lunaria — Product Design

Lunaria is a desktop vision board, following Myosotis in the series. It keeps images visible above the wallpaper and behind desktop icons without covering ordinary application windows.

This document records the current essential product design. Omitted features are not a deferred backlog. The design can grow during building and testing.

## Scope

- Windows initially; other platforms are planned.
- Images only.
- One board on the primary monitor.

## Normal use

- Images appear behind desktop icons and above the wallpaper.
- Mouse input passes through the board. Desktop icons, file operations, and right-click context menus work normally.
- Ordinary application windows stay above the board.

## Edit mode

A keyboard shortcut toggles edit mode.

During editing:

- A semi-transparent white overlay appears behind the images. Images remain fully opaque.
- Only the overlay animates: it fades in over 150 ms and out over 90 ms, moving quickly at first and settling gently. Editing controls appear and disappear immediately. Desktop input returns as soon as editing ends. Windows' disabled-animation preference skips the fades.
- A visible hint tells the user that edit mode is active, in addition to the overlay.
- The board becomes interactive and accepts clicks and drags.
- The board stays below ordinary application windows.
- Editing may take keyboard focus. There is no requirement to preserve another application's focus.
- Click an image to select it and show its handles. Click empty space to deselect.
- Move images by dragging.
- Resize images with their aspect ratio locked.
- Rotate images using handles, with snapping at 0°, 90°, 180°, and 270°.
- Remove selected images with Delete.

Calling edit mode gives the editor keyboard focus so local keyboard commands work. An active file drag must continue uninterrupted; the editor can take focus after the drop.

Losing focus alone does not end edit mode. The user can open or use a non-maximized Explorer window to find and add images while editing remains open. Activating or maximizing an ordinary application window that covers the desktop ends edit mode.

Leaving edit mode returns the board to click-through display and removes the editing overlay, hint, and handles.

## Adding images

Add images through drag-and-drop or **Add Image…** in the tray menu. There is no separate import workflow.

## Board persistence

The board survives application restarts. Board information is stored in a JSON file under the user's local application data folder. Imported images are copied locally so the board does not depend on the originals remaining available.

Adding or deleting an image and completing a move, resize, or rotation automatically saves the board. Saved order and geometry are restored on launch. Failed saves must be reported without replacing the previous saved board; unreadable saved data must not be silently discarded.

## Safety

- Closing or crashing must leave the wallpaper, icons, taskbar, and keyboard state unchanged.
- A failed load must remain invisible.
- Ordinary application windows must always stay above the board.
- The board must remain click-through outside edit mode.
- The keyboard shortcut must not produce an unintended system action.

## Omitted from the current design

- The requirement that editing never takes focus.
- Configurable shortcuts and hold-to-edit mode.
- Image context menus, including copy, save as, and reset aspect ratio.
- Settings window, launch at login, open data folder, and version display.
- Text notes, multiple boards, multiple monitors, and cloud sync.
