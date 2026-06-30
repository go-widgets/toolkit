# wasmdesk/toolkit

Pure-Go widget toolkit for [wasmdesk](https://github.com/wasmdesk) native
apps. Draws into an RGBA byte buffer (the SAB-backed framebuffer wasmbox
clients write to) and dispatches mouse / keyboard events. No JS deps,
no DOM, no canvas — all rendering is per-pixel composition into the
caller's buffer.

## Goals

- **One toolkit per app**: clients/{terminal,files,dock,code,...}
  stop reinventing buttons, scrollbars, text fields. They import
  `github.com/wasmdesk/toolkit` and compose.
- **Coherent theming**: a single `Theme` value cascades through every
  widget — change a colour at the top, see it everywhere.
- **Pure Go + CGO=0**: same constraints as the rest of the wasmdesk
  ecosystem. Builds for `GOOS=js GOARCH=wasm` and every native target
  the test suite runs on.
- **A11y-ready scaffolding**: widgets carry a `Role` + `Label` so a
  future a11y bridge can publish them to screen readers without
  rewriting every widget.

## Non-goals

- **Not a CodeMirror replacement.** Complex web-grade editors stay
  embedded via wasmbox's `dom_window` protocol (an iframe overlay).
- **Not GTK.** No CSS engine, no SVG renderer, no full BiDi/IME. A
  light primitive set: Button, Label, TextInput, ListBox, ScrollView,
  HBox, VBox, Splitter, TabBar, MenuBar. Around 5-10k LoC.

## Status

v0.1 — scaffolding. Widget interface, Theme value, Button + Label
implementations, primitive event dispatch. Tests at 100%, CI gated.

Next:
- TextInput + cursor model
- ScrollView + ListBox
- HBox / VBox layout
- Splitter (for the editor's gutter / panel split)
- Themes ported from clients/dock/internal/theme

## Architecture

```
+----------------------+
| Theme                |  Palette + metrics + font ref
+----------------------+
           |
+----------------------+    +-------------------+
| Widget interface     |    | Event             |
|   Draw(rgba, theme)  |<---|   Kind: click/key |
|   HitTest(x, y)      |    |   X,Y / Code      |
|   OnEvent(ev)        |    +-------------------+
+----------------------+
           |
   +-------+-------+-------+-------+
   |               |       |       |
+--+---+        +--+--+  +-+--+  +-+--+
|Button|        |Label|  |HBox|  | ...|
+------+        +-----+  +----+  +----+
```

## License

BSD 3-Clause. See LICENSE.
