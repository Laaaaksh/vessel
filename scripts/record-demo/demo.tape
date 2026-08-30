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
Set Height 800
Set Padding 10
Set Theme "Dracula"
Set TypingSpeed 80ms

Type "./vessel"
Enter
Sleep 1500ms

# Containers view: live list with mixed running/stopped state.
Sleep 1500ms

# Keyboard-driven navigation: cursor moves update the detail pane live.
Down
Sleep 700ms
Down
Sleep 700ms
Up
Sleep 700ms

# Keybinding help overlay.
Type "?"
Sleep 2200ms
Type "?"
Sleep 500ms

# Switch views: Images, then Volumes, then back to Containers.
Type "2"
Sleep 1600ms
Type "3"
Sleep 1600ms
Type "1"
Sleep 800ms

# Filter down to one container regardless of list order, then start/stop it
# live and watch the status column flip in place. nginx (unlike a plain
# "sleep" container) reacts to SIGTERM immediately, so the transition is
# visible within a couple hundred ms.
Type "/"
Type "web"
Enter
Sleep 1000ms
Type "u"
Sleep 1800ms
Type "s"
Sleep 1800ms

# Clear the filter back to the full list.
Type "/"
Backspace 3
Enter
Sleep 1000ms

Type "q"
Sleep 800ms
