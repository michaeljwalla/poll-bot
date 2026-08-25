/**
 * Poll Bot -> Google Sheets bridge.
 *
 * Pulls finalized polls from the poll-bot REST API and mirrors them into this
 * spreadsheet. Read-only against the API: the only non-GET call is the login
 * POST that exchanges credentials for a bearer token.
 */

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// Default output tabs. Both are overridable at runtime from Output Sheets;
// these are the values used until someone overrides them.
var SHEET_RAW = 'WEB: RAW DATA';
var SHEET_MESSAGE = 'WEB: LAST MESSAGE';

var API_ROOT = 'https://pb.wcl-d.net/api/v1';

// Apps Script's everyMinutes() only accepts 1/5/10/15/30, and everyHours(1)
// covers 60. Restricting the interval to this set means whatever the user
// picks is always installable as a plain recurring trigger.
var ALLOWED_INTERVALS = [5, 10, 15, 30, 60];
var DEFAULT_INTERVAL = 15;

// The API pages 50 polls at a time and signals the end itself; this only stops
// a malformed X-Next-Page from looping until the 6-minute execution cap.
var MAX_PAGES = 500;

// Leading columns the exporter always emits, in order, before the voter
// columns. See root/csv/csv.go.
var FIXED_COLUMNS = ['ID', 'Date', 'Active', 'Topic', 'Total'];
// Dropped before anything reaches the sheet, per spec.
var DROP_COLUMNS = ['ID', 'Active'];
// Every emitted row is terminated by this sentinel.
var ROW_TERMINATOR = '<END>';

var PROP = {
  ID: 'credId',
  PASS: 'credPass',
  INTERVAL: 'fetchInterval',
  ENABLED: 'autofetchEnabled',
  SHEET_RAW: 'sheetRaw',
  SHEET_MESSAGE: 'sheetMessage'
};

var TRIGGER_HANDLER = 'autofetchTick';

// ---------------------------------------------------------------------------
// Stored settings
// ---------------------------------------------------------------------------

function props_() {
  return PropertiesService.getScriptProperties();
}

/** Floors an arbitrary minute count into ALLOWED_INTERVALS, clamped to range. */
function floorInterval_(value) {
  var n = Math.floor(Number(value));
  if (!isFinite(n)) {
    return DEFAULT_INTERVAL;
  }
  var best = ALLOWED_INTERVALS[0];
  for (var i = 0; i < ALLOWED_INTERVALS.length; i++) {
    if (ALLOWED_INTERVALS[i] <= n) {
      best = ALLOWED_INTERVALS[i];
    }
  }
  return best;
}

function getInterval_() {
  return floorInterval_(props_().getProperty(PROP.INTERVAL) || DEFAULT_INTERVAL);
}

function rawSheetName_() {
  return props_().getProperty(PROP.SHEET_RAW) || SHEET_RAW;
}

function messageSheetName_() {
  return props_().getProperty(PROP.SHEET_MESSAGE) || SHEET_MESSAGE;
}

function getCredentials_() {
  var p = props_();
  var id = p.getProperty(PROP.ID);
  var password = p.getProperty(PROP.PASS);
  if (!id || !password) {
    return null;
  }
  return { id: id, password: password };
}

// ---------------------------------------------------------------------------
// Sheet access
// ---------------------------------------------------------------------------

function sheetByName_(name) {
  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var sheet = ss.getSheetByName(name);
  if (!sheet) {
    sheet = ss.insertSheet(name);
  }
  return sheet;
}

/**
 * Records the outcome of the last API interaction. Every non-2xx response ends
 * up here, which is the one durable trace a headless autofetch run leaves.
 * Best-effort: a failure to write the log must not replace the real error.
 */
function writeMessage_(status, text) {
  try {
    var sheet = sheetByName_(messageSheetName_());
    var stamp = Utilities.formatDate(
      new Date(), Session.getScriptTimeZone(), 'yyyy-MM-dd HH:mm:ss z');
    sheet.clear();
    sheet.getRange(1, 1, 3, 2).setValues([
      ['Timestamp', stamp],
      ['Status', status],
      ['Message', text]
    ]);
    sheet.getRange(1, 1, 3, 1).setFontWeight('bold');
    sheet.getRange(1, 2, 3, 1).setWrap(true);
    sheet.autoResizeColumn(1);
  } catch (err) {
    console.error('could not write to the message sheet: ' + err);
  }
}

