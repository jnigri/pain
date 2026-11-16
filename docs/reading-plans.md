# Writing a reading plan

A reading plan says what to read in a folder, in which order, and why. pain
lists it first in the folder view and takes you through it row by row (see
[Reading plans](reference.md#reading-plans) for how it looks and which keys
work). This page is about writing one, by hand or with the `reading-plan`
skill for Claude Code (Claude's command-line tool).

## Where it goes

Each Markdown file in `<folder>/.reading-plans/` is one plan, listed by file
name. Name the file after the goal: `app-apps-appset.md`, `auth-flow.md`.

Some folders are rebuilt by a script that deletes them first (imported docs,
generated code). A plan inside such a folder is lost on the next rebuild: keep
a copy, or make the script spare `.reading-plans/`.

## The format

```markdown
# Argo CD: Application to ApplicationSet

For someone who knows some Kubernetes and nothing about Argo CD. At the end
you can explain what an Application is and why ApplicationSets replaced the
App of Apps pattern.

Three ideas carry everything that follows:

- **GitOps**: Git holds the desired state; Argo CD syncs the cluster to it.
- **Application**: a resource saying "deploy this path of this repo there".

The plan:

- What Argo CD is made of
  - [Architectural overview](02-architecture.md) — the repo server renders, the controller syncs
- The Application
  - [A minimal Application](06-declarative-setup.md#applications)
  - [Sync policy, annotated](files/operator-manual/application.yaml#L213-L258) — automated, prune, selfHeal
- ApplicationSet
  - [Cluster generator](34-applicationset/07-generators/03-generators-cluster.md#L1-L110)
```

| Part | Rule |
|------|------|
| Title | The first `#` heading. Without one, the file name. |
| Description | Everything between the title and the plan: any Markdown, lists included. pain shows it in the preview when the plan is selected. |
| The plan | The **last list** of the file. Put some text between a list in the description and the plan, or Markdown joins them into one list. |
| Row to read | A list item with a link. The link text is its title. |
| Note | The text after the link (a leading `—`, `-` or `:` is dropped). pain shows it above the preview. |
| Group | A list item without a link. It gathers the items nested under it, and shows their progress. |
| Numbering | From the outline: `1`, `1.2`, `1.2.1`. Do not write numbers yourself. |

### What a link can point at

| Link | Reads |
|------|-------|
| `guide.md` | the whole file |
| `guide.md#getting-started` | one section: from that heading to the next heading of the same or a higher level |
| `main.go#L40-L90`, `main.go#L40` | those lines; works for any file |
| `config.yaml` | a file that is not Markdown, shown as one code block |

- Paths are relative to the folder that holds `.reading-plans/`, not to the
  plan file. Absolute paths and `~/` work too, so a plan can point outside the
  folder.
- An anchor is the one GitHub gives the heading: lower case, punctuation
  dropped, each space a hyphen. `## App Of Apps Pattern (Alternative)` is
  `#app-of-apps-pattern-alternative`, and `## Git + Helm` is `#git--helm`
  (`#git-helm` works too). A second heading with the same text is `-1`, a
  third `-2`.
- A section stops at the next heading of its level, so `#applications` on an
  `##` heading includes its `###` subsections.

## Check it

```sh
pain -check <folder>
```

prints every plan row by row, with the words each row reads, and exits with
status 1 if a row is not found. For an anchor that matches no heading, it
lists the anchors the file has:

```
    4.6 Git generator: directories
      MISSING 34-applicationset/07-generators/04-generators-git.md#directories: no heading with that anchor; the file has: git-generator, git-generator-directories, exclude-directories, …
```

In pain itself, such a row is marked `missing` and does not open.

## Writing a good plan

- **Start from the goal.** Write in the description who the plan is for, what
  they know already and what they will be able to explain at the end. The
  rows follow from that.
- **Go from the idea to the details.** What the thing is, then its core
  object, then the patterns built on it, then the advanced cases. Put the
  optional reading last, in its own group.
- **Read parts, not files.** Long reference pages rarely deserve a full read.
  Point at the section that matters, or at the lines of an annotated example.
- **Say why in the note.** "the finalizer and why it matters" tells the
  reader what to take away; the title alone often does not.
- **Fill the gaps in the description.** When the folder lacks the basics (a
  manual written for operators, code with no overview), give them in a few
  sentences or a short list before the plan.
- **Keep it short.** 10 to 25 rows, a few thousand words. `pain -check` gives
  the total; at a typical 200 to 250 words a minute, 10,000 words is under an
  hour of reading, but far more of typing.
- **Use the folder's files.** A plan only points at files. If something is
  missing, say it in a note or in the description rather than linking out.

## With Claude Code

The repository ships a skill, `.claude/skills/reading-plan`. In Claude Code
started in this repository, ask for a plan:

```
/reading-plan docs/argocd — from zero to understanding Application, App of Apps and ApplicationSet
```

Claude surveys the folder, picks the files and sections, writes
`.reading-plans/<name>.md` and runs `pain -check` until every row is found.
To use it from any folder, copy it to `~/.claude/skills/reading-plan/`.
