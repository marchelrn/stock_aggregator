const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const ts = require('typescript')

// Helpers ini hanya memakai type imports, sehingga dapat diuji tanpa browser/Vite.
const source = fs.readFileSync(path.join(__dirname, '../src/lib/market.ts'), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
const helpers = { exports: {} }
new Function('exports', 'module', compiled)(helpers.exports, helpers)
const { buildWatchlist, getWatchlistMovers, marketTime } = helpers.exports
const holding = (ticker, lot, broker_name = 'Broker A') => ({ ticker, lot, broker_name })
const quote = (change_percent) => ({ price: 100, change_percent })

test('watchlist groups owned tickers across brokers and excludes closed positions', () => {
  const rows = buildWatchlist([
    holding(' bbca ', 2), holding('BBCA', 3, 'Broker B'), holding('BBCA', 1),
    holding('ASII', 1), holding('ZERO', 0), holding('NEG', -1), holding('BAD', NaN), holding('', 10),
  ], { BBCA: quote(2) })
  assert.deepEqual(rows.map((row) => row.ticker), ['ASII', 'BBCA'])
  assert.equal(rows[1].lot, 6)
  assert.deepEqual(rows[1].brokers, ['Broker A', 'Broker B'])
  assert.equal(rows[0].quote, null)
  assert.equal(rows[1].quote.price, 100)
})

test('movers use only returned prices and never treat missing data as unchanged', () => {
  const rows = buildWatchlist(['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H'].map((ticker) => holding(ticker, 1)), {
    A: quote(2), B: quote(10), C: quote(-1), D: quote(-5), E: quote(0), F: quote(NaN), G: quote(5), H: quote(1),
  })
  rows.push({ ticker: 'MISSING', lot: 1, brokers: [], quote: null })
  const movers = getWatchlistMovers(rows)
  assert.deepEqual(movers.gainers.map((row) => row.ticker), ['B', 'G', 'A'])
  assert.deepEqual(movers.losers.map((row) => row.ticker), ['D', 'C'])
  assert.equal(movers.unchanged, 1)
  assert.equal(movers.unavailable, 2)
})

test('empty holdings and missing timestamps have explicit empty states', () => {
  assert.deepEqual(buildWatchlist([], {}), [])
  assert.deepEqual(getWatchlistMovers([]), { gainers: [], losers: [], unchanged: 0, unavailable: 0 })
  assert.equal(marketTime(null), 'Tidak tersedia')
  assert.equal(marketTime('invalid'), 'Tidak tersedia')
  assert.match(marketTime(1700000000), /WIB$/)
})
