# Backend refactor — progress / handoff notes

Resuming this work in a new session? Read this file first, then the original
plan at `/Users/home/.claude/plans/parsed-nibbling-hamster.md` for full
context and rationale.

## Objective (user's original ask)

Refactor the whole Go/Gin backend to: (1) reduce redundant code, (2) make it
easier for humans to modify, (3) break large functions into smaller pieces,
(4) ensure good logging/exception handling and reuse of API/DB calls, (5)
decouple where sensible.

**Hard constraint**: do NOT change any route, request param, or response
JSON shape. Recent commits finalized the API contract — this is a purely
internal refactor. Verified by re-checking each rewritten handler's JSON
output field-by-field against the original.

## New shared packages (all done, all tested, all in use)

- `respond/respond.go` — `respond.Error(c, status, message, err, extra gin.H)` and `respond.OK(c, payload)`. Replaces the `gin.H{"error":..., "details":...}` literal that was duplicated 30+ times. If `err` is nil, the `details` key is omitted (matches handlers that 400 on bad input with no underlying Go error).
- `authctx/authctx.go` — `authctx.RequireUser(c) (*models.User, bool)`. Replaces `c.Get("user")` + type-assert + 401 boilerplate (~25 occurrences). Writes the 401 itself on failure; caller does `user, ok := authctx.RequireUser(c); if !ok { return }`.
- `pageparam/pageparam.go` — `pageparam.Parse(c, defaultSize, maxSize) (page, size int)` and generic `pageparam.Slice[T](items []T, page, size int) Result[T]`. `Result.JSON()` returns the standard `{"page","page_size","total_records","total_pages","has_next","has_previous"}` block. Has unit tests (`pageparam_test.go`).
- `s3store/s3store.go` — `OperatorFilePath(ro, filename)` (the `"opt360Store/"+PascalCase(ro)+"/"+filename` builder), `FetchBytes`, `FetchJSON`, `PutJSON`, `FetchParquetRows[T]`/`DecodeParquetRows[T]` (JSON round-trip decode, same as original handlers). `NotFoundError` + `IsNotFound(err)` let callers distinguish "S3 object fetch failed" (→ 404 in the original handlers) from "fetched fine but failed to decode" (→ 500). Has unit tests (`s3store_test.go`).

Important simplification made consistently throughout: the original code had
4-6 *different* 500-level error messages per handler depending on which
exact step failed (S3 client creation vs body read vs temp file vs parquet
open vs parquet decode, etc). These have been collapsed to one or two
generic 500 messages per handler, with the real underlying error preserved
in the `details` field via `err.Error()`. This was a deliberate call: those
distinctions aren't part of the "API contract" (they're undifferentiated
infra-failure 500s), and preserving 5 near-identical message strings per
handler would have defeated the point of the refactor. 404-vs-500 status
code distinctions WERE preserved everywhere (e.g. "file not found" 404 vs
"failed to parse" 500).

## Done

