// Logic tests that need no network. Fixtures are real /polls responses.
const { load, check, throws, section, report, fixture } = require('./harness.js');
const S = load();

section('interval flooring');
[[5, 5], [7, 5], [15, 15], [20, 15], [30, 30], [45, 30], [59.9, 30], [60, 60],
 [1000, 60], [0, 5], [-5, 5]].forEach(([input, want]) =>
  check(`${input} -> ${want}`, S.floorInterval_(input), want));
check('non-numeric falls back to the default', S.floorInterval_('abc'), S.DEFAULT_INTERVAL);

section('snowflake ordering');
check('sorts as integers, not strings',
  ['1528825060595597464', '1527278532177297579', '999', '1527710716256325792']
    .sort(S.compareSnowflake_),
  ['999', '1527278532177297579', '1527710716256325792', '1528825060595597464']);
const a = '1527278532177297579', b = '1527278532177297580';
check('distinguishes ids that collide as doubles', S.compareSnowflake_(a, b) < 0, true);
check('  (Number() really does tie them)', Number(a) === Number(b), true);

section('csv parsing');
const p1 = S.parseCsv_(fixture('polls-page-1.csv'));
const p2 = S.parseCsv_(fixture('polls-page-2.csv'));
check('terminator stripped from header', p1[0][p1[0].length - 1] !== '<END>', true);
check('leading columns as documented', p1[0].slice(0, 5), S.FIXED_COLUMNS);
check('page 1 rows are uniform', p1.slice(1).every(r => r.length === p1[0].length), true);
check('page 2 rows are uniform', p2.slice(1).every(r => r.length === p2[0].length), true);
check('CRLF and blank lines tolerated',
  S.parseCsv_('ID, Date, Active, Topic, Total, Ann, <END>\r\n\r\n')[0].length, 6);

section('merging real pages');
const merged = S.mergePages_([p1, p2]);
check('ID and Active dropped', merged.header.slice(0, 3), ['Date', 'Topic', 'Total']);
check('no terminator column', merged.header.indexOf('<END>'), -1);
const union = [...new Set([...p1[0].slice(5), ...p2[0].slice(5)])].sort();
check('voter columns are the union of both pages', merged.header.slice(3), union);
check('page 1 alone did not have them all', p1[0].slice(5).length < union.length, true);
check('rows are uniform', merged.rows.every(r => r.length === merged.header.length), true);
check('row count is the distinct id count', merged.rows.length,
  new Set([...p1.slice(1), ...p2.slice(1)].map(r => r[0])).size);
check('dates became Date objects', merged.rows.every(r => r[0] instanceof Date), true);
check('totals became numbers', merged.rows.every(r => typeof r[2] === 'number'), true);
check('sorted ascending',
  merged.rows.every((r, i) => i === 0 || merged.rows[i - 1][0] <= r[0]), true);

// Re-derive one row straight from the raw CSV and compare.
const raw = p2[1];
const byName = {};
p2[0].slice(5).forEach((name, i) => { byName[name] = raw[5 + i]; });
const row = merged.rows.find(r => r[1] === raw[3]);
check('votes re-align to the merged columns', row.slice(3),
  merged.header.slice(3).map(v =>
    byName[v] === undefined ? 'N/A' : (/^-?\d+$/.test(byName[v]) ? Number(byName[v]) : byName[v])));
check('topic preserved', row[1], raw[3]);
check('total preserved', row[2], Number(raw[4]));

section('merge edge cases');
const H = 'ID, Date, Active, Topic, Total, Ann, Bob, <END>';
const mk = (id, d, act, t, tot, x, y) => `${id}, ${d}, ${act}, ${t}, ${tot}, ${x}, ${y}, <END>`;
const ID1 = '1527278532177297579', ID2 = '1527710716256325792', ID3 = '1528825060595597464';

check('no pages -> header only', S.mergePages_([]), { header: ['Date', 'Topic', 'Total'], rows: [] });
check('header-only page -> no rows', S.mergePages_([S.parseCsv_(H)]).rows.length, 0);

const dup = S.mergePages_([
  S.parseCsv_(H + '\n' + mk(ID1, '07/16/2026', 1, 'Pizza', 2, '5', '4')),
  S.parseCsv_(H + '\n' + mk(ID1, '07/16/2026', 1, 'Pizza', 9, '1', '2'))]);
check('a poll repeated across pages collapses', dup.rows.length, 1);
check('  and the later copy wins', dup.rows[0][2], 9);

const inactive = S.mergePages_([S.parseCsv_(H + '\n' + mk(ID1, '07/16/2026', 0, 'Old', 3, '3', 'N/A'))]);
check('inactive rows are kept, not filtered', inactive.rows.length, 1);
check('  Active is simply not a column', inactive.header.indexOf('Active'), -1);

