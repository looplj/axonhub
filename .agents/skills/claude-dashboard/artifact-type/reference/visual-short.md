# Visual short

A Visual short is a dashboard that plays: one finding, told in a few chapters on
one stage, each a drawing that builds while a caption says what it shows, a
minute or two in all. It is for people who would rather watch than read: an
all-hands, a quarter's review, a link dropped in a channel. It works when one idea
is on screen at a time and each mark arrives with the words that name it.

It is built from datasets and marks like any dashboard. Below: what has to hold,
each with its reason, then suggestions, which give way to what the person asks
for.

Read `artifact-type/reference/visual-short-skeleton.md` (HTML and CSS) and
`artifact-type/reference/visual-short-script.md` next and start from them: they
already do what has to hold, so the work is the piece itself. The first says why
one reply may not be enough, and what holds if you write the piece in several.

## What has to hold

- **One writer for each part** (`artifact-type/reference/one-writer.md`), or the
  page check fails. Words known as you write are in the HTML. Each marked figure
  in the HTML, each chart and whatever else a script writes is a `data-draw`;
  outside them a script, your scenes too, sets only `class`, `style` and
  `aria-*`.
- **A mark holds its real value from the first draw.** The dashboard checks
  every mark that is rendered against its data, at any opacity and at any
  moment. So a chapter that isn't playing is drawn whole, as at its end, and
  only the one playing is drawn at its time. A number that counts up does so in
  an unmarked `aria-hidden="true"` copy, a drawing of its own, laid over the
  marked one; once the count ends the marked one shows and the copy hides.
- **Chapters share one place and cross-fade.** Stack the chapters in one place,
  and the captions in a stack of their own, and fade them with `opacity`: under
  `display: none` a drawing measures 0 wide and its marks go unchecked. A
  chapter not showing is `aria-hidden` and takes no pointer. That doesn't stop
  the keyboard, so put nothing in a chapter that takes focus: no link, control
  or `tabindex`.
- **Captions are hidden from the eye alone.** Where a sentence or a caption is
  hidden (the sentences not being said; all of them with CC off; not one the
  data rules out, which is `hidden`), hide it with `opacity`, never `display:
  none`, `visibility: hidden`, `hidden` or `aria-hidden`: under any of those a
  screen reader loses the words and Listen doesn't read them, and under
  `display: none` or `hidden` a figure in the caption goes unchecked. With
  `opacity` a screen reader and Listen get every caption, in order: to a screen
  reader they are a transcript after the chapter showing. Every chapter keeps an
  empty `.vs-say` after its headline, the row its caption is laid over: without
  one the caption lies over the drawing.
- **Motion is drawn from time, in script.** Each chapter is a function of its
  own time `t` in seconds, whole at the end of `t`, so it can be held, replayed
  or jumped to and still look right. For this shape that replaces what
  `design-defaults.md` says of `@keyframes`. `t` runs in one animation-frame
  loop, stopped while nothing plays. Each frame writes only styles and
  attributes that aren't marks; text and elements are written only when they
  change, since each such write under `#dash-root` has the dashboard check all
  the marks again. On new data or a new width the drawings run again, the
  skeleton's `settle` last, which redraws the chapter showing at its current `t`
  and leaves `t` alone.
