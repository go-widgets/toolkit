# go-widgets/toolkit

[![CI](https://github.com/go-widgets/toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/toolkit/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/go-widgets/toolkit?display_name=tag&sort=semver&color=0d9488)](https://github.com/go-widgets/toolkit/releases)
[![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-toolkit-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-widgets/toolkit)
![coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)
![go](https://img.shields.io/badge/Go-1.26.4%2B-00ADD8?logo=go&logoColor=white)
[![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)](./LICENSE)

Pure-Go widget toolkit that renders into an RGBA byte buffer. Zero
JS / DOM / canvas dependency — every widget composes pixels into a
caller-supplied `[]byte`, so the toolkit runs identically in
`GOOS=js GOARCH=wasm` (browser SharedArrayBuffer clients), on any
native target Go ships (native canvas backends, image files), or
in headless tests (screenshot-hash regressions).

## Goals

- **One toolkit per app**: consumers stop reinventing buttons,
  scrollbars, text fields. They import
  `github.com/go-widgets/toolkit` and compose.
- **Coherent theming**: a single `Theme` value cascades through every
  widget — change a colour at the top, see it everywhere.
  `LoadGTKTheme(css)` parses libadwaita / GTK3 `@define-color`
  declarations into a `Theme`, so any GTK-desktop palette (Adwaita,
  Juno, WhiteSur, Solarized …) drives the widget ink colours.
- **Pure Go + CGO=0**: no C toolchain, no shared libraries. Builds
  for every Go target including `GOOS=js GOARCH=wasm`.
- **A11y bridge**: core widgets implement an opt-in `Accessible`
  interface (`A11y() A11yInfo`) so a host can `CollectA11y` and
  publish them to screen readers without rewriting every widget.
  `A11yInfo` carries role, name, value, range and the textbox states
  `Focused`, `Disabled`, `ReadOnly` and `Multiline` (WAI-ARIA 1.2
  aria-disabled / aria-readonly / aria-multiline; `Focused` is read from
  the widget's own focus state). `WalkA11y` returns each element with its
  bounds and a stable `Key` (a `/`-separated path of `@<id>` or
  `<Type>#<n>` segments) so a host can diff trees between frames: a
  widget that has a model identity implements `A11yID() string`
  (`A11yIdentified`), and keys survive scrolling, repaint and the
  insertion of siblings. `TextView`, `CodeEditor` and `Entry` take an
  `AccessibleName`.

## Non-goals

- **Not a CodeMirror replacement.** Complex web-grade editors are
  best embedded via an iframe overlay at the host level.
- **Not GTK.** No CSS engine, no SVG renderer, no full BiDi/IME. A
  broad primitive set — buttons, inputs, containers, feedback,
  overlays, structural rows, semantic banners, data displays,
  dashboard/data widgets, charts and desktop-shell pieces — around
  30 kLoC of widget code with roughly 40 kLoC of tests.

## Status

**Broad GTK 4 / DaisyUI / desktop-shell coverage.** The
widget set has grown well past the early parity passes into a full
application toolkit: widget types across inputs, containers,
overlays, structural rows, semantic banners, a dashboard/data suite,
a chart family, desktop-shell pieces (status area, wallpaper,
command palette), and a loading-screen **Skeleton** family. A
declarative `Container` + swappable `Layout` model (`FitLayout`,
`BoxLayout`, `BorderLayout`, `CardLayout`, `FlowLayout`) sits under
the convenience containers (`HBox`/`VBox`/`Grid`/`Frame`/`Dock`/
`Border`).

**HiDPI.** `SetMetricScale(2)` scales every pixel metric and the text
face. As of v0.355.0 that includes a `Frame`'s title bar, border and
padding (`Frame.Padding` is in logical pixels); at scale 1 nothing changes.

**159** exported widget types + 10 stock icons, **68 kLoC** of widget
code across 233 files, 100% statement coverage.

⛔ Those numbers are measured, not remembered, and the headline no
longer carries a version. It said **v0.109.0**, "~150 widget types"
and "~30 kLoC" while the repository stood at **v0.323.0** with 159
types and 68 kLoC — a README is read at whatever tag it ships in, so
a version written into its prose lies in every later one. The current
version is whatever `git tag` says; the rest re-measures in one
command each:

```sh
grep -hA3 '^type [A-Z][A-Za-z0-9]* struct {' $(ls *.go|grep -v _test)   | grep -cE '^[[:space:]]+Base$'          # exported widget types
cat $(ls *.go|grep -v _test) | wc -l       # lines of widget code
grep -lE '^[[:space:]]+MultiSelect bool' $(ls *.go|grep -v _test)
```

**Multi-row selection** is carried by exactly three widgets —
`ListBox`, `Table` and `TreeView` — and in all three it is reachable
from the **mouse and the keyboard**: Ctrl/⌘-click adds a row,
Shift-click takes a run, Shift + any movement key extends from a
fixed anchor (and shrinks back), Ctrl/⌘+A selects all, Ctrl/⌘+Space
toggles the cursor row without activating it. ⌘ is honoured as well
as Ctrl, because on macOS Ctrl-click is the secondary click and
cannot also mean "add this row".
Pure Go, no CGO, stdlib only. Builds for `GOOS=js GOARCH=wasm` and
every native target Go ships.

| Family       | Widgets                                                            |
| ------------ | ------------------------------------------------------------------ |
| Base         | `Widget`, `Base`, `Rect`, `Event` (+ IME composition), `Theme`, `RGBA`, `Accessible`/`A11yInfo`, `Focusable`, `Measurer` |
| Text         | bitmap 5x7 + anti-aliased `NewTrueTypeFont`, one-call AA default `UseOpenTypeText`, global `SetFont` + per-widget `Base.Font`, `DrawText`, `TextWidth`, `Label` |
| Action       | `Button`, `ToggleButton`, `CheckButton`, `RadioButton` + `RadioGroup`, `Switch`, `SplitButton`, `IconButton`, `CycleButton`, `Chip`, `SegmentedBar` |
| History      | `UndoStack` (an application's command history: `Do`/`Push` steps that undo and redo themselves, labels, a limit, re-entrancy guarded; `Bind` keeps an Undo and a Redo `Action` enabled exactly when there is something to undo or redo); a `TextView`'s own history as a value (`History`/`SetHistory`, so one view showing several documents keeps each one's undo apart), `CanUndo`/`CanRedo`, `OnHistory`, `ReplaceText` (an undoable whole-buffer replace) |
| Input        | `Entry`, `TextView` (and so `CodeEditor`) + `Selection` + IME preview + `textinput.Client` (`github.com/go-widgets/textinput`: UTF-16 offsets, marked-text composition, `ReplaceInput`, `InputRect`) + optional `Highlighter` (syntax spans) + `ShowLineNumbers` gutter + size-to-content (`NoVerticalScroll` lays out every line, keeps the horizontal bar; `PreferredHeight()` is an `Observable[int]` following lines, font and width), `SpinButton`, `Scale`, `RangeSlider`, `SearchEntry`, `TagField`, `ComboBox`, `FormField`, `Validate`/`Rule` |
| Terminal     | `Terminal` (an xterm-subset emulator over the pure-Go `vt.Screen` model: cursor movement, erase, insert/delete, scroll regions, SGR 16/256/truecolor, bounded scrollback, selection + copy/paste, key encoding, `ClaimsKey` for hosts with global shortcuts, `SetFont` + `SetCellSize` (pin a cell box such as 16x16 and an OpenType face; the grid is refitted and `OnResize` fires, like `TerminalView.SetCellSize`); safe for untrusted output — no reply channel, every OSC/DCS/APC string discarded, so no clipboard write, no links, no title, no reports; parser fuzzed), `TerminalView` (Cols×Rows `TermCell` grid, block cursor, wrap/scroll, per-cell FG/BG) |
| Selection    | `ListBox`, `TreeView`, `DropDown`                                  |
| Containers   | `Container` + swappable `Layout` (`FitLayout`, `BoxLayout`, `BorderLayout`, `CardLayout`, `FlowLayout`); convenience wrappers `HBox`, `VBox`, `Grid`, `Frame`, `Dock`, `Border`, `Stack`, `Overlay`, `Paned`, `Splitter` (N panes with draggable, focusable dividers after the WAI-ARIA window-splitter pattern: fixed panes sized by an `mvvm.Observable`, flexible ones by weight, min/max, collapse on double-click, Enter or a drag to the minimum, arrows/Home/End/Enter, 3:1 divider and focus ring, resize cursor; panes close and reopen through `Open(i)`, an `mvvm.Observable`, the last `Essential` pane refusing, with `ClosablePane` for a titled header and a close button), `Expander`, `Accordion`; declarative `Node` builder + `ViewController` (`LookupAs`) |
| Tabs         | `Notebook`, `ViewSwitcher`, `Carousel`                             |
| Scroll       | `ScrollView`, `Scrollbar`                                          |
| Feedback     | `ProgressBar`, `ProgressCircle`, `LevelBar`, `Spinner`, `Image`, `Tooltip`, `Notification`, `Toast`, `Banner`, `Alert`, `Badge`, `LoadMask`, `Backdrop`, `FocusRing` |
| Loading      | `Skeleton` (Text/Rect/Circle/Avatar/Block kinds, shimmer via `SetPhase`), `SkeletonGroup`, `NewSkeletonCard`, `NewPageSkeleton` |
| Structure    | `Card`, `HeaderBar`, `ActionRow`, `Breadcrumbs`, `Steps`, `Avatar`, `Rating`, `Stat`, `Kbd`, `ChatBubble`, `Diff`, `Pagination`, `DropZone` |
| Navigation   | `Menu` + `MenuItem.Shortcut`, `MenuBar` + `Alt+letter`, `ContextMenu`, `Popover`, `CommandPalette`, `Dialog`, `MessageDialog`, `Wizard` |
| Window       | `Window`, `WindowDecoration`, `DecoButton` (client-side decorations) |
| Bars         | `Toolbar`, `PagingToolbar`, `Statusbar`, `Ribbon` (Office-style tabs of groups of large/small buttons, toggles, split buttons, drop-downs, a gallery and a size grid (`RibbonGrid`, the Table picker: the pointer or the arrow keys light the cells from the corner, a caption says "3 × 2", `OnGrid(cols, rows)` runs); scales down by group priority, collapses, keytips, tooltips with shortcuts; every control is an `Action`, state is `mvvm.Observable`), 10 stock `DrawIcon*` helpers |
| Composite    | `FileChooser`, `FileTree` (an Explorer over paths: rename in place, new file/folder, delete with confirmation, drag to move, drop from outside, context menu, APG treeview keys, type-ahead, and a per-entry `FileStatus` for a versioned working tree — modified, added, deleted, untracked, conflicted — marked `M`/`A`/`D`/`?`/`!` and spoken in `A11y`; `SetRepository(vcs.Repository)` fills the marks from a `go-widgets/vcs` repository (the git driver is the separate module `go-widgets/vcs/git`), marks a folder `M` when anything under it has a mark and `!` when anything under it is conflicted, and refreshes whenever `Entries` changes (off the UI goroutine when the host sets `PostUI`, newest refresh wins), so a repository view needs no hand-rolled tree), `ForgePane` (the pane of an editor that works against a code forge: forge, repository, branch and folder, Connect or sign in or a frame slot, Load, Propose changes…, Check for changes, Take theirs, Open the pull request, and a wrapped note. It is the view only: everything it shows and does is behind the `ForgeModel` interface of `mvvm` observables and commands, so the toolkit links no forge code. Three layouts by `ForgeMode`: client, frame — `FrameRect` is where the host lays a credential frame — and server; its words are the `forge.*` keys of `MessageKeys`. A model that also implements the optional `ForgeCommitter` (commit message, author name and email, `Commit`, `Push`) adds those controls in every layout, for a git-transport model with no forge REST API; `Repo` is shown as given, so it may be a full remote URL. A nil command removes its control and closes the row up (a capability the model lacks), a command that cannot run now stays greyed (a state); the token field is masked and never copied, cut or exposed to accessibility in clear), `ColorChooser`, `ColorPicker`, `FontChooser`, `FontPicker` (a font by name: a family drop-down, optionally a weight and a pixel size, reporting a `FontSpec`; a family the list lacks is added, never replaced), `ContrastBadge` (the WCAG 2.x ratio of an ink on a ground, as "4.52:1 AA" / "Fail", with `ContrastGrade` for the AA/AAA thresholds of normal and large text), `Calendar`, `DatePicker`, `DateRangePicker`, `TimePicker`, `MarkdownView`, `MarkdownEditor` |
| Data suite   | `Table` (cell edit, frozen columns, group rows, aggregates, row-expanders, `Framed` outer edge and `RowLines` in `theme.Border`), `TreeTable`, `PropertyGrid`, `Inspector` (labelled rows under section headings, each with the editor of its kind: a colour through `ColorPicker`, a font through `FontPicker`, a number with a unit, an enum, text; every value a style-sheet string, and a value an editor cannot show is kept and drawn as text, never rewritten), `Kanban` (drag cards), `Gantt` (drag/resize bars), `Agenda`/`AgendaCalendar` event calendar (week/month/quarter/year, colour-coded calendars, sidebar, inline editor) |
| Text compare | `DiffView` (two texts compared side by side or inline: the changed words of a changed line marked (`WordChanges`), long lines wrapped with both sides level so nothing is cut, line numbers of both, unchanged runs folded around `Context` lines, an overview ruler of the changes, ↑/↓/PageUp/PageDown/Home/End, F7/Shift+F7 or n/p to the next/previous change (outlined); colours from the theme's darkness, text at 4.5:1 and markers at 3:1 in both default themes, +/- marks so it reads without colour); `textdiff` (a pure package, no widget): the shortest line edit script (Myers O(ND), linear space, a work budget past which a block is replaced whole), `Hunks` with context, `Unified` (diff -u), `SideBySide` rows; fuzzed |
| Document     | `PagedView` (a page viewer over pre-rendered bitmaps — or over sizes alone, with a host drawing the pages through `PageRect`: `SetPageSizes` lays out, cards and all, what it will not blit; `SetPageBitmap` fills one page in afterwards, so a viewer of a five-hundred-page document lays out all of them while holding bitmaps for a window around the reader, and a coarse stand-in can be replaced by a sharp rendering without the page changing shape): a continuous stack of page cards or one centred page with prev/next, its own toolbar, uniform zoom, keyboard and wheel navigation (a wheel notch past a page's edge flips, so a tall page scrolls within itself first), and two **sticky** fits — `PagedFitWidth` and `PagedFitPage` — re-derived on every resize, page change and new document, because a page fitted once and then resized is a page that no longer fits; a manual zoom turns the fit off. `PageAt` and `ScrollToPage` are an exact inverse pair, which is what turns a click on a page into a source position and a caret move back into a scroll. `TextSelection` is the selection over a host's text runs (press, drag, `SelectedText`, `CopySelection`): `SelectedRects` answers *where* the selection is, as the rectangles `Draw` paints — by construction, since `Draw` is a loop over it, so a host that writes a PDF highlight annotation over the selected words marks exactly what the reader sees lit, instead of re-deriving the covered spans and drifting |
| Charts       | `LineChart`, `BarChart`, `PieChart`, `AreaChart`, `ScatterChart`, `RadarChart`, `Gauge`, `Sparkline` |
| Graphs       | `GraphView` over a `GraphViewModel` (clusters, nodes, edges, selection and filter as `mvvm` lists and observables): nodes in cluster boxes on a stable grid that never moves on an edge update, edge width by weight, denied edges dashed in the error colour, category colours (Okabe-Ito by default), legend, click and arrow-key selection, filter dimming |
| Shell        | `StatusIcon`, `StatusArea`, `Wallpaper`, `Thumbnail`               |
| Motion       | `Easing`/`Tween` (`Linear`, `EaseIn*/Out*/InOut*`), `GestureRecognizer` |
| Theming      | `LoadGTKTheme(css)` (GTK3 + libadwaita @define-color → Theme)      |

### Several people in one buffer: where a keystroke lands

`collabui.CollabText` binds a `CodeEditor` to one text part of a
[`go-crdt/collab`](https://github.com/go-crdt/collab) session. The merge is the
CRDT's and the binding adds none of it — but **where** an edit is applied is the
binding's, and the two are not the same question.

A peer's edit merges into this replica the moment it arrives off the wire. The
editor is repainted later, when the host runs the closure the binding handed it.
In between, the screen shows one text and the replica holds another, and a
keystroke in that window was aimed at the screen. Replayed at its raw offset it
lands somewhere else — and both replicas agree on the result, which is why it
does not look like a bug. It was one:

| the person did | and got, before | now |
|---|---|---|
| typed `Y` at the end of the `abc` on screen, while `Xabc` had merged | `XabYc` | `XabcY` |
| deleted the `bc` on screen, while a peer's `X` had landed between them | `acd` — **the peer's `X` destroyed** | `aXd` |

So a local edit is carried across that gap before it is applied — the inclusion
transformation of Ellis & Gibbs (SIGMOD '89), on the one side Jupiter
(Nichols et al., UIST '95) transforms on: our own operation against what has
already merged. Two rules, both in `rebase.go`:

- An offset moves with what landed before it, stays put at it, and goes the
  same distance into what replaced it — never to the start of the span.
- **A deletion takes the characters the person could see and not the ones they
  could not.** A peer's insertion inside the deleted range is not part of what
  was selected, so the range is deleted around it. Placement is a guess; a
  character that was on somebody's screen a moment ago is not.

Shown in `collabui` by two real sessions over a loopback WebSocket, driven into
the losing interleaving on purpose rather than raced for, and fuzzed.

### Binary size: a program links what it uses

The toolkit is one large package, and that is fine for the linker: Go's
dead-code elimination keeps only what is reachable from `main.main` **and
from the init tasks of every imported package**
(`cmd/link/internal/ld/deadcode.go`). Widgets a program never touches are
dropped. What is not dropped is anything a package-level initializer or an
imported package's `init` reaches — so the toolkit keeps its own initializers
empty of work and avoids importing packages whose `init` drags code in:

- `Material` and `Thumbnail` use the codec-free `github.com/go-images/images/ops`,
  not `github.com/go-images/images`, whose decoders register themselves in `init`
  (about 1.7 MB of WebAssembly code).
- `CollabText` moved to **`github.com/go-widgets/toolkit/collabui`**: it needs a
  live `go-crdt/collab` session, whose transports pull `net/http` and `crypto/tls`.
  `CollabSheet` and `IsoCRDTDocument` need only `go-crdt/crdt` and stay here.
- The e-mail `Rule`'s regexp, the built-in iso icon registry and the
  `RichEditorToolbar` button table are built on first use, not by initializers.

Measured with `go build -ldflags=-dumpdep` on a `GOOS=js GOARCH=wasm` program
that draws one `Label` (`-ldflags="-s -w"`, gzip -9):

| | raw | gzip |
|---|---|---|
| v0.343.1 | 10.49 MB | 2.88 MB |
| v0.345.0 (this list) |  7.48 MB | 2.13 MB |
| v0.345.1 (go-opentype WOFF2 inflates without `net/http`) |  5.66 MB | 1.61 MB |

What remains is mostly the runtime and the OpenType stack the text renderer
needs. To see what keeps a symbol in your own program, build with
`-ldflags=-dumpdep` and follow the first-parent chain from the symbol back to
`main.main` or `go:main.inittasks`.

### v0.357 breaking change: `Focused()` is now `HasFocus()` on the editors

`TextView`, `CodeEditor` (through its embedded `TextView`), `RichEditor` and
`AddressBar` used to return their focus `*mvvm.Observable[bool]` from
`Focused()`, which collided with `Focusable.Focused() bool`. The accessor is
now `HasFocus()` (`ed.Focused().Get()` becomes `ed.HasFocus().Get()`), and all
four implement `Focusable` — `Focused()` reads and `SetFocused(bool)` writes
that same observable — so Tab traversal in a container now reaches them.

### v0.6 breaking change

`Widget.Draw` signature moved from

```go
Draw(surface []byte, surfaceW int, theme *Theme)
```

to

```go
Draw(p painter.Painter, theme *Theme)
```

where [`painter.Painter`](https://github.com/go-widgets/painter) is
a 5-primitive interface (`FillRect`, `StrokeRect`, `PutPixel`,
`Text`, `Size`). Existing callers that had a `[]byte` + width
migrate one line:

```go
// before v0.6:
wg.Draw(surface, w, theme)

// v0.6+:
p := painter.NewPixelPainter(surface, w, h)
wg.Draw(p, theme)
```

The `[]byte` is still writable + owned by the caller — the
`PixelPainter` just wraps it so the primitives translate to writes.
All other widget APIs (`Bounds`, `SetBounds`, `HitTest`, `OnEvent`,
`NewButton`, …) are unchanged.

`toolkit.Rect` + `toolkit.RGBA` became type aliases of
`painter.Rect` + `painter.RGBA`, so cross-package assignments work
transparently.

The 10 `DrawIcon*` helpers also lost their `(surface, surfaceW)`
prefix; new signature is `DrawIconX(p painter.Painter, r Rect, ink RGBA)`.

### Earlier releases

- **v0.9** — 8 widgets: SplitButton (button + attached dropdown arrow),
  IconButton (toolbar-icon variant), Stat (KPI card with trend
  indicator), Timeline (vertical event log), DropZone (dashed file
  drop target), Chip (removable tag), FormField (Label + Child +
  Help/Error), ProgressCircle (approximated circular progress).
- **Skeleton (loading placeholders)** — shape kinds Text (rounded
  bars, tunable LineH/LineGap/LastFrac/Radius), Rect (rounded media
  block), Circle (avatar), plus the original Avatar/Block swap-parity
  kinds. Optional diagonal shimmer band (a lighter tint over
  Theme.SurfaceAlt, working in light + dark) driven by `SetPhase(t)`
  advanced per frame. `SkeletonGroup` composes primitives; presets
  `NewSkeletonCard(bounds)` (avatar + 2-line header + media block) and
  `NewPageSkeleton(bounds)` (top bar + paragraph groups + image blocks,
  the webengine browser client's loading screen).
- **v0.8** — 12 widgets: Avatar, Skeleton (Text/Avatar/Block kinds),
  Rating, Toast (transient bottom-of-screen), Banner (persistent
  full-width), Popover (Visible container for a Child), ActionRow
  (libadwaita Title/Subtitle/Prefix/Suffix), ViewSwitcher (segmented
  tab picker), ChatBubble (User/Other bubble), SearchEntry (Entry
  with prefix + clear), Diff (Context/Added/Removed colored lines),
  Pagination (prev/numbers/next with disabled ink).
- **v0.7** — 9 widgets: Switch (iOS-style toggle distinct from
  ToggleButton), Badge (auto-sizing pill), Kbd (keyboard-shortcut
  chip), Alert (Info/Success/Warning/Error semantic banner), Card
  (Title/Body/Footer three-zone), Breadcrumbs (chevron path),
  Steps (numbered indicator with connector), HeaderBar (Start/Title/
  Subtitle/End GTK CSD), Table (Columns/Rows/Selected data grid).
- **v0.6** — Painter back-end abstraction. Every widget's `Draw`
  now takes a [`painter.Painter`](https://github.com/go-widgets/painter)
  instead of a fixed `[]byte` + stride pair, so the same widget code
  renders into a pixel buffer (WUI browser canvas, GUI native window,
  image file), a terminal cell grid (TUI), or an SVG stream.
- **v0.5** — Toolbar / Statusbar / FileChooser / ColorChooser /
  Calendar, Selection on TextView, `LoadGTKTheme(css)`, 10 stock
  icon helpers.
- **v0.4** — 34 widgets. Toolbar / Statusbar, FileChooser /
  ColorChooser / Calendar, Selection model on TextView,
  `LoadGTKTheme(css)`.
- **v0.3** — 28 widgets. TextView (multi-line editor), Menu/MenuBar,
  Dialog/MessageDialog, Tooltip, DropDown, TreeView.
- **v0.2** — 22 widgets. Layout containers (HBox/VBox/Grid/Frame),
  scroll (ScrollView/ListBox), input (Entry/Check/Radio/Toggle),
  structural (Stack/Notebook/Paned/Expander), feedback
  (ProgressBar/LevelBar/Scale/SpinButton/Image/Spinner), bitmap font.
- **v0.1** — scaffolding. Widget interface, Theme value, Button +
  Label, primitive event dispatch.

### Next (v1.0 sketch)

Widget coverage is now materially complete versus GTK 4 + DaisyUI 4.
The remaining pre-1.0 work is around the *edges* of the widget
model, not the widget catalogue:

- ~~**Font family plumbing**~~ — **done (v0.20)**: a `Font` interface
  (`Advance`, `Height`, `Draw`) + a scalable built-in bitmap font.
  `SetFont(NewBitmapFont(2))` doubles all text ("retina"), and every
  widget re-lays-out because metrics are now read at draw time.
  **Breaking:** `GlyphHeight` / `GlyphAdvance` and the metric-derived
  dimensions (`CardHeaderH`, `DatePickerFieldH`, `DiffLineH`,
  `FormFieldLabelH`, `TimelineEventH`, `CardFooterH`) are now
  **functions** — append `()` at call sites when upgrading.
  Extended since with anti-aliased vector text
  (`NewTrueTypeFont(ttf, px)`) and, from **v0.34**, a **per-widget
  font**: set `widget.Font` (the exported `Base.Font` field) to give one
  widget its own face/size — e.g. a 10px badge next to a 22px title —
  while every other widget keeps the global `SetFont` font. A widget
  with no `Font` set (the default, `nil`) renders exactly as before.
  Since **v0.77**, `UseOpenTypeText()` turns on anti-aliased, shaped
  text for the whole UI in **one call** — it installs the bundled
  Atkinson Hyperlegible face (from `github.com/go-opentype/fonts`, no
  extra imports, `js/wasm`-safe) as the active font, so consumers get
  crisp AA/multi-script glyphs without sourcing a face of their own.
  Use `UseOpenTypeTextSize(px)` for a specific size, or
  `DefaultOpenTypeFont(px)` to compose the default face into a
  `NewFallbackFont` chain (add a CJK/Arabic face) before `SetFont`. The
  5x7 bitmap stays the compiled-in default (so existing pixel/metric
  tests keep their geometry); AA is an explicit opt-in, and `SetFont(nil)`
  restores the bitmap.
- ~~**First-class drag-and-drop event kinds**~~ — **done (v0.16)**:
  `EventDragStart` / `EventDragMove` / `EventDragLeave` / `EventDrop`,
  plus a `DragSource` / `DropTarget` interface pair and
  `SplitDropPayload` / `JoinDropPayload` for multi-item payloads.
  DropZone dropped its synthetic-`EventChar` seam and now drives its
  hover cue + `OnDrop` from the formal lifecycle as a `DropTarget`.
- ~~**Context menu helper**~~ — **done (v0.17)**: `ContextMenu`
  wraps a `Menu` as a right-click popup — `Popup(x, y)` shows it at
  the cursor, it auto-sizes to its items, clamps itself inside the
  surface, and dismisses on outside-click.
- ~~**Overlay layout container**~~ — **done (v0.18)**: `Overlay`
  stacks z-ordered `Layers` above a primary `Content` child, so
  Popover / Toast / Notification / Tooltip / ContextMenu float
  without hosts arranging z-order. Events route top-down; `Modal`
  makes a miss swallow (backdrop) instead of falling through.
- ~~**A11y bridge**~~ — **done (v0.19)**: an opt-in `Accessible`
  interface (`A11y() A11yInfo` — role + name + value) implemented by
  the core widgets (Button/Label/Entry/CheckButton/RadioButton/
  Switch/Scale), plus `CollectA11y([]Widget)` so a host republishes
  them to the platform layer (WAI-ARIA on wasm, TTY metadata on tui).

## Architecture

```text
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
