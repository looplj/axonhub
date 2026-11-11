# Setting up certified queries (Claude Data) for an organization

For answering an admin, or a member who asks why the Certify query switch is off. Answer the question that was
asked, from the part that applies; don't recite this page. Nothing here is for a dashboard's code.

A certified query is one connector call (SQL or any other tool call) that a person in the organization vouched for,
with a title and what it answers. They live as `*.query.json` files (older ones `*.toolcall.json`, still read) under `skills/certified-queries/references/`
in one plugin the organization chose. Claude reads that plugin's skill wherever the plugin is installed, and the
Dashboard reads and writes it through the Claude Data connector as the person at the keyboard: the connector holds
no access of its own, so who may read or save is always the plugin library's decision.

Until a plugin is chosen, nothing is certified and the editor shows a "Certified queries" box in place of the switch:
"Not configured for your organization yet" for a member, Enable for someone who can set them up. Where a query's
editor shows neither that box nor a Certify query switch, certified queries are off for the organization or not
available to it: where Admin settings › Artifacts › Dashboards offers a Certified queries switch, an admin turns
them on there, and until then none of the routes below applies; where it offers none, they aren't available to
that organization. Where the switch is on, Claude Data may be kept from this member: reload, then ask the admin.

## Three ways to turn it on

**1. Enable, from any dashboard (quickest).** Someone who manages the organization's plugins opens a query
source's editor on a dashboard and presses **Enable** in that box. That makes a plugin
named `claude-data` (shown as "Claude Data") in the organization's own plugin library, makes it available to every
member, and chooses it as where certified queries live. Nothing else changes; later the admin can narrow who sees
the plugin in Admin settings › Plugins, or choose a different one in Admin settings › Artifacts › Dashboards. A member who can't press it
sees whom to ask instead. If the library already has a plugin named `claude-data`, Enable stops and says so:
choose that one in admin settings (route 2) rather than make a second.

**2. Admin settings › Artifacts › Dashboards.** The same page lets an admin pick any plugin the organization
already has, in its own library or in a marketplace it added (one synced from a repository only where route 3 is
open), as the home of certified queries, or make the
Claude Data plugin from there (the Plugin dropdown's “Claude Data” choice, then Save). Use this to point certified queries at an existing plugin (for example one a data team already
curates), or after Enable was refused because a `claude-data` plugin exists. The plugin needs nothing special
inside it: the first certification adds the `certified-queries` skill and its `references/` folder.

**3. A plugin kept in a repository (open to some organizations only, for now).** An organization that wants
review on every certified query keeps the plugin in Git and syncs it into a marketplace (Admin settings ›
Plugins › Add marketplace › from a repository),
then chooses that plugin as in route 2. Where this route isn't open, route 2's Marketplace dropdown leaves synced
marketplaces out (apart from one chosen earlier), and no admin setting opens the route. Once a synced
plugin is chosen, the Dashboard shows every certified query but holds the switch
with "kept in a repository", and a certified query's arguments are read-only in its editor: queries are added, changed
and removed by pull request, never from a dashboard, because a sync would overwrite anything saved outside the
repository. The repository holds:

```
.claude-plugin/plugin.json          {"name": "finance-metrics", "description": "…"}
skills/certified-queries/SKILL.md   copy it from a plugin Enable made; a copy that names only *.toolcall.json needs *.query.json added beside it
skills/certified-queries/references/weekly-signups.query.json
```

and one query file:

```json
{
  "id": "weekly-signups",
  "title": "Weekly signups by platform",
  "description": "Completed signups per ISO week, split by platform.",
  "call": { "connector": "BigQuery", "tool": "bigquery_query", "args": { "sql": "SELECT … WHERE week >= {{start}}" } },
  "filters": { "start": { "label": "From", "type": "date" } },
  "hints": ["Stack by platform.", "Never add web to the total."]
}
```

`id` is lower-case letters, digits, `-` and `_` (64 at most) and is the file's name. A record with no `kind`, as this one, is the older form, whose `{{start}}` Claude Data fills as a quoted literal; one certified from a `kind: "call"` query carries `kind` and its `rules`. `call` is exactly
`connector`, `tool` and `args`, with each value a reader supplies left as `{{name}}` and described under `filters`
(`type` is `text`, `number`, `boolean` or `date`; `many: true` takes a list; `allowed` lists the only values it
takes). `hints` are short sentences Claude follows when it uses the query, 20 at most.

## Which to pick

| Situation | Route |
| --- | --- |
| Just want it on; trust editors to certify from dashboards | 1 (Enable) |
| A team already curates a plugin, or Enable found a `claude-data` plugin | 2 |
| Every certified query must go through code review | 3, where open |

Where route 3 is open, routes 1 and 2 can later move to it: export the plugin, commit it, sync it as a marketplace,
re-point the setting.

## Questions people ask

**Who can press Enable, or change the choice in admin settings?** Enable: whoever may manage the organization's
plugin library, which is the Owner and Primary owner roles, and on Enterprise any custom role with "Libraries: Manage".
The connector checks that right before it tries, and each step is checked again by the library itself. The choice in
Admin settings › Artifacts › Dashboards also needs the right to manage the organization, so a custom role needs that
permission as well as "Libraries: Manage" to see it.

**Who can certify a query once it's on?** Whoever may publish to the marketplace the chosen plugin lives in. For
the organization's own library that is the same people as above (Owners, and custom roles with Libraries: Manage);
for a marketplace the organization added, also anyone made a manager of that marketplace. There is no
per-plugin permission to grant: to let someone certify, give them one of those roles. Everyone else sees the
switch held off with "You don’t have permission to certify queries", naming the plugin so they know what to ask
for access to.

**Who sees certified queries?** Every member the plugin is available to. Enable makes it available to everyone;
an admin can narrow that per group in Admin settings › Plugins. A member it isn't available to sees no checks on
a dashboard and Claude doesn't use the queries for them. If Enable could make the plugin but not change who sees
it, the dashboard says so and the admin finishes that one step in admin settings.

**Does certifying run anything or grant data access?** No. A certified query is text: the call and its
description. Whoever uses it later runs it on their own connector with their own access, exactly as an
uncertified query runs.

**Can we have more than one plugin?** One at a time per organization. Changing the choice doesn't delete the old
plugin or its files; dashboards simply stop showing its checks.

**Why is the switch off for me?** While nothing is set up there is no switch, only the box above (routes above).
Otherwise the tooltip says which of these it is: kept in a repository (route 3: edit the repo), no permission (ask
for one of the roles above), or can't be reached right now (the connector or the library is down; try again, or open
the dashboard inside Claude rather than on its own). Where the lookup itself failed there is no tooltip: the sentence
under the switch says why, and the query's page in Sources ends "Couldn’t check certification" with the same reason behind
its info mark. A Retry comes with it only for a passing fault; none comes where their organization's or their own connector
settings don't allow it, or where it is turned off for this dashboard. Where Claude Data isn't listed for them, nothing of certified queries
shows at all: the platform lists it where the organization allows Live sharing or Certified queries, and an admin's connector settings
can also keep it from a member.

**How do we turn it off?** Where Admin settings › Artifacts › Dashboards offers a Certified queries switch, turn
it off: dashboards then show nothing of certification and nothing can be certified, and turning it back on brings
everything back, the chosen plugin included. Before an organization has that switch, the page has no off: the
plugin choice can be changed to another plugin there, but not cleared.
