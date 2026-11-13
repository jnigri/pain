# pain reference

Every key, preference and file, and how the code is laid out. For a short
introduction, see the [README](../README.md).

`pain` is a terminal app that renders a Markdown file and makes you retype it,
one block at a time, in the spirit of [monkeytype](https://monkeytype.com).
It is not a typing test: there is no timer and nothing to win. The point is to stay
active while reading, so you actually take in what is on the page.

```
pain notes.md
```

The file is shown rendered, not as raw Markdown: headings are colored, lists
get bullets, code sits on its own background. Text you have not typed yet is
dimmed. As you type, it lights up in its real style. The sentence you are on is
underlined.

The colors come from the [0x96f](https://github.com/filipjanevski/0x96f.nvim)
theme.

## Install

You need Go 1.27 or later.

```sh
go install .          # installs `pain` into $GOBIN (usually ~/go/bin)
# or
go build -o pain .    # builds ./pain in the repository
```

Try it on the bundled tour, which shows every supported element:

```sh
pain samples/tour.md
```

## Usage

```
pain [-restart] FILE.md
pain DIR
```

`pain` remembers where you stopped in each file and resumes there next time.
`-restart` ignores the saved position and starts from the top.

### Reading a folder

`pain DIR` (for example `pain .`) opens a folder view in the style of
[mini.files](https://github.com/echasnovski/mini.files): the parent folder, the
current folder and a preview of the selected entry, side by side. Only
Markdown files and the folders that hold some are listed; hidden entries are
left out.

The header shows how much of the folder you have read, in words, how many
files you have finished and, once pain has seen you type for a while, about how
long the rest takes at your speed. Each entry is marked `✓` when read to the
end, or with the share you have read. A file whose checkpoint questions you
answered also shows its score, such as `4/5`: the answers right over the
questions asked, counting the latest try of each section. Folders and the
header sum the scores under them. The preview shows a file from where you
stopped.

`s` shows your stats, for this folder next to everything you typed in pain:
words typed, time spent typing, average and top speed in words per minute
(wpm), questions answered right and explanations written. The top speed is the
fastest run of 20 words. Questions count every try, so answering a section
again adds to them. The folder column only counts sessions from the version
that added it.

The view opens on the file you read last, and `r` reopens it where you
stopped. In a file, `esc` or `ctrl+o` saves your position and goes back to the
folder view, on that file. Finishing a file also goes back, with the next file
you have not finished selected, so `enter` carries on.

| Key                | Action                                  |
|--------------------|-----------------------------------------|
| `j` / `k`          | move the selection (`↓` / `↑` too)      |
| `l` / `enter`      | open the folder or file (`→` too)       |
| `h` / `-`          | go up a folder (`←` too)                |
| `g` / `G`          | go to the first / last entry            |
| `n`                | select the next file not read yet       |
| `r`                | reopen the last file you read           |
| `x`                | mark the file read, or unread           |
| `s`                | show the stats: words, speed, questions |
| `t`                | turn active reading on or off for the folder |
| `q` / `esc`        | quit                                    |

### What you type

Each paragraph, heading, list item, table row and code line is one block. When
you finish a block correctly, the next one starts on its own.

Some things are filled in for you:

- bullets, numbers, task boxes and quote bars
- section numbers at the start of a heading, such as `1. ` or `2.3 `
- leading indentation in code blocks
- table borders (type a row as its cells separated by single spaces)
- characters you cannot type on a normal keyboard, such as emoji and arrows

Some characters accept a plain keyboard equivalent: `“ ”` accept `"`, `‘ ’`
accept `'`, `– —` accept `-`, `…` accepts `.`, and accented letters accept their
base letter (`é` accepts `e`).

Enter counts as a space, so you can press it at the end of each screen line.
`ctrl+l` and `ctrl+h` move by sentence, the one the underline shows.
`ctrl+l` fills in the rest of the sentence you are on, and `ctrl+h` un-types
it; at a sentence start, `ctrl+h` goes back to the one before.

`tab` widens the selection: from the sentence to the whole block, then to the
section, then back to the sentence. `shift+tab` goes the other way. The footer
shows the three, with the one selected underlined. The underline covers the
whole block when the block or the section is selected, and a bar in the left
margin runs along the section when it is. A block of a single sentence goes
straight to the section. The selection is what `ctrl+l` and `ctrl+h` move by,
what `ctrl+a` asks about and what `ctrl+e` has you explain. Sections are the
ones active reading uses (see below), also when it is off.

`ctrl+j` and `ctrl+k` do the same by screen line. Going forward past a block's end moves to
the next block, and going back past its start moves into the previous one.
`alt+enter` also skips a line, and terminals can send it for `shift+enter`,
for example Ghostty with `keybind = shift+enter=text:\x1b\r`.

### Images

An image that is a file on your machine is drawn where the Markdown puts it,
from `![alt](path)` or from a HyperText Markup Language (HTML) `<img>` tag.
Relative paths start from the folder of the Markdown file. Images are never
typed. They are centered, at most 20 lines tall, and never larger than their
real size.

How an image is drawn depends on the terminal:

- **Kitty and Ghostty** show the real picture, through the
  [Kitty graphics protocol](https://sw.kovidgoyal.net/kitty/graphics-protocol/).
  Inside tmux this needs `set -g allow-passthrough on`; without it pain falls
  back to blocks.
- **Other terminals** show it in half-block characters, two pixels per
  character. This is coarse: fine for a photo or a logo, not for a diagram
  with small text.

Set the `PAIN_IMAGES` environment variable to `kitty`, `blocks` or `off` to
choose yourself. The formats read are Portable Network Graphics (PNG), Joint
Photographic Experts Group (JPEG), Graphics Interchange Format (GIF, first
frame only) and WebP.

An image that cannot be drawn (a missing file, another format) is shown as a
line with its alt text. Images from the web are not fetched: their alt text
stays in the paragraph, to be typed like the rest.

### Keys

| Key                              | Action                          |
|----------------------------------|---------------------------------|
| `backspace`                      | delete one character            |
| `ctrl+w` / `alt+backspace`       | delete one word                 |
| `enter`                          | type the next space             |
| `tab` / `shift+tab`              | widen / narrow the selection: sentence, block, section |
| `ctrl+l`                         | skip the rest of the selection  |
| `ctrl+h`                         | back to the start of the selection, then the one before |
| `ctrl+j` / `alt+enter`           | skip to the next screen line    |
| `ctrl+k`                         | back to the start of the screen line, then the line before |
| `ctrl+a`                         | ask Claude about the selection  |
| `ctrl+e`                         | explain the selection in your own words, and have Claude check it |
| `ctrl+g`                         | with active reading, answer the questions of the section you are in |
| `ctrl+t`                         | turn active reading on or off   |
| `↑` / `↓`                        | scroll; any other key brings the cursor back |
| `ctrl+u` / `ctrl+d`              | scroll up / down half a page (`pgup` / `pgdown` too) |
| `ctrl+p`                         | open or close the preferences   |
| `ctrl+/`                         | list every key of the screen you are on |
| `esc` / `ctrl+c`                 | save your position and quit; from a folder, `esc` goes back to it |
| `ctrl+o`                         | from a folder, save your position and go back to it |

The footer only shows the keys that matter where you are, in the width of the
text, on a second line when the screen is narrow. `ctrl+/` lists
them all, and any key closes the list. Terminals send `ctrl+/` as `ctrl+_`, so
that works too.

### Asking Claude

Press `ctrl+a` to ask a question about the selection: the sentence you are on,
or after `tab` the whole block or the section. In a new conversation, `tab`
still moves between the three until you send the first question.

Claude gets the file's path, the headings the block sits under, the whole file,
the block and, for a sentence, the sentence; for a section, the section instead
of the block. The answer is rendered as Markdown. Press `enter`
on an answer to reply: the conversation carries on, and Claude sees the earlier
turns. `n` starts a new conversation about the same block.

A question can carry images. `ctrl+v` attaches the image on the clipboard
(macOS only), and dragging an image file into the terminal attaches that file
instead of typing its path. Attached images show as `▣ name` under the
question; `backspace` on an empty question removes the last one. Up to 5
images of 5 MB each. Only the latest question's images are sent: later turns
see earlier images by name only.

Blocks you have asked about get a purple `●` in the left margin. `ctrl+a` on
such a block reopens its latest conversation about the same selection, so you
can read it again or carry on. A conversation about a section reopens from any
block of it.

This runs the [Claude Code](https://claude.com/claude-code) CLI, so `claude`
must be on your `PATH` and logged in. Questions use that account, such as a
Claude subscription; no API key is needed. pain starts it with `--safe-mode`,
so your own `CLAUDE.md`, skills and hooks do not change the answers, and with
no tools, so it only answers.

| Key           | Action                                                      |
|---------------|-------------------------------------------------------------|
| `enter`       | send the question; on an answer, reply to it                |
| `tab`         | before the first question, move between the sentence, the block and the section |
| `n`           | on an answer, start a new conversation about the same text  |
| `ctrl+v`      | attach the image on the clipboard to the question           |
| `←` / `→`     | move in the question; with `alt` (or `alt+b` / `alt+f`), by word; `ctrl+a` / `ctrl+e` to the start / end |
| `ctrl+w` / `ctrl+k` / `ctrl+u` | delete the word before the cursor / to the end / to the start |
| `↑` / `↓`     | scroll the answer (`pgup` / `pgdown` for half a page; on an answer, `ctrl+u` / `ctrl+d` too) |
| `ctrl+r`      | list the conversations about this file, newest first; `enter` opens one, and you can carry it on |
| `g`           | on an answer or in the list, go to the block it was about   |
| `esc`         | cancel a running question, or go back                       |

Each turn runs `claude -p` again with the conversation so far, so Claude Code
keeps no session of its own, and nothing shows up in `claude --resume`.

Answers are kept in `asks.json` (see below). By default the questions take the
whole screen; the `ask_split` preference shows them below the text instead.

### Explaining in your own words

Press `ctrl+e` to explain what you just read, in your own words and from
memory: the sentence, the block or, after `tab`, the section as far as you have
read it. The text is hidden while you write. Claude then checks the substance,
not the wording:

- when you have the main ideas, it only says so, such as "Yes, you got it.";
- when something important is missing or wrong, it says what, or asks one
  question that leads you to it.

`enter` on the feedback replies, to try again, answer its question or discuss
the text. `esc` goes back to reading. Explanations are not saved; the stats
count them.

This works on any file, with or without active reading, where it is also one
of the two kinds of checkpoint.

### Active reading

Active reading ends each section of a file with a checkpoint, so you test what
you just read before moving on. Turn it on with `t` in the folder view, for the
folder you opened, or with `ctrl+t` in a file: for the folder it was opened
from, or for the file alone with `pain FILE.md`. The same key turns it off. A
green `◆` in the header shows it is on. The choice is saved in the
preferences, under `active_reading`.

A file under 500 words is one section, with one checkpoint at its end. A longer
file is cut at its headings and horizontal rules, and sections under 150 words
join the next one.

You see a checkpoint coming. A line across the text, `◆ checkpoint · 2
questions`, marks where each section ends. While Claude writes the questions,
the header shows how far it is, such as `writing questions 3/8 sections · 5 so
far`, and the lines of the sections still waiting say so. If the questions
could not be written, both say it, and `ctrl+g` opens the error with a key to
try again. The header shows the section you are
in, such as `◆ 2/5`. For the last 25 words of a section it turns yellow and
counts them down: `◆ 2/5 · checkpoint in 12`. Once a checkpoint is passed, its line
shows how it went: `✔ 2 of 2 right`, `● 1 of 2 right`, `✔ explained` or
`◇ checkpoint skipped`.

A checkpoint comes in two kinds. `ctrl+e` (or `tab`) in the questions switches
to explaining, and `tab` in an explanation not sent yet switches to the
questions. pain remembers the one you used last.

- **Questions.** The first time you open a file, Claude writes one or two
  quick multiple-choice questions per section, in the background while you
  type: short choices, on the main point, never trick questions. At
  the checkpoint, pick a choice with `↑` / `↓` and `enter`, or with its number.
  A bar then fills over the question for under a second before the answer
  counts, so a key hit by mistake can be undone: `esc` takes it back, another
  choice changes it, and the same key again skips the wait.
  pain shows the right choice and one line on why, then your score. On the
  score, `r` answers the same questions again, with the choices in another
  order. `ctrl+r` has Claude write new questions for the section, at any
  point of the checkpoint. The questions are kept in `quizzes.json` and
  written again only when the file changes. The latest score of each section
  is kept in `scores.json`, and shows in the text and in the folder view.
- **Explain.** You write what the section says in your own words, and Claude
  checks it, as with `ctrl+e` (see above).

`esc` skips a checkpoint. A checkpoint shows once per section and session, so
going back and forth does not ask again. `ctrl+g` opens the questions of the
section you are in at any time, before its end or to answer them again.

| Key               | Action                                              |
|-------------------|-----------------------------------------------------|
| `↑` / `↓`         | move between the choices (`j` / `k` too)            |
| `enter` / `space` | answer with the choice selected, then go on         |
| `1` – `9`         | answer with that choice                             |
| `r`               | on the score, answer the same questions again       |
| `ctrl+r`          | have new questions written for the section          |
| `ctrl+e` / `tab`  | explain the section instead                         |
| `esc`             | skip the checkpoint                                 |

Like `ctrl+a`, this runs the Claude Code command-line interface (CLI) with the model from the
preferences.

### Preferences

Press `ctrl+p` to change preferences. They save straight away to
`~/.config/pain/config.json` (or `$XDG_CONFIG_HOME/pain/config.json`):

```json
{
  "case_sensitive": false,
  "skip_punctuation": true,
  "ask_split": false,
  "model": "sonnet",
  "checkpoint_explain": false,
  "active_reading": ["/Users/you/notes"]
}
```

| Preference          | Default | Effect                                                                                                                                              |
|---------------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| `case_sensitive`    | off     | When off, `a` and `A` are the same key.                                                                                                             |
| `skip_punctuation`  | on      | Typing the next letter or digit fills in the punctuation and spaces before it. With `). a` ahead of the cursor, pressing `a` types all four. Punctuation at the end of a block is free. |
| `ask_split`         | off     | When on, questions and answers show below the text instead of taking the whole screen.                                                             |
| `model`             | unset   | The Claude model that answers questions, passed to `claude` as `--model`. Chosen the first time you ask, changed from the preferences. `default` uses your Claude Code setting; any alias (`fable`, `opus`, `sonnet`, `haiku`) or full model name works. |
| `checkpoint_explain` | off    | When on, checkpoints ask you to explain the section instead of answering questions. Switching at a checkpoint changes it.                           |
| `active_reading`    | empty   | The folders and files read actively, as absolute paths. A folder covers everything under it. Changed with `t` and `ctrl+t`.                         |

### Files pain writes

| File                                         | Content                                  |
|----------------------------------------------|------------------------------------------|
| `~/.config/pain/config.json`                 | preferences                              |
| `<user cache dir>/pain/progress.json`        | the block you stopped at, per file, or that you read it to the end. On macOS the cache dir is `~/Library/Caches`, on Linux `~/.cache`. |
| `<user cache dir>/pain/folders.json`         | the file you opened last, per folder.    |
| `<user cache dir>/pain/stats.json`           | words typed, time spent typing, top speed, questions answered and explanations written, in total and per file, for the time left and the stats in the folder view. Pauses over 10 seconds count as 10. |
| `<user cache dir>/pain/asks.json`            | every conversation started with `ctrl+a`, per file, saved after each answer. Cancelled and failed questions are not kept. |
| `<user cache dir>/pain/quizzes.json`         | the checkpoint questions Claude wrote, per file, with a hash of the file they were written for. |
| `<user cache dir>/pain/scores.json`          | the latest score of each section's questions, per file, with the same hash. |
| `<user cache dir>/pain/images/`              | images pasted with `ctrl+v`, so saved conversations still point to them. |

A saved position is dropped if the file has changed shape since. A file read
to the end stays marked as read, and opening it again starts from the top.

## How it works

The code is built on
[Bubble Tea](https://github.com/charmbracelet/bubbletea) (the app loop),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) (styling) and
[goldmark](https://github.com/yuin/goldmark) (Markdown parsing, with the GFM
extensions for tables, task lists and strikethrough).

```
 file.md ──markdown.Parse──▶ []Unit ──reader.Model──▶ typing state ──view.RenderUnit──▶ screen
                             (blocks of cells)     (cur, typed)
```

### Packages

`main.go` parses flags and starts the right screen. Everything else lives
under `internal/`, and imports only flow down this list:

| Package    | Role |
|------------|------|
| `markdown` | walks the goldmark AST and flattens it into units; character folding and matching, word wrapping, sentence bounds, word counts. Knows nothing about the terminal. |
| `theme`    | the 0x96f palette and `CellStyle`, which maps a cell's style and typing state to a Lip Gloss style |
| `picture`  | an image file to lines of text: sent to the terminal and shown through placeholder characters (Kitty graphics protocol), or drawn in half-block characters |
| `store`    | state kept between runs: atomic JSON files for saved positions, last file per folder, typing and question stats, questions, scores and conversations, plus the preferences file |
| `view`     | drawing shared by the screens: `RenderUnit` (one unit to styled lines, with the sentence underline) and the key help box |
| `reader`   | the file view: key handling, typing rules, viewport, asking Claude (`ask.go` runs `claude -p` and reads its stream, `askview.go` draws it), active reading (`sections.go` cuts a file into sections, `check.go` holds the checkpoints and the explanations), the preferences and key screens |
| `browser`  | the folder view: scanning the folder, the three columns, reading progress and scores per file, the stats screen |
| `app`      | folder mode: switches between `browser` and `reader` |

### Core types

- **`markdown.Cell`**: one rune plus its inline style (`Attr` flags: bold,
  italic, code, link…) and a `Skip` flag. Skip cells are drawn but never typed.
- **`markdown.Unit`**: one block to type. It holds its cells and how to
  draw the block around them: kind (paragraph, heading, code, rule, spacer, image),
  heading level, indent, list prefix, quote depth. Rules, spacers and images
  are units too, so layout is simply "draw the units in order".
- **`reader.Model`**: the Bubble Tea model for a file. `cur` is the index of
  the current unit and `typed` holds the runes entered for it, one per cell.
  Skip cells are copied into `typed` automatically (`autoSkip`), so `typed[i]`
  always lines up with `Cells[i]`.

### Rendering

`view.RenderUnit` wraps a unit's cells to the content width (88 columns at
most) and draws each cell according to its state: done, pending (dimmed),
wrong, or cursor. Consecutive cells with the same look are grouped into one
styled run, so styling cost grows with style changes, not with characters.

The view only renders the units around the cursor. Units other than the
current one are cached per index and invalidated when the window is resized or
when a unit moves from pending to done. The cursor is kept about a third of
the way down the screen.

## Contributing

### Build and check

```sh
mise run check    # gofmt, go vet and go test ./...
mise run build    # builds pain and installs it into $GOBIN
```

Unit tests cover the pure parts: parsing, matching, wrapping and sentences
(`internal/markdown`), word counts (`internal/browser`) and time estimates
(`internal/store`). To check the screens, run it on `samples/tour.md`, which
contains every supported element. To check from a
script, drive it in a detached tmux session and read the screen back:

```sh
tmux new-session -d -s t -x 100 -y 40 "./pain -restart samples/tour.md"
tmux send-keys -t t -l 'a tour of pain'
tmux capture-pane -t t -p        # add -e to see the color escape codes
tmux kill-session -t t
```

Set `XDG_CONFIG_HOME` to a scratch directory to run with default preferences
without touching your own.

### Common changes

**Support a new Markdown element.** Add a case to `builder.block` (block
elements) or `builder.inlines` (inline elements) in
`internal/markdown/parse.go`. Emit units, and mark anything decorative as
`Skip`. If it needs its own look, add an `Attr` flag or a `Kind` and handle it
in `theme.CellStyle` and, for block-level layout, in `view.RenderUnit`.

**Add a preference.** Add a `bool` field with a JSON tag to `store.Prefs`, set
its default in `defaultPrefs` (`internal/store/prefs.go`), and add a row to
`prefItems` (`internal/reader/prefsview.go`); the preferences screen is
generated from that list. Then read `m.prefs.YourField` where the behavior
lives, usually in `internal/reader/reader.go`.

**Change colors.** All colors are constants in `internal/theme`. Use
palette constants (`theme.Yellow`…) rather than inline hex values, and
`theme.Dim` for pending text.

### Conventions

- Keep `internal/markdown` free of terminal code, and keep imports flowing
  down the package list above: nothing imports `reader`, `browser` or `app`
  except the package above it.
- Comment the why, not the what, and keep comments short.
- Decoration is never typed: if a reader would not read it aloud, it should
  probably be a skip cell.

## Known limitations

- Raw HTML is not rendered, apart from the local images of its `<img>` tags.
- Images from the web are not fetched, and Scalable Vector Graphics (SVG)
  images are not drawn.
- Code blocks have no syntax highlighting.
- Line breaks inside a paragraph become a single space.
- Very wide tables wrap instead of scrolling.
