# Plan: move pain to Bubble Tea v2, then answer questions on key release

Written 2026-11-16 for a fresh agent with no context. Read it all before
starting. Work in the root of the pain repository.

## Goal

1. Move pain from Bubble Tea v1 (`github.com/charmbracelet/bubbletea`
   v1.3.10), Lip Gloss v1 and Bubbles v0.21 to their v2 modules, with **no
   change in behavior**.
2. Then use what v2 adds: the kitty keyboard protocol, which reports when a
   key is **released**. The hold-to-answer control in `ui/hold` should use
   real release events when the terminal sends them, and keep its current
   guessing from auto-repeat when it does not.

### Why

At a checkpoint, the reader answers a multiple-choice question by holding a
key for one second. The selected choice underlines as the key is held
(`ui/hold.Underline`), and a note at its right says `hold to answer`,
`keep holding` or `hold the key to answer`. Terminals in legacy mode never say
when a key is let go, so `ui/hold.Hold` guesses from auto-repeat: the press, a
pause, then repeats. It learns the pace and notices a release about 100
milliseconds after the last repeat, and a tap only after the first-repeat
wait (up to 700 milliseconds). With release events, letting go is seen at
once, and a hold no longer needs key repeat to be turned on.

Bubble Tea v1 cannot read the kitty keyboard protocol. v2 can:
`tea.KeyReleaseMsg`, `tea.Key.IsRepeat`, and
`View.KeyboardEnhancements.ReportEventTypes`. This is a new project, so it
should be on v2 anyway.

### Where it will and will not help

