// Exercises the real API. Opt-in:
//   POLLBOT_ID=... POLLBOT_PASS=... node tests/live.js
// Read-only apart from the login POST that mints a token.
const { load, check, section, report } = require('./harness.js');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const ID = process.env.POLLBOT_ID, PASS = process.env.POLLBOT_PASS;
if (!ID || !PASS) {
  console.log('skipped: set POLLBOT_ID and POLLBOT_PASS to run the live suite');
  process.exit(0);
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'pollbot-'));
const store = { credId: ID, credPass: PASS };
let wroteTable = null;
const messages = [];
const sheets = ['WEB: RAW DATA', 'WEB: LAST MESSAGE'];
let installedInterval = null;

const S = load();
S.PropertiesService = { getScriptProperties: () => ({
  getProperty: k => (k in store ? store[k] : null),
  setProperty: (k, v) => { store[k] = String(v); }
}) };
S.SpreadsheetApp = { getActiveSpreadsheet: () => ({
  getSheetByName: n => (sheets.includes(n) ? {} : null),
  insertSheet: n => { sheets.push(n); return {}; }
}) };
S.writeTable_ = t => { wroteTable = t; };
S.writeMessage_ = (status, text) => { messages.push([status, text]); };
S.installTrigger_ = m => { installedInterval = m; };

// A UrlFetchApp-shaped wrapper around curl.
S.UrlFetchApp = { fetch(url, params) {
  const head = path.join(tmp, 'h'), body = path.join(tmp, 'b');
  const args = ['-s', '-D', head, '-o', body, '-X',
    (params.method || 'get').toUpperCase(), '--max-time', '40', url];
  Object.keys(params.headers || {}).forEach(k => args.push('-H', `${k}: ${params.headers[k]}`));
  if (params.payload) args.push('-H', 'Content-Type: application/json', '-d', params.payload);
  execFileSync('curl', args);
  const raw = fs.readFileSync(head, 'utf8');
  const code = Number(raw.match(/HTTP\/[\d.]+ (\d+)/g).pop().split(' ')[1]);
  const headers = {};
  raw.split('\n').forEach(l => {
    const m = /^([A-Za-z0-9-]+):\s*(.*)$/.exec(l.trim());
    if (m) headers[m[1]] = m[2];
  });
  return {
    getResponseCode: () => code,
    getContentText: () => fs.readFileSync(body, 'utf8'),
    getAllHeaders: () => headers
  };
} };

section('full fetch against the live API');
const count = S.runFetch_();
check('returned a row count', count > 0, true);
check('wrote a table', wroteTable !== null, true);
check('header shape', wroteTable.header.slice(0, 3), ['Date', 'Topic', 'Total']);
check('ID column dropped', wroteTable.header.indexOf('ID'), -1);
check('Active column dropped', wroteTable.header.indexOf('Active'), -1);
check('terminator dropped', wroteTable.header.indexOf('<END>'), -1);
check('rows uniform', wroteTable.rows.every(r => r.length === wroteTable.header.length), true);
console.log(`     ${count} rows x ${wroteTable.header.length} cols`);

section('failure reporting');
store.credPass = 'WrongP@ss1';
try {
  S.runFetch_();
  check('bad password should abort', 'no throw', 'throw');
} catch (err) {
  check('surfaces the server reason', S.userMessage_(err),
    'POST /login/json -> HTTP 401: unknown credentials');
}

delete store.credId; delete store.credPass;
try {
  S.runFetch_();
  check('missing credentials should abort', 'no throw', 'throw');
} catch (err) {
  check('points at Change Credentials', /No credentials stored/.test(S.userMessage_(err)), true);
}

messages.length = 0;
S.autofetchTick();
check('the trigger records rather than throws', messages.length, 1);
check('  logged as ERROR', messages[0][0], 'ERROR');
check('  with the reason', /No credentials stored/.test(messages[0][1]), true);

section('credential dialog');
check('blank input rejected', S.dialogSubmit('credentials', { id: '', password: '' }).ok, false);
const bad = S.dialogSubmit('credentials', { id: ID, password: 'WrongP@ss1' });
check('wrong password rejected', bad.ok, false);
check('  with the server message', /unknown credentials/.test(bad.message), true);
check('  and nothing persisted', store.credId, undefined);
check('valid pair accepted', S.dialogSubmit('credentials', { id: ID, password: PASS }).ok, true);
check('  persisted only after the server agreed', store.credId, ID);

fs.rmSync(tmp, { recursive: true, force: true });
report();