/** The message a user should see for an error, however it was raised. */
function userMessage_(err) {
  if (!err) {
    return 'unknown error';
  }
  return err.userMessage || err.message || String(err);
}

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

function safeJson_(text) {
  try {
    return JSON.parse(text);
  } catch (err) {
    return null;
  }
}

/** Header lookup that tolerates whatever casing the proxy hands back. */
function header_(response, name) {
  var headers = response.getAllHeaders();
  var wanted = name.toLowerCase();
  for (var key in headers) {
    if (key.toLowerCase() === wanted) {
      var value = headers[key];
      // a repeated header arrives as an array
      return Array.isArray(value) ? value[0] : value;
    }
  }
  return null;
}

/** The API reports failures as {code, error, message}; fall back to raw text. */
function apiMessage_(response) {
  var text = response.getContentText();
  var body = safeJson_(text);
  if (body && (body.message || body.error)) {
    return body.message || body.error;
  }
  text = String(text || '').replace(/\s+/g, ' ').trim();
  return text.length > 300 ? text.slice(0, 300) + '...' : text;
}

function apiError_(what, code, message) {
  var label = code ? what + ' -> HTTP ' + code : what + ' -> request failed';
  var err = new Error(label + (message ? ': ' + message : ''));
  err.userMessage = err.message;
  err.code = code;
  return err;
}

/**
 * Single entry point for every API call. Non-2xx always throws, so no caller
 * can accidentally treat an error body as data.
 */
function request_(method, path, options) {
  options = options || {};
  var params = {
    method: method,
    muteHttpExceptions: true,
    followRedirects: true,
    validateHttpsCertificates: true
  };
  if (options.token) {
    params.headers = { Authorization: 'Bearer ' + options.token };
  }
  if (options.payload) {
    params.contentType = 'application/json';
    params.payload = JSON.stringify(options.payload);
  }

  var what = method.toUpperCase() + ' ' + path;
  var response;
  try {
    response = UrlFetchApp.fetch(API_ROOT + path, params);
  } catch (err) {
    // The request never completed, so there is no status to report. A lossy
    // link shows up here; the spec says stop and say why rather than retry.
    throw apiError_(what, 0, userMessage_(err));
  }

  var code = response.getResponseCode();
  if (code < 200 || code >= 300) {
    throw apiError_(what, code, apiMessage_(response));
  }
  return response;
}

/** Exchanges credentials for a bearer token. */
function fetchToken_(id, password) {
  var response = request_('post', '/login/json', {
    payload: { id: id, password: password }
  });
  var body = safeJson_(response.getContentText());
  if (!body || !body.token) {
    throw apiError_('POST /login/json', response.getResponseCode(),
      'the response contained no token');
  }
  return body.token;
}

/** Confirms the token is accepted; throws with the server's reason if not. */
function verifyToken_(token) {
  request_('get', '/login/check', { token: token });
}

/** Logs in with the stored credentials. */
function authorize_() {
  var cred = getCredentials_();
  if (!cred) {
    var err = new Error(
      'No credentials stored. Set them under Poll Bot > Change Credentials.');
    err.userMessage = err.message;
    throw err;
  }
  return fetchToken_(cred.id, cred.password);
}

// ---------------------------------------------------------------------------
// CSV
// ---------------------------------------------------------------------------

/**
 * The exporter writes plain ", "-joined fields with no quoting, strips commas
 * out of poll titles, and terminates every row with <END>. So a naive split is
 * correct, and the terminator doubles as a check that the row is intact.
 */
function parseCsv_(text) {
  var lines = String(text).split('\n');
  var rows = [];
  for (var i = 0; i < lines.length; i++) {
    var line = lines[i].replace(/\r$/, '');
    if (line.trim() === '') {
      continue;
    }
    var cells = line.split(',');
    for (var j = 0; j < cells.length; j++) {
      cells[j] = cells[j].trim();
    }
    if (cells.length && cells[cells.length - 1] === ROW_TERMINATOR) {
      cells.pop();
    }
    rows.push(cells);
  }
  return rows;
}

