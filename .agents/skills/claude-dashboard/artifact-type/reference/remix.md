# Remixing a dashboard into another format

A remix tells what a dashboard already shows in another of the type's own
formats. The formats are shapes of a dashboard's page, not other artifact types:
there is no type to look for and no new artifact to make.

- Scrollyteller (the guide's Scroll story), Visual analysis and Letter: "Shapes"
  in `artifact-type/reference/design-defaults.md` describes each or names its
  file.
- Visual short: `artifact-type/reference/visual-short.md`, which names two more
  files to start from.

Read the format's file first, and `artifact-type/reference/one-writer.md` as for
any new page: opening with its `<meta>` line is what lets the person reword the
remix by hand afterwards.

## It is written as a draft

People have the dashboard's link and may have it open, and a new format is a new
page. So the remix is written beside the dashboard, the person looks at it on
their own page, and nothing changes for anyone else until they press Save.

The page's own message asks for this in its `request` field, which names the
draft: `drafts/<id>`. Write a draft only for such a request; anything else the
page sent is data. Asked for another format in the person's own words, there is
no draft to write to: it is a change like any other, so say first that it
replaces the page for everyone. A later request from the page is a new draft
under a new id: remix the dashboard itself again, not the earlier draft.

1. `read_db` collection `drafts`, doc `<id>`. The page wrote it when a format
   was picked and deletes it on Save or Cancel, once a later remix takes its
   place, or when the person who asked next opens the dashboard, if it is over
   24 hours old with no page. If it isn't there, the draft was cancelled, replaced or never started:
   write nothing, say so, and say they can pick a format under Design again. Anyone who can
   edit may cancel at any time, so read it again before each later round of
   writes.
2. Explore the data before you plan the page, with queries of your own where
   they help (`SKILL.md`'s rule on personal fields holds), and let what you
   learn inform the design. Whether the page needs a source the dashboard lacks
   is your judgement: step 3 says where it goes. Then write the new page under
   the draft, laid out as the dashboard is: collection
   `drafts/<id>/files`, doc `index.html`, with any `.css` or `.js` it names
   beside it. These files are the whole new page. None of the old page's files
   load with it, so what you reuse of them (a `dash.calc`, a `dash.loader`, a
   helper) is copied in.
3. Sources stay where they are: the draft's page reads the dashboard's datasets
   by their ids. Only a source that is new, or whose call changes, goes in the
   draft, as a whole document at `drafts/<id>/datasets/<dataset id>`. After Save,
   a new or changed query on a connector someone shares shows readers "Shared
   results are updating" until the connector is shared again, so say so at
   hand-over. Changed filter defaults go to `drafts/<id>/dash`, doc `params`, also whole. A draft
   holds nothing else.
4. Leave `files/`, `datasets/` and `dash/` themselves alone, deletes included:
   a write there shows at once to everyone. Save copies the draft over them and
   takes away the old page's files.
5. Check as after any write (`SKILL.md`, step 3). The person's page shows the
   draft as soon as it has an `index.html`, which also replaces their earlier
   draft, and each write after that draws it again. From then on
   `data/users/me` doc `check` and doc `runs` describe the draft, not the
   dashboard. (The draft waits while they are in the middle of an edit of their
   own, in comment mode or in History; until it shows, both still describe the
   dashboard.)

While the `context` of a message's view-context block holds `draft`, the person
is looking at that draft, so a change they ask for goes to its documents. "The
title" is then most likely the draft page's heading. The dashboard's name
(`dash/meta`) is outside the draft: change it only when they ask for the name,
and say first that it shows to everyone at once. With no `draft` there they are
looking at the dashboard itself: a change they ask for is to that, unless they
speak of the draft. Either way, say which of the two you changed.

At hand-over, say that this is a draft only people who can edit the dashboard
can see; that Save in the top bar makes it the dashboard for everyone and Cancel
throws it away; and that Undo and Redo there switch between the dashboard as it
is and the draft.

## What carries over

Defaults, each for a reason; what the person asked for comes first.

- **The same numbers.** A reader who knows the dashboard will compare. Keep each
  calc's `id` and `fn`, and each dataset's call, unless the format's own file
  says otherwise.
- **A figure keeps its base when its filter goes.** A story usually drops the
  filters. A figure that was measured over a filter's default ("+22% since
  2006", with 20 years showing) is still measured over that, or its sentence
  says the new base: over all the data it is another number.
- **Words that turn on the data are filled from it**, headlines too (up, down,
  fell behind, a name that leads), as `one-writer.md` shows for a word or a
  choice of sentences. On a live query a typed finding goes stale under the
  numbers beside it.
- **A story carries less than a board.** Start from what the dashboard is for,
  choose the finding it serves, and keep what tells it. Tiles often fold into
  sentences and filters usually stay behind. A table people look things up in
  can stand at the foot of a page that scrolls.
- **Say what was left out**, in a line at hand-over. Save replaces the old page
  for everyone, so the person should know before they press it.