- Ghostty (the user's terminal), kitty, WezTerm and foot support the
  protocol. Releases will arrive there.
- **tmux 3.6a**, which the user often runs pain inside, does not pass the
  protocol through. Inside tmux there are no release events, and `ui/hold`
  must fall back to guessing. The user accepted that pain works better
  outside tmux.

## Before you start

- **Working tree.** Run `git status`. If anything is uncommitted, **ask the
  user** whether to commit it first. Do not mix it into the migration commits.
  Do not discard it.
- **The user's rules** for commits, messages and pushing load on their own
  (their `CLAUDE.md` and the project memory): follow them. Commit messages
  follow the repository style: one plain sentence, no prefix, such as "Hide
  the cursor when not typing and light the sentence being read".
- **Read first:** `docs/reference.md`, especially "How it works", "Packages",
  "Build and check" and "Conventions". The code comments explain the why;
  keep that style (short comments on why, not what).
- **Checks:** `mise run check` (gofmt, go vet, go test). `mise run stories`
  opens the UI component demos (`ui/cmd/stories`).

## Versions verified on 2026-11-16

These were fetched into a scratch module and their docs read:

| Module | Version |
|---|---|
| `charm.land/bubbletea/v2` | v2.1.1 |
| `charm.land/lipgloss/v2` | v2.1.1 |
| `charm.land/bubbles/v2` | v2.2.1 |
| `github.com/charmbracelet/x/ansi` (pulled in by them) | v0.11.9 |

The official upgrade guide ships in the module:
`$(go env GOMODCACHE)/charm.land/bubbletea/v2@v2.1.1/UPGRADE_GUIDE_V2.md`.
Read it. It is short and accurate. Use `go doc` for anything else rather than
memory: v2 changed a lot, and older blog posts describe betas.

## Step 0: spike the risky parts first (throwaway, not committed)

Some parts of pain write to the terminal in unusual ways. The v2 renderer is
cell-based (it parses `View` content into a cell grid and redraws by diff),
so these might not survive. Check them in a small program under the
scratchpad **before** touching pain. If one fails and you cannot find a fix,
stop and report to the user. Do not drop a feature to get the migration
through.

1. **Images.** `internal/picture` shows images in Kitty-graphics terminals by
   sending the image once with `picture.Out.Write` (straight to `os.Stdout`,
   outside Bubble Tea), then putting placeholder characters in the view:
   `U+10EEEE` followed by two combining diacritics that encode row and
   column, colored `38;2;r;g;b` to encode the image id
   (`picture.go: placeholders`, `transmit`, `diacritics.go`). Check that:
   - the v2 renderer keeps each placeholder with its two diacritics as one
     cell, at width 1, with its color. Render the placeholders of a real
     image in a v2 program in Ghostty, outside tmux, and look at it;
   - the transmission still lands. A direct write to `os.Stdout` may now
     race with the renderer. `tea.Raw(...)` (a `Cmd` that writes bytes
     unprocessed) is the v2 way, but `picture.Lines` is called from rendering
     code that has no way to return a `Cmd`. Find the least invasive fix:
     the direct write may still be fine, or `Lines` could queue the bytes and
     the reader return them as a `tea.Raw` from `Update`;
   - the half-block fallback (`blockLines`, used in tmux without passthrough)
     still looks right.
2. **Raw styling escapes in view strings.** pain puts its own Select Graphic
   Rendition (SGR) codes in view content, which Lip Gloss cannot express:
   underline `\x1b[4m` / `\x1b[24m` and strikethrough around faint text
   (`theme.Cell`), dotted underline `\x1b[4:4m` (`theme.Dotted`, links and
   the hold track), and colored underline `\x1b[58;5;Nm` / `\x1b[59m`
   (`ui/hold.Underline`). Check that the v2 renderer keeps all of them,
   including the colon form `4:4` and underline color 58/59. If it drops the
   underline color or the style, the underline in the quiz loses its yellow.
   Fix by using the renderer's own underline style and color support if Lip
   Gloss v2 has it (look for `UnderlineStyle` / `UnderlineColor` on
   `lipgloss.Style`), and note it in the report.
3. **Light/dark switch.** `theme.Watch` writes `\x1b[?2031h` so the terminal
   reports light/dark switches as `\x1b[?997;1n` (dark) or `;2n` (light), and
   `theme.Filter` catches them from v1's unknown-sequence message by
   reflection. In v2 the input decoder (`github.com/charmbracelet/ultraviolet`,
   `decoder.go`, case 997) turns them into `uv.DarkColorSchemeEvent` /
   `uv.LightColorSchemeEvent`. Check that they reach `Update` (`tea.Msg` is
   `uv.Event`). If they do, `Filter` and its reflection go away. Check also
   whether v2 turns mode 2031 on itself; if not, keep `Watch`/`Unwatch`.
4. **Key release.** In a v2 program with
   `View.KeyboardEnhancements.ReportEventTypes = true`, print every key
   message, then press, hold and release `1`, `enter` and `space`:
   - in Ghostty outside tmux: expect `tea.KeyboardEnhancementsMsg` with
     `SupportsEventTypes()` true, presses with `IsRepeat` while held, and one
     `tea.KeyReleaseMsg` at release. Check that the release's `String()`
     equals the press's `String()` for all three keys (`ui/hold` matches keys
     by string). If enter or space need extra flags to report a release (the
     kitty spec says Enter, Tab and Backspace releases need "report all keys
     as escape codes"), find out whether `ReportAllKeysAsEscapeCodes` breaks
     anything else, and prefer leaving enter's release to the fallback over
     a risky flag;
   - inside tmux 3.6a: expect no event types support and no releases. The
     fallback must keep working there.

Write down what you found. It goes into the final report.

## Step 1: migrate to v2 with no behavior change (one commit)

Do it package by package and keep `go build ./...` green as early as you can.
Inventory of v1 usage at the time of writing:

- Files using Charm modules: `main.go`, `internal/app/app.go`,
  `internal/browser/browser.go`, `internal/picture/picture.go` (and test),
  `internal/reader/{ask,askview,check,picker,prefsview,reader}.go` and the
  tests `check_test.go`, `navigation_test.go`, `internal/theme/theme.go`,
  `internal/view/{keybox,render}.go`, `ui/hold/*.go`, `ui/story/story.go`.
- Most used: `tea.Cmd`, `tea.Msg`, `tea.KeyMsg` (16 uses), `tea.Model`,
  `lipgloss.Width` (26), `lipgloss.NewStyle`, `Place`, `JoinVertical`,
  `RoundedBorder`, `lipgloss.Color`, `lipgloss.TerminalColor`,
  `lipgloss.ColorProfile`, `termenv` (theme and picture), `textarea`,
  `key.Binding`, `bubbles/cursor`.

### Modules

Replace in `go.mod` and imports:
`github.com/charmbracelet/bubbletea` → `charm.land/bubbletea/v2`,
`github.com/charmbracelet/lipgloss` → `charm.land/lipgloss/v2`,
`github.com/charmbracelet/bubbles/...` → `charm.land/bubbles/v2/...`.
Drop `github.com/muesli/termenv` if nothing needs it any more. Keep
`github.com/charmbracelet/x/ansi` (used by `ui/hold`); let `go mod tidy`
settle its version. `mise run check` must pass at the end.

### Views

`View() string` becomes `View() tea.View` only on **top-level** models: the
ones passed to `tea.NewProgram`. Those are `reader.Model` (single-file mode in
`main.go`), `app` (folder mode) and the story runner. `app` calls the reader's
view as a string, so give `reader.Model` a `Render() string` holding today's
`View` body, and a `View() tea.View` that wraps it. Story models implement
`tea.Model`, so they need `View() tea.View` too; the runner can use their
`.Content`. `tea.WithAltScreen()` goes away: set `v.AltScreen = true` in each
top-level `View`. Inner helpers (`quizView`, `askView`, `footer`, `header`…)
stay strings.

### Keys

Follow the guide's tables. pain-specific points:

- `case tea.KeyMsg:` → `case tea.KeyPressMsg:` everywhere a key is handled
  (`reader.update`, `updateQuiz`, `updateAsk`, `updatePrefs`, picker,
  browser, `app.Update`, story runner, `ui/hold/stories.go`). Releases must
  not reach today's handlers. In particular, `reader.Update` decides
  `typing := isKey && …` from `msg.(tea.KeyMsg)`, and that must become
  `KeyPressMsg` or a release would open checkpoints.
- `msg.Type` switches in `reader.go` (`KeyCtrlC`, `KeyEsc`, `KeyCtrlO`,
  `KeyCtrlA`, `KeyCtrlE`, `KeyCtrlG`, `KeyCtrlT`, `KeyTab`, `KeyShiftTab`,
  `KeyCtrlL`, `KeyCtrlH`, `KeyCtrlJ`, `KeyCtrlK`, `KeyDelete`,
  `KeyBackspace`, `KeyCtrlW`, `KeySpace`, `KeyEnter`, `KeyRunes`,
  `KeyCtrlP`, `KeyCtrlUnderscore`) and in `ask.go`: move to
  `msg.String()` cases. Check each in the spike program, because v2 tells
  apart keys v1 merged, and pain relies on some of the old merges:
  - `ctrl+/` (key list): v1 got it as `KeyCtrlUnderscore`. Find what v2
    reports in Ghostty and in tmux (`"ctrl+/"`, `"ctrl+_"`, or both) and
    accept all of them.
  - `ctrl+h` versus backspace, `ctrl+j` versus enter, `ctrl+i` versus tab:
    pain binds `ctrl+h`, `ctrl+j`, `ctrl+k`, `ctrl+l` to navigation, so check
    that they arrive as themselves, and that plain enter and backspace still
    do what they did.
  - `alt+enter` (`msg.Alt` with `KeyEnter`) and `alt+backspace`: use
    `msg.Mod.Contains(tea.ModAlt)` or the strings `"alt+enter"`,
    `"alt+backspace"`.
  - Space: `String()` is now `"space"`, not `" "`. Fix `updateQuiz`
    (`next := key == "enter" || key == " "`), `updatePrefs` (`" "`), and any
    other `" "` comparisons (`git grep -n '" "'`).
- Typing: `msg.Runes` → `msg.Text` (a string). `reader.update` types each
  rune of it.
- Paste: v1 flagged pasted text with `msg.Paste` on a key message. v2 sends
  `tea.PasteMsg{Content}`. `reader.update` ignores pastes while typing (keep
  ignoring `PasteMsg` there). `ask.go` turns a pasted or dropped file path
  into an image attachment (`droppedImage(...)` when `msg.Paste ||
  len(msg.Runes) > 1`): handle `tea.PasteMsg` for that, and keep the
  multi-rune check on `KeyPressMsg.Text` for terminals that type a dropped
  path instead of pasting it.
- Tests build key messages (`tea.KeyMsg{Type: tea.KeyRunes, Runes: …}` in
  `navigation_test.go`, `check_test.go`). Use `tea.KeyPressMsg{Code: '1',
  Text: "1"}`, `tea.KeyPressMsg{Code: tea.KeyEnter}`, and so on. A small test
  helper that builds a press from a string keeps them readable.

### Colors and theme (`internal/theme`)

- Lip Gloss v2 colors are `image/color.Color`. `lipgloss.TerminalColor` is
  gone. `theme.Color.c` is `nil` for "the terminal's own foreground"; check
  how v2 expresses that (`UnsetForeground` on the style, or
  `lipgloss.NoColor{}`) and keep the meaning.
- `lipgloss.Color("3")` still gives ANSI (American National Standards
  Institute) palette color 3. Check that v2 emits palette colors as palette
  codes (`\x1b[33m`), not converted to RGB. pain depends on that to follow
  the terminal's theme.
- `theme.Detect` queries the terminal's background and foreground with
  `termenv` before the program starts, and works out `dark`, the surfaces
  and `Bright`. Replace it with `lipgloss.BackgroundColor(os.Stdin,
  os.Stdout)` (and a foreground equivalent if there is one), or with
  `tea.RequestBackgroundColor` / `tea.RequestForegroundColor` in `Init` and
  `tea.BackgroundColorMsg` / `tea.ForegroundColorMsg` in `Update`. Keep the
  macOS fallback (`systemDark`) for terminals that do not answer. Pre-start
  detection is simpler, since styles are needed before the first frame.
- `theme.Filter` and `tea.WithFilter(theme.Filter)` in `main.go`: replace
  with handling of the color scheme events found in step 0, in `reader` and
  `app` `Update` (both already handle `theme.ChangedMsg`; translating the uv
  events into `theme.ChangedMsg` in one place keeps that).
- `theme.CellStyle` caches styles by key and is cleared on a theme switch.
  Keep that.

### Pictures (`internal/picture`)

- `lipgloss.ColorProfile()` and `termenv.Profile` are gone. Use
  `github.com/charmbracelet/colorprofile` (`colorprofile.Detect(os.Stdout,
  os.Environ())`), or `tea.ColorProfileMsg`. `blockLines` takes the profile
  to choose true color or 256 colors, and `Lines` returns nothing useful in
  ASCII-only terminals.
- Apply whatever step 0 found for the transmission write.

### Ask box (`internal/reader/ask.go`)

`bubbles/textarea` v2 changed its style API (`Styles`, `SetStyles`,
`StyleState`, `CursorStyle`), cursor handling (`SetVirtualCursor`,
`Cursor() *tea.Cursor`) and possibly `KeyMap`. Today `newInput` sets
`FocusedStyle`, a static cursor (`bcursor.CursorStatic`), and unbinds
`InsertNewline`, `Paste`, `LineNext`, `LinePrevious`. Keep the same look and
the same bindings. Use `go doc charm.land/bubbles/v2/textarea` for each.
Prefer the virtual cursor, which is drawn in the content as today, unless
the real cursor is simpler to place.

### Docs

Update `docs/reference.md` wherever it names v1 behavior. The theme
paragraphs about `theme.Filter` and unknown sequences, the package table,
"Build and check" (the tmux recipe still works) and anything about
`tea.WithFilter`.

### Check, then commit

- `mise run check` passes.
- Drive pain in a detached tmux session as `docs/reference.md` shows, and
  compare against v1 (build the old binary from `git stash` or the previous
  commit). Same screens, same keys: open `samples/tour.md`, type in typing
  mode (ctrl+p preferences), move by sentence (enter/space, delete), tab
  scopes, ctrl+h/j/k/l, ctrl+/ key list, ctrl+a ask box (type, paste,
  alt+enter), folder mode (`pain .` then esc back), the finish screen.
- In a pseudo-terminal (PTY) driver like the one used for `ui/hold` (Python
  `pty.fork`, answer the terminal's color queries `\x1b]11;?` and cursor
  position request `\x1b[6n`, time each frame), check that the hold underline
  still appears within one frame of the press and completes at about one
  second.
- Commit: "Move to Bubble Tea, Lip Gloss and Bubbles v2".

## Step 2: hold with real key release (second commit)

### `ui/hold`

`Hold` today takes keys through `Repeat(key string) (mine bool, cmd)`,
ticks through `Update(msg) (done, cmd)`, and is started with `Start(key)`.
It guesses a release from auto-repeat timing (`wait()`, `firstWait`,
`minGap`/`maxGap`, `lateWait` for slow first repeats, the learnt `delay` and
`every`). Keep all of that as the fallback, and add an exact mode:

- `Hold` learns that releases come, from a `tea.KeyboardEnhancementsMsg` with
  `SupportsEventTypes()`, or from the first `tea.KeyReleaseMsg` it sees. Give
  it one entry point for messages, so callers do not have to know: for
  example `Update(msg)` also takes `KeyboardEnhancementsMsg` and
  `KeyReleaseMsg`, and returns `done` and a `Cmd` as now.
- In exact mode:
  - a hold completes when `Duration` has passed and no release came, with no
    repeat needed (key repeat may be off);
  - a release of the held key stops it at once (`Running()` false, `Tapped()`
    true if it was short);
  - presses with `IsRepeat` of the held key or of the spent key are the
    hold's (`Repeat` returns true). The spent key stays the hold's until its
    release, instead of until a gap.
- Keep `Underline`, `Hint` and `Tapped` working the same in both modes.
- Match keys by `String()` as now, if step 0 confirmed release strings equal
  press strings. Otherwise compare `Code` and `Mod`.
- Tests: add exact-mode tests next to `hold_test.go`'s (they already inject a
  clock through `now`): completes without repeats, release cancels at once,
  spent key repeats ignored until release, falls back when nothing says
  releases come.