/**
 * Snowflakes overflow a double, so they are compared as digit strings: longer
 * is larger, and equal lengths compare lexicographically.
 */
function compareSnowflake_(a, b) {
  a = String(a);
  b = String(b);
  if (a.length !== b.length) {
    return a.length - b.length;
  }
  return a < b ? -1 : (a > b ? 1 : 0);
}

/** "07/16/2026" -> a real Date, so the sheet sorts and formats it as one. */
function parseDate_(value) {
  var match = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(String(value).trim());
  if (!match) {
    // Anything unexpected stays text rather than being guessed at.
    return value;
  }
  return new Date(Number(match[3]), Number(match[1]) - 1, Number(match[2]));
}

function toNumberOrText_(value) {
  var text = String(value).trim();
  if (text === '' || !/^-?\d+(\.\d+)?$/.test(text)) {
    return value;
  }
  return Number(text);
}

/**
 * Each page's CSV header only lists the voters who appear on that page, so the
 * pages have different column sets and cannot simply be stacked. This unions
 * the voter columns across every page and re-aligns each row against that
 * union, filling absences with the same "N/A" the exporter itself uses.
 *
 * Voter columns come out alphabetical. The server orders them by voter
 * snowflake, which is not visible in the export and so cannot be reproduced
 * across a merge; alphabetical is at least stable and readable.
 */
function mergePages_(pages) {
  var voterSet = {};
  var byId = {};
  var order = [];

  for (var p = 0; p < pages.length; p++) {
    var rows = pages[p];
    if (!rows.length) {
      continue;
    }
    var head = rows[0];
    if (head.length < FIXED_COLUMNS.length) {
      throw new Error('Unexpected CSV header on page ' + (p + 1) + ': ' + head.join(', '));
    }
    var voters = head.slice(FIXED_COLUMNS.length);
    for (var v = 0; v < voters.length; v++) {
      voterSet[voters[v]] = true;
    }

    for (var r = 1; r < rows.length; r++) {
      var cells = rows[r];
      if (cells.length !== head.length) {
        // Misalignment here would silently attribute votes to the wrong
        // person, so refuse the page rather than write a plausible lie.
        throw new Error(
          'Malformed CSV row on page ' + (p + 1) + ': expected ' + head.length +
          ' fields, got ' + cells.length + '.');
      }
      var meta = {};
      for (var f = 0; f < FIXED_COLUMNS.length; f++) {
        meta[FIXED_COLUMNS[f]] = cells[f];
      }
      var votes = {};
      for (var k = 0; k < voters.length; k++) {
        votes[voters[k]] = cells[FIXED_COLUMNS.length + k];
      }
      var id = meta.ID;
      // Paging is offset-based, so a poll finalized mid-fetch can shift a row
      // across a page boundary and repeat it. Last copy wins.
      if (!(id in byId)) {
        order.push(id);
      }
      byId[id] = { meta: meta, votes: votes };
    }
  }

  var voterList = Object.keys(voterSet).sort();
  order.sort(compareSnowflake_);

  var keep = [];
  for (var c = 0; c < FIXED_COLUMNS.length; c++) {
    if (DROP_COLUMNS.indexOf(FIXED_COLUMNS[c]) === -1) {
      keep.push(FIXED_COLUMNS[c]);
    }
  }

  var header = keep.concat(voterList);
  var out = [];
  for (var i = 0; i < order.length; i++) {
    var rec = byId[order[i]];
    var row = [];
    for (var m = 0; m < keep.length; m++) {
      var name = keep[m];
      var raw = rec.meta[name];
      if (name === 'Date') {
        row.push(parseDate_(raw));
      } else if (name === 'Total') {
        row.push(toNumberOrText_(raw));
      } else {
        row.push(raw);
      }
    }
    for (var n = 0; n < voterList.length; n++) {
      var vote = rec.votes[voterList[n]];
      row.push(vote === undefined ? 'N/A' : toNumberOrText_(vote));
    }
    out.push(row);
  }

  return { header: header, rows: out };
}

