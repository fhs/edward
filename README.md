# Overview

Edward is an experimental fork of
[Edwood](https://github.com/rjkroege/edwood) (itself a Go rewrite of acme)
which removes window management. Each window in Edward uses a separate
window in the OS windowing system, leaving window management entirely
to the OS-native window manager. On Xorg/Wayland you can use a tiling
window manager (e.g. i3, sway, dwm, wmii) to get an experience similar
to traditional acme interface.

Removing window management from acme gives the user the freedom to arrange
acme windows however they like, across multiple virtual workspaces.
It allows more windows on limited screen real estate without having to
open multiple instances of acme. It should also simplify the code base.

# Project Status

Currently, Edward is just a proof-of-concept. It is more unstable than
Edwood. You should expect frequent crashes and things not working.

# Screenshot

Edward running in i3:

![Edward in i3](https://i.imgur.com/TzRws0P.png)
