const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const ts = require('typescript')

const source = fs.readFileSync(path.join(__dirname, '../src/lib/cashInput.ts'), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
const helpers = { exports: {} }
new Function('exports', 'module', compiled)(helpers.exports, helpers)
const { formatCashAmount, formatCashInput, parseCashAmount, formatPastedCash, cashInputCaret } = helpers.exports

test('cash prefills hide floating-point noise without changing meaningful cents', () => {
  assert.equal(formatCashAmount(9476675.080000004), '9.476.675,08')
  assert.equal(formatCashAmount(1500000), '1.500.000')
  assert.equal(formatCashAmount(0), '0')
  assert.equal(formatCashAmount(1234.567), '1.234,57')
  assert.equal(formatCashAmount(Infinity), '')
})

test('typing groups thousands and preserves incomplete decimal input', () => {
  for (const [input, expected] of [
    ['', ''], ['0', '0'], ['1000', '1.000'], ['1.0000', '10.000'],
    ['9476675,08', '9.476.675,08'], ['1.000,', '1.000,'],
    ['1.000,0', '1.000,0'], [',', '0,'], [',5', '0,5'],
  ]) assert.equal(formatCashInput(input), expected)
  for (const input of ['-1', '1e3', 'abc', '1,234', '1,2,3']) {
    assert.equal(formatCashInput(input), null)
  }
})

test('saving converts localized input to a finite nonnegative API number', () => {
  assert.equal(parseCashAmount('9.476.675,08'), 9476675.08)
  assert.equal(parseCashAmount('9476675,08'), 9476675.08)
  assert.equal(parseCashAmount('1.000,'), 1000)
  assert.equal(parseCashAmount('0,50'), 0.5)
  assert.equal(parseCashAmount('0'), 0)
  for (const input of ['', ',', '-1', '1,234', 'NaN', 'Infinity', '1e3', '12.34', '1..000']) {
    assert.equal(parseCashAmount(input), null)
  }
})

test('paste accepts Indonesian money and raw decimal values but not invalid amounts', () => {
  for (const [input, expected] of [
    ['9476675.080000004', '9.476.675,08'], ['9.476.675,08', '9.476.675,08'],
    [' Rp 9.476.675,08 ', '9.476.675,08'], ['Rp1.000', '1.000'],
    ['1.234', '1.234'], ['1234.56', '1.234,56'], ['0.08', '0,08'],
    ['0', '0'], ['1000', '1.000'], ['1.9999', '2'],
  ]) assert.equal(formatPastedCash(input), expected)
  assert.equal(formatPastedCash('0.080'), '0,08')
  for (const input of ['-100', '1e6', 'abc123', 'NaN', '1,2,3', '1.234,567', '1,234.56']) {
    assert.equal(formatPastedCash(input), null)
  }
})

test('caret stays beside the same digits after thousands separators change', () => {
  assert.equal(cashInputCaret('1000', 4, '1.000'), 5)
  assert.equal(cashInputCaret('12.345', 2, '12.345'), 2)
  assert.equal(cashInputCaret('1.0000', 6, '10.000'), 6)
  assert.equal(cashInputCaret('1.0000', 0, '10.000'), 0)
  assert.equal(cashInputCaret(',', 1, '0,'), 2)
  assert.equal(cashInputCaret(',5', 2, '0,5'), 3)
})