throws('a short row is refused rather than misaligned',
  () => S.mergePages_([S.parseCsv_(H + '\n' + ID1 + ', 07/16/2026, 1, Pizza, 2, 5, <END>')]),
  /Malformed CSV row/);

const disjoint = S.mergePages_([
  S.parseCsv_(H + '\n' + mk(ID1, '07/16/2026', 1, 'A', 2, '5', '4')),
  S.parseCsv_('ID, Date, Active, Topic, Total, Cid, <END>\n' + ID2 + ', 07/17/2026, 1, B, 1, 3, <END>')]);
check('disjoint voter sets unite', disjoint.header, ['Date', 'Topic', 'Total', 'Ann', 'Bob', 'Cid']);
check('  absent voter reads N/A', disjoint.rows[0][5], 'N/A');
check('  and the other way round', disjoint.rows[1].slice(3, 5), ['N/A', 'N/A']);

check('out-of-order rows are sorted',
  S.mergePages_([S.parseCsv_([H, mk(ID3, '07/20/2026', 1, 'Later', 1, '1', 'N/A'),
    mk(ID1, '07/16/2026', 1, 'Earlier', 1, '2', 'N/A')].join('\n'))]).rows.map(r => r[1]),
  ['Earlier', 'Later']);
check('same-day polls break the tie on id',
  S.mergePages_([S.parseCsv_([H, mk('1527278532177297580', '07/16/2026', 1, 'Second', 1, '1', 'N/A'),
    mk(ID1, '07/16/2026', 1, 'First', 1, '2', 'N/A')].join('\n'))]).rows.map(r => r[1]),
  ['First', 'Second']);
check('titles keep their punctuation',
  S.mergePages_([S.parseCsv_(H + '\n' + mk(ID1, '07/16/2026', 1, 'Pickles (in general)', 1, '3', 'N/A'))])
    .rows[0][1], 'Pickles (in general)');

section('cell coercion');
check('date is MM/DD, not DD/MM', S.parseDate_('07/16/2026').getDate(), 16);
check('  month is July', S.parseDate_('07/16/2026').getMonth(), 6);
check('unparseable date stays text', S.parseDate_('not a date'), 'not a date');
check('N/A stays text', S.toNumberOrText_('N/A'), 'N/A');
check('a rating becomes a number', S.toNumberOrText_('5'), 5);

section('settings dialogs');
{
  // Fresh context so the stubs below cannot leak into the tests above.
  const T = load();
  const store = {};
  const sheets = ['WEB: RAW DATA', 'WEB: LAST MESSAGE'];
  let installedInterval = null;

  T.PropertiesService = { getScriptProperties: () => ({
    getProperty: k => (k in store ? store[k] : null),
    setProperty: (k, v) => { store[k] = String(v); }
  }) };
  T.SpreadsheetApp = { getActiveSpreadsheet: () => ({
    getSheetByName: n => (sheets.includes(n) ? {} : null),
    insertSheet: n => { sheets.push(n); return {}; }
  }) };
  T.installTrigger_ = m => { installedInterval = m; };

  store.autofetchEnabled = 'false';
  [[5, 5], [7, 5], [20, 15], [45, 30], [60, 60], [90, 60], [1, 5]].forEach(([input, want]) => {
    const result = T.dialogSubmit('interval', { minutes: input });
    check(`interval ${input} stores ${want}`, Number(store.fetchInterval), want);
    check(`  and reports success`, result.ok, true);
  });
  check('a snapped value says so',
    /adjusted from 45/.test(T.dialogSubmit('interval', { minutes: 45 }).message), true);
  check('an exact value does not',
    /adjusted/.test(T.dialogSubmit('interval', { minutes: 30 }).message), false);
  check('non-numeric rejected', T.dialogSubmit('interval', { minutes: 'abc' }).ok, false);

  store.autofetchEnabled = 'true';
  installedInterval = null;
  T.dialogSubmit('interval', { minutes: 10 });
  check('a running autofetch is rescheduled immediately', installedInterval, 10);

  check('blank tab name rejected', T.dialogSubmit('sheetData', { name: '  ' }).ok, false);
  check('data and message may not share a tab',
    T.dialogSubmit('sheetData', { name: 'WEB: LAST MESSAGE' }).ok, false);
  const fresh = T.dialogSubmit('sheetData', { name: 'Fresh Tab' });
  check('a new tab is accepted', fresh.ok, true);
  check('  and reported as created', /new tab created/.test(fresh.message), true);
  check('  and actually created', sheets.indexOf('Fresh Tab') !== -1, true);
  check('  and honoured afterwards', T.rawSheetName_(), 'Fresh Tab');
  check('an existing tab is not announced as new',
    /new tab created/.test(T.dialogSubmit('sheetMessage', { name: 'WEB: LAST MESSAGE' }).message), false);
  check('unknown form is handled, not thrown', T.dialogSubmit('nope', {}).ok, false);
}

report();