function writeTable_(table) {
  var name = rawSheetName_();
  if (name === messageSheetName_()) {
    throw new Error(
      'The data and message sheets are both set to "' + name +
      '". Point them at different tabs before fetching.');
  }
  var sheet = sheetByName_(name);
  var values = [table.header].concat(table.rows);

  sheet.clear();
  sheet.getRange(1, 1, values.length, table.header.length).setValues(values);
  sheet.getRange(1, 1, 1, table.header.length).setFontWeight('bold');
  if (table.rows.length) {
    sheet.getRange(2, 1, table.rows.length, 1).setNumberFormat('MM/dd/yyyy');
  }
  sheet.setFrozenRows(1);
}

// ---------------------------------------------------------------------------
// Fetch
// ---------------------------------------------------------------------------

/**
 * Walks /polls from page 1 until the API says to stop, then rewrites the data
 * sheet. Returns the number of poll rows written. Any non-2xx aborts the whole
 * run, so a partial page set never overwrites a complete one.
 */
function runFetch_() {
  var token = authorize_();
  var pages = [];

  for (var page = 1; page <= MAX_PAGES; page++) {
    var response = request_('get', '/polls?page=' + page, { token: token });
    if (response.getResponseCode() === 204) {
      break; // past the end
    }
    var text = response.getContentText();
    if (text && text.trim() !== '') {
      pages.push(parseCsv_(text));
    }
    var next = header_(response, 'X-Next-Page');
    if (next === null || String(next).trim() === '' || String(next).trim() === '0') {
      break;
    }
    if (page === MAX_PAGES) {
      throw new Error('Stopped after ' + MAX_PAGES +
        ' pages without the API signalling an end.');
    }
  }

  var table = mergePages_(pages);
  writeTable_(table);
  return table.rows.length;
}

/** Installed time-based trigger. Errors are reported, never rethrown, so a
 *  lossy fetch does not generate a failure mail on every interval. */
function autofetchTick() {
  try {
    var count = runFetch_();
    writeMessage_('OK', 'Autofetch wrote ' + count + ' poll row(s) to "' +
      rawSheetName_() + '".');
  } catch (err) {
    writeMessage_('ERROR', userMessage_(err));
  }
}

// ---------------------------------------------------------------------------
// Triggers
// ---------------------------------------------------------------------------

function autofetchTriggers_() {
  var found = [];
  var all = ScriptApp.getProjectTriggers();
  for (var i = 0; i < all.length; i++) {
    if (all[i].getHandlerFunction() === TRIGGER_HANDLER) {
      found.push(all[i]);
    }
  }
  return found;
}

function removeTriggers_() {
  var triggers = autofetchTriggers_();
  for (var i = 0; i < triggers.length; i++) {
    ScriptApp.deleteTrigger(triggers[i]);
  }
}

/** Recreating the trigger is also what resets the countdown. */
function installTrigger_(minutes) {
  removeTriggers_();
  var builder = ScriptApp.newTrigger(TRIGGER_HANDLER).timeBased();
  if (minutes >= 60) {
    builder.everyHours(1);
  } else {
    builder.everyMinutes(minutes);
  }
  builder.create();
}

/**
 * The menu label is built by a simple onOpen trigger, which may not be allowed
 * to enumerate triggers, so the flag is the thing onOpen reads.
 */
function isAutofetchOn_() {
  return props_().getProperty(PROP.ENABLED) === 'true';
}

function startAutofetch_(minutes) {
  installTrigger_(minutes);
  props_().setProperty(PROP.ENABLED, 'true');
}

function stopAutofetch_() {
  removeTriggers_();
  props_().setProperty(PROP.ENABLED, 'false');
}

// ---------------------------------------------------------------------------
// Menu
// ---------------------------------------------------------------------------

function onOpen(e) {
  var ui = SpreadsheetApp.getUi();

  var label;
  try {
    label = isAutofetchOn_() ? 'Stop (running)' : 'Start (stopped)';
  } catch (err) {
    // Before the project is authorized, onOpen runs in limited auth mode and
    // may not read properties. Only the label degrades; the handler behind it
    // runs with full authorization when clicked.
    label = 'Start / Stop';
  }

  var autofetch = ui.createMenu('Autofetch')
    .addItem(label, 'menuToggleAutofetch')
    .addItem('Fetch interval', 'menuFetchInterval');

  var outputs = ui.createMenu('Output Sheets')
    .addItem('Data', 'menuOutputData')
    .addItem('Message', 'menuOutputMessage');

  ui.createMenu('Poll Bot')
    .addSubMenu(autofetch)
    .addItem('Fetch Now', 'menuFetchNow')
    .addSeparator()
    .addItem('Change Credentials', 'menuChangeCredentials')
    .addSubMenu(outputs)
    .addSeparator()
    .addItem('Help', 'menuHelp')
    .addToUi();
}

