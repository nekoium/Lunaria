# Floating board design

Issue: https://github.com/nekoium/Lunaria/issues/12

## Intent

Bring the existing board above applications to check images while continuing work underneath. Placement and editing are independent.

## Behavior

- Ctrl+Alt+Shift+L toggles desktop/top placement without changing editing state.
- Ctrl+Alt+L toggles normal/edit mode without changing placement.
- Normal mode passes mouse input through and does not take focus. Leaving top editing returns focus to the previously active application if Lunaria still has focus and that window exists.
- Top mode uses native Windows always-on-top ordering, including above fullscreen applications where Windows permits it. No special taskbar ordering or fullscreen detection is added.
- Top edit mode retains existing image operations, clipboard paste, and edit fade. Maximizing or activating another application does not exit top editing.
- Desktop auto-exit takes precedence over preserving editing: switching from top edit to desktop, or entering desktop editing, immediately exits editing when a visible maximized app is underneath. Non-maximized, hidden, or minimized apps do not trigger auto-exit. Top editing ignores this rule. Board images and geometry are shared across all four states.

## Approaches

Reuse the existing top-level editor for both top states, changing input styles outside editing. This avoids duplicating windows and rendering. Reparenting the passive desktop window would complicate Explorer attachment and alpha rendering. A third dedicated overlay would duplicate the existing top-level surface.

## Scope

No dependencies, settings UI, configurable shortcuts, new board storage format, or saved placement preference. Launch starts in desktop normal mode. Native topmost ordering cannot guarantee visibility over exclusive fullscreen, secure desktops, or other topmost applications.