- **The reader starts it.** It opens on its title, whole and at rest, which is
  also what a thumbnail shows, and plays only when they press Play or pick a
  chapter: motion nobody asked for distracts. Its controls are `<button>`s in
  the HTML, each with a name (its words or an `aria-label`; the script sets a
  chapter mark's again from its chapter's label, so that a hand edit reaches it,
  and on a page whose script doesn't, keep the two in step), so a keyboard and a
  screen reader can work them.
- **It fits the room it is given.** A stage that runs off the view loses its
  foot. The room changes with the window and with the dashboard's own bars,
  above the page and under it, so measure it in the box the page scrolls in, and
  set the stage's width from the height there is room for, so it keeps its shape
  (`fit` in the script).
- **Sizes need a container.** Nothing above the page is a container, so
  `@container` never matches and `cqi` measures the window. Set
  `container-type: inline-size` on the wrapper, for rules at narrow widths, and on
  the stage. Size what is inside the stage in `cqi`
  (`.vs-caps { font-size: max(14px, 2cqi); }`): `cqi` on an element measures the
  nearest container above it, never the element itself.
- **`prefers-reduced-motion: reduce`** shows each chapter's drawing whole at once,
  with only the fade. Its sentences still take their turns.
- **The voice is an extra.** Some readers have Listen
  (`artifact-type/reference/read-aloud.md`); many don't, so the piece plays whole
  without it. Mark it to be read, as the skeleton does: the captions' holder
  `data-read`, with `lang` for the language they are written in and nothing in
  it but the captions, each caption
  `data-read-step`, and the stage around them `data-read-stage`. Without that last
  one the dashboard tints the words and scrolls each caption to the middle. Keep
  the captions as the same elements throughout: the dashboard stamps the ones that
  were there when the reading started, and can find a replaced one only by its
  `id`, which the skeleton's captions don't have.
- **While a reading is open, the dashboard keeps time.** `#dash-root` then has
  `data-dash-read-phase`, and the caption being read, or read last, has
  `data-dash-read-now`. When that stamp lands on another caption, or the phase
  turns to `starting` (a press can have the same caption read again), show its
  chapter from `t` = 0. `t` runs only while the phase is `playing`: test for that,
  since nothing is heard during `starting` or `paused` or after `ended`. The page
  moves to no chapter by itself: one that finishes early waits, whole, for the
  next stamp. At `ended`, once the last caption is read or as a reader edits by
  hand, show the chapter whole and stay on it. When the phase goes the voice is
  off, by a press or because it failed: carry on without it from the same place,
  playing if it was starting or playing.
- **One set of controls, with the voice or without, and one clock at a time.**
  The page keeps its own controls through a reading, and the dashboard draws
  none; `read-aloud.md` has the rules. The speaker is `data-read-sound`, stored
  `aria-pressed="true"`: the voice is on at first and comes with Play. The
  dashboard keeps `aria-pressed`, by which CSS shows a speaker or a struck-out
  one. Play and Pause is `data-read-pause`; Back, Next and each chapter mark are
  `data-read-move`.
  While a reading is open their `onclick`s still show the chapter, but start and
  stop no clock of the page's: the two would run against each other.
- **Every number in a caption is marked and filled from the data,** as a
  Letter's are, whatever the source. Words you type are stored for everyone and
  never change, while each reader's page runs a live query on their own
  connection: a typed number would reach readers with no access to the source,
  could contradict the figure on their stage, and would go stale. Words that
  turn on the data (up, down, more, fewer) are filled too, from a calc. A goal
  the person named is one constant in the script, unmarked, drawn wherever it
  shows (`one-writer.md`). What you type is the gist, which holds whatever the
  numbers are.

## Suggestions

**The piece.** Five to seven chapters usually carry one finding. The first is a
title: what it is, the period and how long it runs (worked out from the captions,
not typed). Set the title at about 6cqi, or put the one figure the story turns on
under a headline. The last is what to do next; it reads best ending with every
mark at full strength, since it stays up. The stage stands alone: tiles, filters,
tabs, tables or text around it pull the eye from what is playing.

**The stage.** 16:9 suits a wide page; in a wrapper under 800px (beside a chat) a
square uses the room better, and under 560px `4 / 5`, sized as in the skeleton.
`background: var(--color-panel)`, `border: 1px solid var(--color-border-line)`,
`border-radius: var(--cds-radius)`; no shadow, gradient or rule down the left.

**A chapter's frame**, the same each time so only the content changes: a small
uppercase label for the part of the story; a one-sentence headline that states
the finding, not the topic; the caption's row; one drawing; a small muted source
line (measure, who, dates). The chapter marks run across the top of the stage,
and the controls stand at the top right, on the label's line. So keep the label
short, and keep it on every chapter, the title too.

**Type** is set as on a slide, in `cqi` of the stage: headline about 3.4 and 18px
at least, to stay larger than the caption under it, labels and values on marks
1.8 (1em inside a drawing), captions 2, source line 1.3, a big number 12.

**The drawing fills its box,** the room between the caption's row and the source
line, which is most of the stage. The skeleton makes that box a container of both its sizes:
inside it `cqh` is 1% of its height, `cqi` 1% of its width, a little under the
stage's, and `em` the labels' size, set on the box. Size what is drawn from the
box, not from a fixed number of pixels. Bars share its height: each row is
100cqh over their number, up to about 18cqh, and the bar some 60% of its row.
Columns stand its full height, less their labels. A dot grid is the largest
square the box holds, `min(100cqh, 100cqi)` a side. What can't fill the box sits
in its middle, as the skeleton has it, not at its top.

**The drawing** is the one that shows the sentence. A share of people: a 10 by 10
grid of dots that fills, then recolors. A total: one big number that counts, with
what made it up beside it. A comparison: horizontal bars from zero, slightly
rounded. A gap between two: two bars and a bracket naming the gap. A run of
chapters that are all bars is a sign another drawing would fit somewhere. A bar
or dot painted with `background` vanishes in forced colors, so give it
`forced-color-adjust: none` and, under `@media (forced-colors: active)`, a system
color such as `CanvasText`, as the chapter marks in the skeleton do.

**How marks arrive.** Bars grow from zero, a value fades in once its bar lands,
dots come in a quick wave. 400 to 700ms with `d3.easeCubicOut` feels right (for
the chapters' fade, `--cds-ease-out`); camera moves, 3D and bounce don't. Gray
first, with one accent color on what the sentence is about; to focus, fade the
rest to about 25% opacity rather than add a color. Labels sit on the marks, so
gridlines, axis numbers and legends can go, and so can the tooltip `SKILL.md`
asks of a chart: every value is labeled.

**Captions.** One paragraph a chapter, 150 to 250 characters in two to four
short sentences, each in its own `.vs-s` span as in the skeleton and under some
100 characters, so that it fits its row. They read best
written as if said aloud, with no brackets or symbols, and are in the HTML. A
voice reads only the captions, so each says what its chapter shows without
leaning on the headline.

**Figures in captions.** Phrase each sentence so that it reads well around a
filled figure. By default that figure is rounded for the ear, to about two
significant figures ($100 for 102.08, 1,900 for 1,937), with "about" typed before
it where it helps, while the stage shows the exact one. A `dash.calc` returns the
rounded number and the caption's mark names that calc (`said` in the script), so
the dashboard still checks it. A calc can return words too ("almost half"); those
it can't check. Or a caption can leave the number out and let the figure on the
stage show it. Work out each mark's cue (see Pace) after the numbers are filled,
since filling moves where the words fall. A Visual short usually tells of a
period that has ended, so pin the dates in its queries: the gist is yours, and
should stay true.

**How captions show.** The sentence being said stands alone under the headline,
as plain words, so it covers nothing drawn. Each chapter keeps a row for it
(`.vs-say`, empty in the HTML), and the script lays the caption over that row.
The row is as tall whatever is being said, and with captions off, so the drawing
never moves. On the title before Play the sentence is the first; once a chapter
is whole, the last. A CC button among the controls, `aria-pressed` and on at
first, switches captions off and on; a page may use no storage, so the choice
lasts until it is loaded again.

**Pace.** People read the caption and watch the drawing at once, so a second for
every 15 characters is comfortable: a chapter lasts its caption's length divided
by 15, in seconds, plus 1. A mark's cue is the place where its word starts in
the caption (`indexOf`), divided by 15, so no seconds are typed. A voice keeps
its own pace, which the page isn't told. So while a reading is open the script times the voice: each caption it
reads through sets the pace for the next, and every cue, the sentences' too, is
scaled by it. A mark that hasn't moved when the voice goes on is drawn whole as
it leaves. When the voice goes off, `t` is divided by the pace, so the piece goes
on from the same place. With no reading open, at a chapter's end the next starts from `t` =
0; at the last it stops, and Play starts over.

**Controls.** Play or Pause, Back, Next and one mark per chapter let a reader
hold it and find their place; a chapter is the smallest part that makes sense
alone, so a slider adds little. The marks are a band of bars across the top of
the stage, each a button named for its chapter's label that plays from that
chapter's start, the current one with `aria-current="true"` and filling as it
plays. They stay up: they show how far the piece has got. The rest are slight:
bare, with no box around them, at the stage's top right on the label's line,
away from claude.ai's own bars under the page; on a narrow stage they take a row
of their own above the label. In order: Play or Pause (both in the HTML, one
shown), Back, Next and the speaker, which the dashboard hides where there is no
voice, each the inline SVG in the skeleton (the dashboard's own icons; a page has no icon font, so no text,
emoji or other drawing in their place) and named by an `aria-label`, the
toggle's set by the script to Play or Pause; and CC. While the piece plays they
fade as a video's controls do: as the pointer leaves the stage, or after a mouse
has lain still on it for 3 seconds. They come back when the pointer moves or
presses on the stage, or the keyboard moves focus into it. At rest, paused and
at the end they stay up, so Play can always be found; where the main pointer
can't hover, as on a phone, they never fade. They fade by `opacity`, so a
keyboard and a screen reader still reach them. With the voice on they fade and
come back the same way. The dashboard pauses the piece when a reader opens
Sources, picks a figure there or comments, and has each chapter stand whole
while they edit by hand (`read-aloud.md`).

## Handing over

Besides what `design-defaults.md` has you tell the person: how long it runs, and
that Play starts it. Of the voice, say nothing unless the person asked to listen
(`read-aloud.md`).