function menuToggleAutofetch() {
  var ui = SpreadsheetApp.getUi();

  // Trust the trigger list over the flag: if they ever disagree, an orphaned
  // trigger is the one that keeps firing.
  if (isAutofetchOn_() || autofetchTriggers_().length) {
    stopAutofetch_();
    ui.alert('Autofetch',
      'Autofetch stopped.\n\nReopen the Poll Bot menu to refresh its label.',
      ui.ButtonSet.OK);
    return;
  }

  var minutes = getInterval_();
  startAutofetch_(minutes);

  var summary;
  try {
    var count = runFetch_(); // "starts immediately"
    writeMessage_('OK', 'Autofetch started (every ' + minutes +
      ' min). First fetch wrote ' + count + ' poll row(s).');
    summary = 'Autofetch started, running every ' + minutes + ' minutes.\n\n' +
      'The first fetch wrote ' + count + ' poll row(s).';
  } catch (err) {
    var message = userMessage_(err);
    writeMessage_('ERROR', message);
    summary = 'Autofetch started, running every ' + minutes + ' minutes.\n\n' +
      'The first fetch failed:\n' + message;
  }
  ui.alert('Autofetch',
    summary + '\n\nReopen the Poll Bot menu to refresh its label.',
    ui.ButtonSet.OK);
}

function menuFetchNow() {
  var ui = SpreadsheetApp.getUi();
  try {
    var count = runFetch_();
    if (isAutofetchOn_()) {
      installTrigger_(getInterval_()); // resets the countdown
    }
    writeMessage_('OK', 'Manual fetch wrote ' + count + ' poll row(s) to "' +
      rawSheetName_() + '".');
    ui.alert('Fetch Now',
      'Wrote ' + count + ' poll row(s) to "' + rawSheetName_() + '".',
      ui.ButtonSet.OK);
  } catch (err) {
    var message = userMessage_(err);
    writeMessage_('ERROR', message);
    ui.alert('Fetch Now failed', message, ui.ButtonSet.OK);
  }
}

function menuHelp() {
  var text = [
    'Autofetch',
    '- routinely fetches content on the given interval. Current Interval: ' +
      getInterval_(),
    '',
    'Fetch Now',
    '- reset the autofetch countdown and fetch immediately',
    '',
    'Change Credentials',
    '- set userid/passfield here. Otherwise, fetching will not work',
    '',
    'Output Sheets',
    '- Point to where data should be outputted.',
    '- If you accidentally overwrite data, just restore it with sheets history.'
  ].join('\n');
  var ui = SpreadsheetApp.getUi();
  ui.alert('Help', text, ui.ButtonSet.OK);
}

// ---------------------------------------------------------------------------
// Dialogs
//
// ui.prompt() cannot prefill a value or mask a password, both of which the
// spec asks for, so the inputs are small modal HTML forms instead.
// ---------------------------------------------------------------------------

function showDialog_(spec, width, height) {
  var template = HtmlService.createTemplateFromFile('Dialog');
  template.spec = JSON.stringify(spec);
  SpreadsheetApp.getUi().showModalDialog(
    template.evaluate().setWidth(width).setHeight(height), spec.title);
}

function menuChangeCredentials() {
  showDialog_({
    form: 'credentials',
    title: 'Change Credentials',
    submit: 'Verify & Save',
    // Deliberately blank: stored credentials are never echoed back, and this
    // only accepts a pair that already exists on the server.
    fields: [
      { name: 'id', label: 'User ID', type: 'text', value: '' },
      { name: 'password', label: 'Password', type: 'password', value: '' }
    ],
    note: 'Checked against the server before being saved.'
  }, 340, 290);
}

