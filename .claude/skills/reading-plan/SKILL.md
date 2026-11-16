---
name: reading-plan
description: Write a pain reading plan for a folder: an ordered, annotated list of the files and sections to read to reach a learning goal, saved in <folder>/.reading-plans/<name>.md and checked with `pain -check`. Use when the user says "/reading-plan", asks for a reading plan, a learning path or a reading order through a folder of docs or code, or wants to learn a subject from the files in a folder.
argument-hint: <folder> — <what the reader should understand at the end>
---

# Write a reading plan

Arguments: `$ARGUMENTS`

You write a reading plan for [pain](../../../docs/reading-plans.md): a Markdown
file in `<folder>/.reading-plans/` that pain shows on top of the folder and
walks the reader through, row by row. Read `docs/reading-plans.md` in the
pain repository for the full format if anything below is unclear.

## 1. Pin down the goal

You need three things. Take them from the arguments; ask only for what is
missing, in one question:

- **the folder** to plan through;
- **the goal**: what the reader can explain or do at the end, in concrete
  terms ("explain Application, App of Apps and ApplicationSet", not "learn
  Argo CD");
- **the reader**: what they know already. Assume a working developer new to
  the subject if nothing says otherwise.

## 2. Survey the folder

Read before choosing. Do not edit anything in the folder except
`.reading-plans/`.

- List every file with its line count (`find <folder> -type f -not -path '*/.*' | xargs wc -l`),
  and read the READMEs and tables of contents.
- For the candidate Markdown files, list their headings with line numbers
  (`grep -n '^#' file.md`). Lines starting with `#` inside code blocks are
  not headings: check the context when in doubt.
- Read the candidates' key passages, enough to know what each section really
  teaches and whether it is written for the reader you have in mind.
- For code, find the entry points, the core types and the smallest
  end-to-end path, with line ranges.
- Note what the folder lacks: basics it takes for granted, an overview it
  never gives. Look for a script that regenerates the folder (an import or
  docs refresh script deleting it first): a plan inside would be lost, so
  tell the user.

## 3. Design the path

- Order by what each step needs: what the thing is, then its core object,
  then the patterns built on it, then advanced cases. Put optional material
  last, in a group such as "Going further".
- Group rows under short headings (list items without a link), one per stage
  of the path.
- Prefer **sections and line ranges** over whole files. A long reference page
  is rarely worth a full read: point at `file.md#anchor` or `file#L40-L90`.
- Aim for 10 to 25 rows and a few thousand words. More is a course, not a
  plan.
- Every row gets a **note** after the link saying what to take from it, not
  what it is called.
- Put in the description who the plan is for, what they will be able to
  explain at the end, and, in a few sentences or a short list, the basics the
  folder lacks. End the description with a line of text such as "The plan:"
  so a list in it is not joined to the plan.

## 4. Write it

Save `<folder>/.reading-plans/<goal-in-kebab-case>.md`:

```markdown
# <Subject>: <goal in a few words>

<Who it is for, what they will know at the end.>

<The basics the folder lacks, if any.>

The plan:

- <Stage>
  - [<What this row teaches>](<path relative to the folder>#<anchor>) — <what to take from it>
  - [<…>](<file>#L10-L40) — <…>
- <Next stage>
  - …
```

- Paths are relative to the folder that holds `.reading-plans/`, not to the
  plan file.
- Anchors are GitHub's: lower case, punctuation dropped, spaces become
  hyphens, a repeated heading takes `-1`, `-2`. A section runs to the next
  heading of the same or a higher level, so it includes its subsections.
- Files that are not Markdown are shown as code: use line ranges on them.
- Do not number the rows; pain does.

## 5. Check it

```sh
pain -check <folder>
```

If `pain` is not installed or does not know `-check`, run
`go run . -check <folder>` from the pain repository root.

It prints every row with its word count and marks the ones not found as
`MISSING`, listing the anchors the file has when an anchor is wrong. Fix the
rows and run it again until it prints `ok`. Then skim the word counts: a row
far larger than its note suggests is a section that runs on; narrow it with a
deeper anchor or a line range.

## 6. Report

Tell the user, in a few lines:

- the plan's path and its stages, one line each;
- the number of rows and words, and a reading time at about 200 words a
  minute;
- what the folder lacked and how the description covers it;
- any risk to the plan, such as a script that regenerates the folder.

They open it with `pain <folder>`: the plan is listed first.
