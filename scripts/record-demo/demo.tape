# VHS tape for the vessel demo recording.
# Run via `make demo` (or scripts/record-demo/record.sh), never directly:
# it assumes ./vessel is already built and Apple's container CLI has been
# seeded with demo containers/images/volumes by seed.sh.
#
# https://github.com/charmbracelet/vhs

Output scripts/record-demo/out.mp4

Set Shell "bash"
Set FontSize 14
Set Width 1280
Set Height 640
Set Padding 8
Set Theme "Dracula"
Set TypingSpeed 80ms

Type "./vessel"
Enter
Sleep 1500ms

# Containers view: live list with mixed running/stopped state.
Sleep 4000ms

# Keyboard-driven navigation: cursor moves update the detail pane live.
Down
Sleep 1500ms
Down
Sleep 1500ms
Up
Sleep 1500ms
Up
Sleep 1500ms
Down
Sleep 1500ms
Up
Sleep 1500ms

# Move focus into the detail pane and back (h/l), inspecting a container.
Type "l"
Sleep 2000ms
Type "h"
Sleep 1500ms

# Images view: browse, then inspect a second image.
Type "2"
Sleep 5000ms
Down
Sleep 2000ms
Up
Sleep 2000ms

# Volumes view.
Type "3"
Sleep 5000ms

# Back to Containers.
Type "1"
Sleep 1500ms

# Keybinding help overlay.
Type "?"
Sleep 6000ms
Type "?"
Sleep 800ms

# Filter down to one container regardless of list order, then start/stop it
# live and watch the status column flip in place. nginx (unlike a plain
# "sleep" container) reacts to SIGTERM immediately, so the transition is
# visible within a couple hundred ms.
Type "/"
Type "web"
Enter
Sleep 2000ms
Type "u"
Sleep 3000ms
Type "s"
Sleep 3000ms

# Clear the filter back to the full list.
Type "/"
Backspace 3
Enter
Sleep 1500ms

Type "q"
Sleep 1200ms
