// Code.gs is plain JS and only touches Apps Script services from inside
// functions, so the whole file runs under node and its logic can be exercised
// directly. It is evaluated in a vm context that doubles as the global scope,
// which is what lets a test drop a stub in (`S.UrlFetchApp = ...`) and have the
// script's own functions pick it up at call time.
const fs = require('fs');
const vm = require('vm');
const path = require('path');

const CODE = path.join(__dirname, '..', 'Code.gs');

function load() {
  const context = vm.createContext({ console, JSON, Date, Math, Number, String,
    Object, Array, RegExp, Error, isFinite, parseInt, parseFloat, setTimeout });
  vm.runInContext(fs.readFileSync(CODE, 'utf8'), context, { filename: 'Code.gs' });
  return context;
}

let failures = 0;
function check(name, got, want) {
  const g = JSON.stringify(got), w = JSON.stringify(want);
  if (g !== w) {
    console.log(`FAIL ${name}\n       got  ${g}\n       want ${w}`);
    failures++;
  } else {
    console.log(`ok   ${name}`);
  }
}
function throws(name, fn, pattern) {
  try {
    fn();
    check(name + ' (expected a throw)', 'no throw', 'throw');
  } catch (err) {
    check(name, pattern.test(err.message), true);
  }
}
function section(name) { console.log(`\n=== ${name} ===`); }
function report() {
  console.log(failures ? `\n${failures} failure(s)` : '\nall passed');
  process.exitCode = failures ? 1 : 0;
}
function fixture(name) {
  return fs.readFileSync(path.join(__dirname, 'fixtures', name), 'utf8');
}

module.exports = { load, check, throws, section, report, fixture };