1. **db/db.go**: `GetActiveOperators`/`GetInactiveOperators` collapsed into private `getOperatorsByStatus(ro, wantActive bool)`, with the two exported functions as one-line wrappers (call sites unchanged). Extracted `queryOptDetailsByRO` and `queryUIDUserStatuses` helpers.
2. **db/db.go**: moved the raw SQL out of `handlers/OperatorTab/activeOpt.go` into new `db.GetActiveUserCodes()` (handler was doing raw SQL directly — a layering violation; now consistent with the rest of `db` package).
3. **db/db.go**: added `GetOperatorsByRiskBucket` (moved out of `operatorListWithRisk.go` handler), with `scanOperatorWithRisk` and `enrichOperatorsWithUIDStatus` extracted as named helpers. Reuses the `uidUserInfo` type from step 1.
4. **db/operatorFilter.go** (new file): moved the ~250-line cross-cluster filter/pagination logic out of `handlers/OperatorTab/filterOperatorList.go` into `db.GetFilteredOperators(OperatorFilterParams)`, split into `queryUIDStatusMap`, `buildOperatorFilterWhere`, `countFilteredOperators`, `fetchFilteredOperatorPage`, `scanFilteredOperator`, `enrichFilteredOperatorsUserStatus`.
5. **handlers/OperatorTab/** — all 9 files migrated to the shared packages and shrunk substantially:
   - `activeOperatorList.go`, `inactiveOperatorList.go` — now ~35 lines each (were ~96).
   - `activeOpt.go` — now ~28 lines, raw SQL moved to db package.
   - `highRiskOpt.go`, `medRiskOpt.go`, `lowRiskOpt.go` — now ~35 lines each (were ~180-193). Share a new private generic helper `fetchOperatorParquetRows[T]` / `fetchRiskOperatorRows` in new file `handlers/OperatorTab/riskOperators.go` (handles the 404-vs-500 split per file's own not-found message/key wording — high/med use `"Parquet file not found"`+`"file_path"`, low uses `"Low risk operators file not found"`+`"file"` — preserved exactly).
   - `allOptList.go` — now ~65 lines (was ~380). Filtering logic extracted to new file `handlers/OperatorTab/operatorRowFilters.go` (`filterOperatorRows`, `matchesOperatorFilters`, `matchesActiveStatus`, `matchesRiskBucket`, `findRowField` + fold-compare helpers). Behavior double-checked against original including the "unrecognized active_status value is a no-op filter" quirk and the `continue`-vs-`break` semantics in the original risk-score type switch.
   - `operatorListWithRisk.go` — now ~75 lines (was ~268), query logic moved to db package, kept `parseRiskBucket` as a small private validator.
   - `filterOperatorList.go` — now ~85 lines (was ~435), query logic moved to db package.
6. Ran `go build ./...` and `go vet ./...` after every file — clean throughout.

## Remaining (see plan file for the full original scope)

- [x] **OperatorDetailView** — DONE. `operatorDetails.go`, `operatorFeatures.go`, `operatorRiskDetails.go` all wired to `fetchOperatorJSONFile[T]`. `transformFeatureKPIs` extracted in `operatorFeatures.go`. `operatorStatus.go` migrated to `respond.Error` (no auth check kept as-is).
- [x] **LandingPage** — DONE (all 12 files).
  - S3-based: `kpi.go`, `roRiskDistribution.go`, `featureAnalysis.go`, `selectedEas.go`, `selectedRegistrars.go` — all use `s3store.FetchJSON` + `s3store.OperatorFilePath` + `authctx` + `respond`. `selectedEas.go` and `selectedRegistrars.go` each gained a private `loadAuditData`/`loadAuditDataRegistrar` helper to eliminate the 3-way duplicate fetch within the file.
  - DB-only: `roRiskDist.go`, `highestRiskOpt.go`, `stateDistrict.go`, `eaRegistrar.go`, `getEaAndReg.go`, `top10eav1.go`, `top10regv1.go` — all migrated to `authctx` + `respond.Error`. Success responses that return a typed struct (e.g. `models.RORiskDistResponse`) kept as `c.JSON` since `respond.OK` only accepts `gin.H`.
- [ ] **SidReview** (anamolousSid.go, searchBySid.go, sidBatchGetProxy.go) — not started. `searchBySid.go` (~385 lines, `SearchOperatorPacketsBySID`) needs splitting: extract `parseSearchFilters(c)`, keep parquet fetch via `s3store.FetchParquetRows[map[string]interface{}]` (note: it does its own date-field normalization post-decode, keep that as its own extracted function), extract `applyFilters(rows, filters)`, pagination via `pageparam`. `anamolousSid.go` (~200 lines) needs `flattenAnomalyRecords` extracted for its nested-loop JSON flattening.
- [x] **AnamolyIndicators** — DONE. `GetAnamolyIndicators` uses `s3store.FetchBytes` (not FetchJSON) because the file needs a `strings.ReplaceAll("'", "\"")` pass before JSON parsing.
- [x] **RegionEvaluation** — DONE. Dynamic path (ro/state/district variants) built with `utils.ToPascalCase`; uses `s3store.FetchJSON`.
- [x] **Feedback** — DONE. `SubmitFeedback` uses `s3store.PutJSON` to replace the marshal+PutObject block.
- [x] **Anomaly** — DONE. `ReportAnomaly` has no auth check (intentional, kept as-is); just swapped `c.JSON` errors to `respond.Error`.
- [x] **User** — DONE. `GetUserInfo` uses `authctx.RequireUser`, keeps `c.JSON(200, user)` (struct response, can't use `respond.OK`). `UpdateRO` uses `respond.Error`/`respond.OK` but keeps its own token-claims auth (can't use `authctx`).
- [x] **SidReview** — DONE.
  - `anamolousSid.go`: `flattenAnomalyRecords` extracted; `s3store.FetchJSON`; `pageparam.Parse`+`Slice`.
  - `searchBySid.go`: `searchFilters` struct + `parseSearchFilters`, `normalizeDateFields`, `applySearchFilters` extracted; `s3store.FetchParquetRows[map[string]interface{}]`; `pageparam.Parse`+`Slice`.
  - `sidBatchGetProxy.go`: minimal change — just `respond.Error`/`respond.OK`.
- [x] **Final sweep** — DONE. `go build ./...` ✓, `go vet ./...` ✓, `go test ./...` ✓ (pageparam + s3store unit tests pass). `grep -rn 'c.Get("user")' handlers/` → 0 matches. `grep -rn '"details": err.Error()' handlers/` → 0 matches. `grep -rn 'aws-sdk-go' handlers/` → 0 matches.
- [ ] Manually smoke-test a couple of endpoints (e.g. `/api/high_risk_operator`, `/api/kpi`) against a real config if possible — no test suite exists for handler logic so build+vet is the main safety net.

## Conventions established (follow these for the remaining files)

- Auth: `user, ok := authctx.RequireUser(c); if !ok { return }`
- Pagination: `page, pageSize := pageparam.Parse(c, <default>, <max>)` then `result := pageparam.Slice(items, page, pageSize)`, respond with `result.JSON()` under `"pagination"` and `result.Items`/`len(result.Items)` for data/count.
- S3 JSON fetch: `s3store.FetchJSON(s3Cfg, key, &out)`, check `s3store.IsNotFound(err)` to pick 404 vs 500.
- S3 parquet fetch: `s3store.FetchParquetRows[T](s3Cfg, key)`, same 404/500 split via `IsNotFound`.
- Errors: `respond.Error(c, status, message, err, extra gin.H)` (pass `nil` for `err`/`extra` when not applicable). Success: `respond.OK(c, gin.H{...})`.
- Don't rename DB columns/JSON field names, don't touch `routes/routes.go`, don't fix the `enrolnment_type` typo or `Anamoly`/`Anomaly` package-naming inconsistency — explicitly out of scope per the plan.
- When a handler is doing raw SQL inline (like `activeOpt.go` was), prefer moving the query into the `db` package rather than just wrapping it in `respond`/`authctx` — keeps the layering consistent with the rest of the codebase.
