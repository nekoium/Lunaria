# Temporary board hiding

Issue: https://github.com/nekoium/Lunaria/issues/14

Users can take a break from distracting images without quitting or changing their board. This is a small tray and visibility change, authorized for automatic completion.

## Approaches

Use a session-only hidden flag and the existing native window surfaces. Quitting would remove the tray control needed to restore the board. Persisting visibility or adding a timed pause adds behavior outside this request.

## Behavior

The tray has a **Hide Lunaria** item, checked while hidden, without a shortcut. Hiding immediately ends dragging, editing, and fades, then conceals both surfaces. Unchecking restores the previous desktop/floating placement in normal mode. Images and saved transforms remain intact.

Add Image, edit, and placement menu commands are disabled while hidden. Existing shortcuts and tray left-click respect the pause. Recovery and completed background imports cannot reveal the board. Quit stays available. Launch starts visible; hiding is not persisted.

## Constraints

Use the existing Win32 menu and rendering paths with no new dependencies, shortcuts, timers, or storage changes. Test native visibility, menu state, recovery, imports, and content preservation without touching the user's board or taking focus. Physical tray interaction and compositor appearance require manual verification.
