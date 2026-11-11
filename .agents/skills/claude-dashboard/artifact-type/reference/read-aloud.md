# Reading a dashboard aloud

Reading aloud suits a Letter: prose, written to be read from the first word to
the last. Heard, it still makes sense, so a reader can listen with their eyes
elsewhere. Its entry in `artifact-type/reference/design-defaults.md` says where
`data-read` goes. No other shape in that file has a mark unless the person asks
to listen, a Scroll story included, though its chapters are prose too. A shape
taught in a file of its own says there whether it has one.

A page of tiles, charts and tables is scanned, not read through. Its prose is
often just the takeaway line, and marking that gives a few seconds the eye has
already had, under a Listen button that takes more room than it gives. So a
dashboard of that kind has no mark. If the person asks to listen to one, give
them something worth hearing, such as a summary of a few short paragraphs at
the top (two or more: a single one is not read), and mark that. If they ask to
listen to a Scroll story, put `data-read` on its column of chapters,
`data-read-step` on each chapter, and `<button data-read-play>Listen</button>`
above the first chapter.

It is a progressive enhancement. The dashboard has a voice of its own, Claude's
and not the browser's, for some readers only: many have none, and no page can
turn it on. So a page is whole without it, and shows or says nothing about
listening unless the dashboard says there is a voice.

- `data-read` goes on the one element whose words are read, in order. Only that
  element is read; tables, controls and what is hidden are skipped.
- `lang` on that element says what language its words are in: `lang="en"`,
  `lang="es"`, `lang="pt-BR"`. It is the language you wrote them in, whatever
  language the person wrote to you in or sees the dashboard in. Where the voice
  has that language, it and how figures and dates are said go by it. Without it
  the language is guessed afresh for each part, and a short part of names or
  figures can come out in the wrong one. A paragraph or step in another language, such as a quotation, has
  its own `lang`; a phrase inside a sentence needs none for this. It is in the
  HTML as stored, and no script sets it.
- `data-read-step` goes on each part of it that is one step, such as a chapter
  of a Scroll story: each is read as one, and brought to the middle of the
  view as it is read, or as near as its height allows. Whatever is outside a
  step is read a paragraph at a time. A step or paragraph over 2,000
  characters is said as several lines. One reading takes 100 lines, or some
  20,000 characters, from where it starts; it then stops and shows that this was
  as much as one reading takes; Next, or a later step's own button, reads on when
  the platform's hourly allowance allows.
- Where there is a voice the dashboard draws its own Listen button just above
  the `data-read` element, and its own player there once that is pressed. For
  room it raises that element's top margin, where it is less and not negative,
  to 44px (64px on a touch screen), or to 72px (92px) where the element's first
  words start within 12px of its top edge. The page writes no player, no script
  for either and no `speechSynthesis`.
- Two kinds of page are not read at all, and are told there is no voice, unless
  they have a `data-read-play` button or a `data-read-sound` speaker (below):
  one whose `data-read` element holds one part to read (one paragraph, heading
  or step of 2,000 characters or fewer), and one that marks steps outside a
  `data-read-stage`.
- `data-read-play` on a `<button>` of the page's own starts the reading instead,
  wherever the design wants it; the dashboard then draws no button, and makes
  the room only while its player is open. Where a press would do nothing the
  dashboard hides it, whatever `display` the page's CSS gives it: never set
  `hidden` on it yourself, and leave nothing around it (a bar, a rule, a margin)
  that looks empty then.
- `#dash-root` has `data-dash-read="ready|reading"` where there is a voice and
  `data-dash-read="unavailable"`, or none yet, where there is not. It changes
  after the page loads, so anything else about listening (a line of words, an
  icon, a caption) is shown by CSS alone, and kept outside the `data-read`
  element, or it is read aloud too:
  `#dash-root:not([data-dash-read="ready"], [data-dash-read="reading"]) .hear { display: none }`.
- While a reading is open `#dash-root` also has
  `data-dash-read-phase="starting|playing|paused|ended"`, and the
  `data-read-step` being read, or read last, has `data-dash-read-now`. Both go
  when it is closed. (A piece on a `data-read-stage` is also told `ended` for a
  moment as a reader edits by hand: see below.) A page can move with the reading by these two, in
  CSS or through a `MutationObserver` on `#dash-root` (`subtree`, and
  `attributeFilter` with the two names). Four things break otherwise:
  - Put every rule that fades what lacks the stamp under
    `#dash-root:has([data-dash-read-now])`. With no voice, before Listen is
    pressed, and while words before the first step are read, no step has the
    stamp, and a bare rule would fade them all:
    `#dash-root:has([data-dash-read-now]) [data-read-step]:not([data-dash-read-now]) { opacity: 0.25 }`.
  - Let motion run while the phase is `playing`, and test for that, not for
    `paused`: nothing is heard during `starting` or after `ended` either.
    `#dash-root[data-dash-read-phase]:not([data-dash-read-phase="playing"]) [data-read-step] * { animation-play-state: paused }`
    holds the CSS animations inside the steps.
  - Fade a step with `opacity`, never `hidden`, `display: none`,
    `visibility: hidden` or `aria-hidden="true"`: a step hidden in one of those
    ways when the reading starts is not read.
  - Keep the steps in the HTML, outside every drawing, and don't make one again
    by script: the words read are those that were there when the reading
    started, and only a step with an `id` gets the stamp on its new element.
    Never make one again in answer to the stamp: the new one is stamped in its
    turn, and after 20 such rounds in a row no more stamps are written, which
    leaves the page with none.
