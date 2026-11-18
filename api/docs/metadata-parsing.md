# Metadata parsing: stages 1 through 4

## Implemented

Templates remain JSONB records in `pdf_parsing_templates`. No new table or
column is needed. Existing schema v1 trade templates still work.

Schema v2 adds:

- `document`: `pdf` or `email_body`.
- `kind`: `trade`, `cash`, or `dividend`.
- `parse_mode`: `metadata` or `legacy`.
- `legacy_parser`: an existing extractor (`stockbit`, `trimegah`, `mandiri`, `eipo`).
- `email`: amount, date, reference and ticker patterns for cash parsing.
- `sender_domain`, `subject_pattern`, filename inclusion/exclusion patterns.
- `trade`: PDF extraction strategy, side mappings, fee allocation, reference
  fields, date-word replacements and configured fee fallback rates.
- `ref_template`: optional `{ticker}`, `{paydate}` and `{ref}` placeholders.
- `discovery_only`: include selectors in discovery without routing extraction.

Missing document/kind/mode default to `pdf`/`trade`/`metadata`.
See `migrations/templates/pdf_parsing_config_v2.schema.json`. Regex capture
indexes, date layouts, numeric separators and reference placeholders are also
validated by Go; JSON Schema alone cannot validate those semantics.

## Trade routing

1. Read and validate active templates from the database.
2. Match sender, subject substring and filename suffix.
3. Reject overlapping matching templates as ambiguous.
4. Execute the selected metadata engine or explicitly named legacy extractor.
5. Optionally use the old routing chain if no template applies or extraction
   fails. Store/configuration errors and ambiguous routes never use fallback.

Template selectors, not built-in broker selector lists, drive discovery.
Trade discovery requests only `trade` selectors; cash discovery requests
`cash` and `dividend` selectors. Cash queries are built from those selectors
without a hardcoded subject list. Missing/invalid templates fail closed.

For a PDF without email headers, `Parse` tries metadata first and permits the
old PDF chain only when fallback is enabled.

## Transition controls

- `PDF_PARSER_FALLBACK=true` (default): keep compatibility with uncovered layouts.
- `PDF_PARSER_FALLBACK=false`: do not use the old automatic routing chain.
- `PDF_PARSER_COMPARE=true`: compare successful metadata output against legacy
  output and log equality and row counts; always return the metadata result.
  This does not persist an extra transaction.

Disabling fallback does **not** disable legacy extractors explicitly referenced
by database templates.

## Cash and dividend execution

`metadata_cash.go` reads a validated template snapshot once per sync. It matches
the parsed sender domain and subject classifier, normalizes HTML and plain
text, then extracts and validates the fields. HTML script/style content is
ignored. Numeric separators, payment-date word replacements and reference
formats all come from the template.

The service parses MIME body parts and collapses equivalent HTML/plain-text
representations into one event. Conflicting representations abort the message.
Named text attachments are not used as the email body. Existing registration
cutoffs, registered-broker filtering, cash baselines and repository deduplication
remain unchanged.

## Current seeded templates and cutover

Migration `upgrade_pdf_templates_v2` sets the unmodified Stockbit seed and the
legacy Trimegah discovery seed to valid v2 `legacy` routes. It only upgrades
recognized seed configurations; custom configurations are left unchanged.
Stockbit extraction patterns are preserved. Trimegah discovery is re-enabled
using its database sender selector.

After read-only parity validation, `promote_metadata_trade_templates` promotes
known Stockbit/Trimegah seeds to v2 metadata extraction and archives their v1
legacy routes. Custom configs/selectors/activation choices are not overwritten.
The migration preserves fees, amount-done conventions and reference formats;
it does not recalculate historical balances.

`add_metadata_cash_templates` seeds six active DB templates:

- Stockbit deposit, withdrawal and dividend.
- Trimegah deposit and withdrawal.
- Mandiri dividend.

The EIPO layout has an explicit DB route to the existing EIPO extractor. The
regular Stockbit template excludes its filename pattern, preventing overlap.
Mandiri trade confirmations use `templates/mandiri_trade_v2.json`, seeded into
`pdf_parsing_templates` by `add_mandiri_trade_template`. The template selects
the TC subject, maps wrapped table rows, and allocates charges and sales tax.
Client statements are excluded by the subject selector. Its
`trade.password_source = "broker"` resolves the encrypted credential stored
on the user's broker; credentials are never stored in parsing templates or
returned by broker APIs. The setup form accepts PIN updates for existing
brokers through the broker update endpoint.
The `trade.ignore_patterns` array removes explicitly configured page furniture
before parsing. Mandiri uses it for the footer-to-repeated-header boundary on
multi-page confirmations. Transaction rows remain strictly validated after
cleanup; repeated headers without a configured page boundary still fail.
Trimegah dividends and Mandiri deposit/withdrawal formats are not seeded without
validated source examples.

`pdftotext` from Poppler is required for `trade.extraction = "layout"`.
`plain` keeps the existing PDF-text extractor and its Poppler fallback.

The old cash implementation is retained in `cash_parser_legacy.go` only as the
replay oracle. Production cash sync never calls it. Legacy trade extractors
remain for explicit legacy routes, optional comparison and compatibility with
uncovered layouts; this is not yet a claim that every broker has zero hardcode.

## Verification

Run from `api`:

```sh
go build ./...
go test ./...
go vet ./service/parser ./migrations
```

Local tests cover v1 compatibility, v2 metadata extraction, legacy routing,
kind-specific selectors, invalid configs, ambiguous routes, strict fallback,
metadata-first dispatch and reference-template validation. Live DB checks read
the current development templates and parse a synthetic Stockbit PDF without
writing transactions.

### Development mailbox replay, 2026-11-18

Read-only corpus: 997 Gmail messages, including all 173 cached trade
confirmation PDFs. Raw mail and PDFs remain outside the repository in a
private local directory; no email bodies, client identifiers or OAuth tokens
are included in this report.

| Check | Result |
| --- | --- |
| Cash MIME-part comparison | 176 old / 176 metadata results; zero differences or errors |
| Cash after MIME deduplication | 175 events, identical to the legacy baseline |
| Cash after registered-broker filter | 174 events |
| Regular trade PDFs | 163 PDFs, 280 trades; zero differences or errors |
| Explicit legacy EIPO route | 10 PDFs, 10 trades; zero differences |
| Combined active DB trade routes, fallback disabled | 173 PDFs, 290 trades; zero differences |
| Portfolio writes during replay | 0 |

The 175 cash events include one Mandiri dividend; it is filtered from this
user's result because Mandiri is not registered. HTML/plain-text duplicates
explain the difference between 176 MIME-part results and 175 events.

Trade comparison includes ticker, side, lot, price, date, reference, fees and
amount done (monetary tolerance below half a cent). These comparisons establish
compatibility with the old parser on this corpus, not independent correctness
of every historical fee convention. Trimegah withdrawal is covered by a
synthetic fixture, not a positive real-message sample in this corpus.

The live replay tests require `LIVE_DB=1 METADATA_CORPUS=<private directory>`:

```sh
go test ./service -run TestReplayMetadataCashIntegration -count=1 -v
go test ./service/parser -run TestMetadataTradeCorpus -count=1 -v
```

This repository ignores `*_test.go`; these local tests are not included in a
normal commit on `main`. Tests are maintained separately on `testing`.