### Wiring

- Ask for the events: set `v.KeyboardEnhancements.ReportEventTypes = true` in
  the top-level views (reader, app, story runner). Turning it on only while a
  quiz is open is possible, but every press handler already ignores
  releases after step 1, so always on is simpler. If step 0 showed side
  effects, turn it on only while `m.quiz != nil`.
- `reader.Update` and `app.Update`: pass `tea.KeyReleaseMsg` and
  `tea.KeyboardEnhancementsMsg` to `m.hold`. The release must reach the hold
  even though it is not a press (see `holdEvent` / `updateQuiz` in
  `internal/reader/check.go`).
- `ui/hold/stories.go`: the same, and show which mode the hold is in (for
  example `releases: seen` or `releases: guessed from key repeat`) in the
  story, so the user can compare inside and outside tmux.

### Docs and check

- `docs/reference.md`, "Active reading" → "Questions": today it says pain
  watches the key repeat and that with key repeat off no answer counts. Say
  that in terminals with the kitty keyboard protocol (Ghostty, kitty,
  WezTerm, foot) letting go is seen at once and key repeat is not needed;
  elsewhere, and inside tmux, pain watches the key repeat as before.
- `mise run check` passes; PTY driver still passes in fallback mode (a PTY
  sends no releases).
- Commit: "Answer a held question on key release where the terminal reports
  it".

## Report back

Tell the user, in the terminal:

- what step 0 found (images, raw escapes, light/dark, release events in
  Ghostty and in tmux), and any fix that changes how something works;
- the two commits, and what you could not check yourself. You cannot hold a
  physical key, so list what the user should try in Ghostty outside tmux and
  inside tmux: `mise run stories`, hold `1`, let go half-way, tap; then a real
  checkpoint in `samples/tour.md` with active reading on (ctrl+t), the ask
  box (ctrl+a) with a pasted image path, an image in the tour, and a
  light/dark switch while pain runs.