function menuFetchInterval() {
  showDialog_({
    form: 'interval',
    title: 'Fetch interval',
    submit: 'Save',
    fields: [{
      name: 'minutes',
      label: 'Minutes',
      type: 'number',
      value: String(getInterval_()),
      min: ALLOWED_INTERVALS[0],
      max: ALLOWED_INTERVALS[ALLOWED_INTERVALS.length - 1]
    }],
    note: 'Rounded down to ' + ALLOWED_INTERVALS.join(', ') +
      ' - the intervals Apps Script can schedule.'
  }, 340, 250);
}

function menuOutputData() {
  showDialog_({
    form: 'sheetData',
    title: 'Output Sheets - Data',
    submit: 'Save',
    fields: [{ name: 'name', label: 'Tab name', type: 'text', value: rawSheetName_() }],
    note: 'Rewritten in full on every fetch. Created if it does not exist.'
  }, 340, 250);
}

function menuOutputMessage() {
  showDialog_({
    form: 'sheetMessage',
    title: 'Output Sheets - Message',
    submit: 'Save',
    fields: [{ name: 'name', label: 'Tab name', type: 'text', value: messageSheetName_() }],
    note: 'Receives the status of the last fetch. Created if it does not exist.'
  }, 340, 250);
}

// ---------------------------------------------------------------------------
// Dialog submissions
// ---------------------------------------------------------------------------

/** Called from Dialog.html via google.script.run. Never throws to the client. */
function dialogSubmit(form, values) {
  try {
    switch (form) {
      case 'credentials':
        return submitCredentials_(values);
      case 'interval':
        return submitInterval_(values);
      case 'sheetData':
        return submitSheet_(PROP.SHEET_RAW, values, 'Data');
      case 'sheetMessage':
        return submitSheet_(PROP.SHEET_MESSAGE, values, 'Message');
      default:
        return { ok: false, message: 'Unknown form: ' + form };
    }
  } catch (err) {
    return { ok: false, message: userMessage_(err) };
  }
}

function submitCredentials_(values) {
  var id = String(values.id || '').trim();
  var password = String(values.password || '');
  if (!id || !password) {
    return { ok: false, message: 'Both fields are required.' };
  }

  try {
    // Token first, then confirm the token is actually accepted.
    verifyToken_(fetchToken_(id, password));
  } catch (err) {
    var message = userMessage_(err);
    writeMessage_('ERROR', message);
    return { ok: false, message: message };
  }

  // Only a pair the server already accepted is persisted.
  var p = props_();
  p.setProperty(PROP.ID, id);
  p.setProperty(PROP.PASS, password);
  // The id is not echoed here: the message sheet is visible to everyone.
  writeMessage_('OK', 'Credentials accepted.');
  return { ok: true, message: 'Credentials accepted. You can close this dialog.' };
}

function submitInterval_(values) {
  var raw = Number(values.minutes);
  if (!isFinite(raw)) {
    return { ok: false, message: 'Enter a number of minutes.' };
  }

  var minutes = floorInterval_(raw);
  props_().setProperty(PROP.INTERVAL, String(minutes));
  if (isAutofetchOn_()) {
    installTrigger_(minutes); // apply without waiting for the next tick
  }

  var asked = Math.floor(raw);
  var note = minutes === asked ? '' : ' (adjusted from ' + asked + ').';
  return { ok: true, message: 'Fetch interval set to ' + minutes + ' minutes' + note };
}

function submitSheet_(key, values, label) {
  var name = String(values.name || '').trim();
  if (!name) {
    return { ok: false, message: 'Enter a tab name.' };
  }

  var other = key === PROP.SHEET_RAW ? messageSheetName_() : rawSheetName_();
  if (name === other) {
    // A fetch clears the data sheet wholesale, so sharing a tab would erase
    // the message log every run.
    return {
      ok: false,
      message: 'The data and message sheets must be different tabs.'
    };
  }

  var ss = SpreadsheetApp.getActiveSpreadsheet();
  var existed = !!ss.getSheetByName(name);
  if (!existed) {
    ss.insertSheet(name);
  }
  props_().setProperty(key, name);

  return {
    ok: true,
    message: label + ' output set to "' + name + '"' +
      (existed ? '.' : ' (new tab created).')
  };
}