- Each step is spoken as one take. 10 to 30 seconds of speech, some 150 to 450
  characters, is a good length, and under 5 seconds (75 characters) is short
  for a take. What the person asks for comes first.
- For chapters, `data-read-play="ch2"` on a `<button>` starts the reading at the
  step whose `id` is `ch2`, wherever the button is, and moves a reading that is
  under way. A press for the step being read leaves it playing, or resumes it.
  For a step that is not among those read, the button acts as a plain
  `data-read-play`. A move can take about a second, which suits chapters better
  than a slider.
- Where an animation stays in one place and shows the reading's place itself,
  `data-read-stage` on the `data-read` element, or one around it, stops the
  dashboard tinting words and bringing each step to the middle. It brings the
  `data-read-stage` element into view as the player opens, and then leaves the
  scrolling to the viewer. The Listen button and the player then stand just
  below the `data-read` element, not above it, as a video's controls do, and
  the room is made at its bottom margin. So the `data-read` element reads best
  as the last thing in the stage, where nothing falls between the words and
  the controls, with a bottom margin of 48px (68px on a touch screen) from the
  start, so that nothing moves when a voice comes (a page with a speaker, below,
  needs no such margin). A stage that fits the view
  with that margin shows whole. Of a taller one only the end that has the
  controls shows as the player opens, and at rest a Listen button at its foot
  is below the fold.
- A piece that plays by itself, with the voice or without, keeps one set of
  controls of its own for both, and the dashboard then draws no button and no
  player, makes no room, moves no focus and, under a `data-read-stage`, scrolls
  nothing. Three attributes, none with a value, each on a `<button>` outside the
  `data-read` element:
  - `data-read-sound` on the speaker, in place of `data-read-play`. It is what
    tells the dashboard that the page keeps its own controls. It is on at
    first, as a video's sound is. While the piece is still, a press only turns it
    off or on. If it is on and no reading is open, the next press of Play, Back,
    Next or a chapter's mark (below) brings the voice, at the top of the
    `data-read-step` that has `aria-current="true"`; if none has, Play reads from
    the top and a move reads nothing. So a piece always opens at rest: no voice
    can start before a press. In play a press ends a reading at once, or with
    none open starts one at that step. The dashboard hides it where there is no
    voice, as it does `data-read-play`, and keeps `aria-pressed` on it: store it
    `aria-pressed="true"`, draw its two icons by that, and never write it
    yourself.
  - `data-read-pause` on Play and Pause. While a reading is open a press pauses
    or resumes the voice, or, when all has been read, reads it again from the
    top.
  - `data-read-move` on Back, Next and each chapter's mark. While a reading is
    open a press takes the voice to the top of the step that has
    `aria-current="true"` once the button's own `onclick` has run, and plays it
    from there, from a pause too; if that is the step being read, it starts
    again. The dashboard hears the press at `#dash-root`, after every listener
    inside it. So that `onclick` is on the button, not on `document`, moves
    `aria-current` at once, not in a timer or a frame, and never calls
    `stopPropagation()`.

  The last two are never hidden, and with the speaker off or hidden and no
  reading open the dashboard leaves their presses alone: they are the page's own
  controls first. Their `onclick`s go by `data-dash-read-phase` alone. With none
  they work the page's own clock, even on the press that brings the voice: the
  phase comes just after it. While it is there the voice keeps time: make them
  show the chapter and move `aria-current`, and start or stop no clock of the
  page's own. On such a page a reading that is refused or cut short ends by
  itself, since there is no player to say so (the dashboard shows a refusal in a
  toast): the phase goes, with no `ended` first, and the speaker is off again,
  unless the reading was paused and no toast came. A voice still silent 4 seconds
  after it starts ends the same way.
  So whenever the phase goes, carry on without the voice from the same place:
  playing if the phase was `starting` or `playing`, paused otherwise. A reading
  comes to `ended` only when all has been read, and the speaker stays on.
  While the piece plays, by either clock, keep the class `vs-playing` on an
  element that holds the `data-read-pause` button, and take it off at once when
  it stops: the dashboard goes by it. The dashboard keeps the speaker off over a
  piece playing in silence. It pauses the piece when a reader opens Sources or
  picks a figure there, and whenever it plays under comments or a dialog: an
  open reading it pauses itself, and with none it presses that button. So write
  no pause of your own for these. In Sources the page's controls still work. An
  edit by hand ends a reading, and pauses a piece in the same way. Then one on a
  `data-read-stage` that has played is told `ended` with no reading open, as the
  edit starts and whenever it plays, as after a move. That `ended` stands for one
  microtask, so CSS never sees it: read it in the `MutationObserver` on
  `#dash-root`. At `ended`, always, stop the clock and show what is up whole, and
  leave it so when the phase goes: that is what the reader edits.
  While the phase is there the speaker is the page's Stop, so it stays in
  place: never `display: none`, `hidden` or `visibility: hidden`. Only the
  dashboard hides it, with `data-dash-read="unavailable"`, while comments or a
  dialog holds the reading paused. It may fade
  with the other controls, as a video's do, so long as any move of the pointer
  over the piece, any tap on it and Tab bring it back.
- Say nothing of listening in your reply unless the person asked to listen.
  Then say only that a Listen button, or the page's speaker, shows for readers
  who have read-aloud, which may not yet include them.
