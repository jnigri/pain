<p align="center">
  <img src="docs/hand.png" width="240" alt="A pixel-art hand, reaching for the keyboard">
</p>

<h1 align="center">pain</h1>

<p align="center"><b>Read Markdown by typing it.</b></p>

`pain` is a terminal app that shows a Markdown file and makes you retype it,
one block at a time, in the spirit of [monkeytype](https://monkeytype.com).
There is no score and no timer. Typing keeps you active, so you take in what is
on the page instead of skimming it.

```
 notes.md  ◆ active                                  ━━━━━━━╺━━━━━━━━━━━━  35%

 Tides
 ━━━━━

 Tides are caused mainly by the gravity of the Moon. The Moon pulls harder
 on the side of the Earth that faces it█than on the far side, and this
 difference stretches the oceans into two bulges.
```

Text you have not typed yet is dimmed, and lights up in its real style as you
type it.

## What it does

- **Rendered Markdown.** Headings, lists, tables, quotes and code look like
  what they are, not like raw Markdown.
- **Forgiving typing.** Punctuation fills itself in, case does not matter, and
  `é` accepts `e`. You can skip a sentence, a line or a block.
- **Folders.** `pain DIR` lists your files with how much of each you have
  read, and about how long the rest takes at your speed.
- **Resume.** pain remembers where you stopped in every file.
- **Ask Claude.** `ctrl+a` asks a question about the sentence or block you are
  on, and keeps the conversation next to it.
- **Active reading.** Each section ends with a checkpoint that tests what you
  just read.

## Install

You need Go 1.27 or later.

```sh
go install .
pain samples/tour.md    # a tour of everything pain renders
```

## Use

```sh
pain notes.md    # read one file
pain .           # browse a folder
```

| Key       | Action                                        |
|-----------|-----------------------------------------------|
| `ctrl+l`  | skip the sentence (`ctrl+h` goes back)        |
| `tab`     | work on the whole block instead of a sentence |
| `ctrl+a`  | ask Claude about the sentence or block        |
| `ctrl+t`  | turn active reading on or off                 |
| `ctrl+p`  | preferences                                   |
| `ctrl+/`  | list every key                                |
| `esc`     | save your position and leave                  |

## Active reading

Press `ctrl+t` in a file, or `t` in the folder view, and pain remembers that
this folder wants more from you. From then on, each section ends with a
checkpoint, in one of two kinds:

- **Questions.** Claude writes a few multiple-choice questions per section the
  first time you open a file. You answer them and see why the right choice is
  right.
- **Explain.** You write what the section says in your own words. Claude tells
  you whether you got it, without nitpicking, and you can discuss it from
  there.

`tab` switches between the two, and `esc` skips a checkpoint. A short file is
one section; a longer one is cut at its headings.

## Claude

Asking and active reading run [Claude Code](https://claude.com/claude-code),
so `claude` must be installed and logged in. They use that account, and no key
is needed. Everything else works without it.

## More

[docs/reference.md](docs/reference.md) has every key and preference, the files
pain writes, and how the code is laid out.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and
[goldmark](https://github.com/yuin/goldmark). Colors from the
[0x96f](https://github.com/filipjanevski/0x96f.nvim) theme.
