# Poll Bot -> Google Sheets

An Apps Script that mirrors finalized polls from the poll-bot REST API into a
Google Sheet, on demand or on a schedule.

It is read-only against the API. The only non-GET call is the login POST that
exchanges credentials for a bearer token.

## Files

| File | Purpose |
| --- | --- |
| `Code.gs` | Everything: menu, fetch, CSV merge, triggers, settings |
| `Dialog.html` | The modal input forms |
| `appsscript.json` | Manifest and OAuth scopes |
| `tests/` | Node test suites (not deployed; see `.claspignore`) |

## Install

With [clasp](https://github.com/google/clasp), from this directory:

```sh
clasp create --type sheets --title "Poll Bot"   # or: clasp clone <scriptId>
clasp push
```

By hand: open the sheet, **Extensions > Apps Script**, then create a script file
named `Code` and an HTML file named `Dialog`, paste the matching contents, and
paste `appsscript.json` over the manifest (enable *Show "appsscript.json"* in
project settings first).

Then reload the spreadsheet. A **Poll Bot** menu appears. The first action you
take will ask for authorization.

## Use

```
Poll Bot
├── Autofetch
│   ├── Start (stopped) / Stop (running)
│   └── Fetch interval
├── Fetch Now
├── Change Credentials
├── Output Sheets
│   ├── Data
│   └── Message
└── Help
```

Set credentials first — nothing fetches without them. Starting autofetch also
fetches immediately, and Fetch Now resets the countdown.

The menu label reflects autofetch state as of when the sheet was opened;
reload to refresh it after toggling.

## Configuration

The two default output tabs are top-level variables at the head of `Code.gs`:

```js
var SHEET_RAW = 'WEB: RAW DATA';
var SHEET_MESSAGE = 'WEB: LAST MESSAGE';
```

Output Sheets overrides them at runtime without editing the script. Both tabs
are created if missing, and they may not be the same tab — a fetch clears the
data tab wholesale, which would erase the message log every run.

**Interval.** Apps Script schedules recurring triggers only at 1/5/10/15/30
minutes or hourly, so the interval is restricted to **5, 10, 15, 30, 60**.
Whatever you type is floored into that set and clamped to `[5, 60]`; the dialog
reports the value it settled on.

**Credentials** live in script properties, shared by everyone who can edit the
sheet. Apps Script has no encrypted store — properties are access-controlled,
not encrypted, so anyone who can open the script editor can read them. They are
never displayed back in the UI, and the Change Credentials dialog always opens
blank. A pair is only saved after the server accepts it: `POST /login/json` for
a token, then `GET /login/check` with that token as `Authorization: Bearer`.
This cannot create an account; the pair must already exist.

## Output

The data tab is rewritten in full on every fetch:

| Date | Topic | Total | *(one column per voter)* |
| --- | --- | --- | --- |
| 07/16/2026 | Chick-fil-A | 6 | 4, N/A, 3, ... |

The API's `ID`, `Active`, and `<END>` columns are dropped per spec. Rows are
sorted by poll snowflake, which orders same-day polls correctly where the
day-resolution date cannot.

`/polls` pages 50 records at a time, and **each page's CSV header lists only the
voters appearing on that page** — so pages have different column sets and cannot
be stacked. The script unions the voter columns across all pages and re-aligns
every row against that union, filling absences with `N/A`. Merged voter columns
come out alphabetical: the server orders them by voter snowflake, which the
export does not expose, so that order cannot be reproduced across a merge.

The message tab records the outcome of the last API interaction:

```
Timestamp   2026-08-25 13:40:02 UTC
Status      OK
Message     Autofetch wrote 72 poll row(s) to "WEB: RAW DATA".
```

Every non-2xx response lands here with the server's own reason, which is the
only durable trace a headless autofetch run leaves. A failed fetch aborts the
whole run, so a partial page set never overwrites a complete one.

## Tests

```sh
node tests/offline.js                                     # logic, no network
POLLBOT_ID=... POLLBOT_PASS=... node tests/live.js         # against the real API
```

`Code.gs` is plain JS that only touches Apps Script services from inside
functions, so the harness runs the real file in a `vm` context and stubs the
services per test. Fixtures under `tests/fixtures/` are captured `/polls`
responses, kept because pages 1 and 2 differ in their voter columns — the case
the merge exists to handle.
