# NBA_Go API v1 — Query Contract Redesign

**Status:** Proposal, awaiting approval · **Date:** 2026-11-04 · **Base URL:** `https://nba.space-city.dev`

## Why

The current `/api/*` endpoints work, but they're hard to explore. Parameters are case-sensitive, bad values are silently ignored, the same stat has different names on different endpoints, and the docs describe a different pagination object from the one the API returns. v1 should feel like a professional sports API: one vocabulary, predictable filters, and clear errors. It's also the version that replaces the older deployments (`nbago.server.nbaapi.com`, `base_url_2`).

**Goals**
- One consistent set of parameters and field names across every endpoint.
- Lenient input (case, aliases, season formats), with clear errors when a value can't be understood.
- The same pagination envelope everywhere, including shot charts.
- Postman collection rebuilt around v1, with better example queries and the HTML visualizers kept.

**Non-goals (later, dbt data marts):** re-modelling into star schemas, new stat sources, per-36 and on/off metrics, live data.

**Anti-goals:** breaking existing `/api/*` consumers without notice; any change to the scrape/import pipeline beyond the additive columns below.

## Evidence (live probes, 2026-11-04)

| Request | Result | Problem |
|---|---|---|
| `/api/playertotals?season=2025&team=hou` | 0 rows | team filter is case-sensitive on stats; games silently drop unknown teams |
| `/api/playertotals?sortBy=pts` | sorted by points anyway | unknown `sortBy` silently falls back to the default |
| `/api/games?season=2025` | 33,384 rows (all games) | games has no season filter |
| `/api/games?date=2026-06-13` | 0 rows | exact match against a timestamp; stored time is also wrong (8:30p ET stored as `12:30Z`) |
| `/api/playershotchart?...` | bare array | no pagination; leaks `ID/CreatedAt/DeletedAt`; `playerName` = player id; `date:"Oct 24,2024"`, `qtr:"3rd Qtr"` |
| totals vs games | `fieldGoals` vs `fg` | the same stat has two names; `minutesPg` is actually total minutes |
| box score DNP rows | all zeros | a DNP can't be told apart from a 0-for-0 line |
| `/api/games` cache | `immutable`, 30-day edge TTL | current-season scores go stale |

## Design

### 1. Versioning and deprecation
- New routes live under **`/v1`**. `/api/*` keeps working unchanged and adds `Deprecation: true`, `Sunset: <date>` and `Link: </v1/...>; rel="successor-version"` headers.
- The swagger docs show v1 by default, and the legacy endpoints are grouped under "Deprecated".

### 2. Resources

| Endpoint | Purpose |
|---|---|
| `GET /v1/games` | Schedule and results (list) |
| `GET /v1/games/{gameId}` | One game with its full box score (line score, player and team stats) embedded |
| `GET /v1/players` | Player directory and search (`q=jokic`), built from the distinct players in the stats tables |
| `GET /v1/players/{playerId}` | Profile summary: seasons played, teams |
| `GET /v1/stats/players` | Season stats; `measure=totals\|advanced` replaces two separate endpoints |
| `GET /v1/shots` | Shot chart events |
| `GET /v1/teams` | Static team list (abbreviation, name, historical aliases) for discovery |

`/v1/players` and `/v1/teams` are the "general exploration" entry points: you can find valid IDs without already knowing Basketball Reference conventions.

### 3. Shared parameters (same name and behavior everywhere)

| Param | Accepts | Replaces |
|---|---|---|
| `season` | `2025`, `2024-25`, or a list `2023,2024`; `current` | `season` (int only) |
| `seasonType` | `regular` (default) \| `playoffs` \| `all` | `isPlayoff` (kept as an alias) |
| `team` | Any case; current abbreviations plus BR aliases (`BRK→BKN`, `PHO→PHX`, `CHO→CHA`); list `LAL,BOS` | `team` |
| `opponent` | same as `team` | shot chart `opponent` only |
| `player` | `playerId`, or a list of them | `playerId` / `player_id` |
| `dateFrom`, `dateTo`, `date` | `YYYY-MM-DD`, matched on the local game date | `date` (exact timestamp) |
| `sort` | `-pts,ast` (prefix `-` = descending) | `sortBy` + `ascending` (kept as aliases) |
| `page`, `perPage` | `perPage` default 25, max 100 | `pageSize` (alias), shot chart's fixed 50 |
| `fields` | sparse fieldset, e.g. `fields=playerName,pts,ast` | — |

Endpoint-specific parameters:
- **games:** `homeAway`, `include=lineScore,playerStats,teamStats` (list endpoint only, any case, whitespace allowed)
- **stats:** `measure`, `per=total|game` (per-game is computed as total / games), `minGames`
- **shots:** `period=1..4|OT`, `made=true|false`, `shotType=2PT|3PT`, `minDistance`, `maxDistance`

### 4. Lenient but honest validation
- Normalize first: trim, apply case-folding, accept aliases.
- If a value still can't be understood, return **400** with a helpful body:
  ```json
  {"error":{"code":"invalid_parameter","param":"sort","value":"pts2",
            "message":"Unknown sort field 'pts2'.","allowed":["pts","ast","reb","..."]}}
  ```
- Unknown parameter *names* are ignored but echoed in `meta.warnings`, so typos stay visible without breaking clients.

### 5. One response envelope
```json
{
  "data": [ ... ],
  "meta": {"page":1,"perPage":25,"totalItems":2073,"totalPages":83,
           "query":{"season":2026,"seasonType":"playoffs","sort":"-date"},"warnings":[]},
  "links": {"self":"...","next":"...?page=2","prev":null}
}
```
`meta.query` echoes the *normalized* filters, so callers can see how their input was interpreted.

### 6. One field vocabulary
These names follow the conventions of common sports APIs and are the same on every endpoint:

`gp, gs, min, pts, reb, oreb, dreb, ast, stl, blk, tov, pf, fgm, fga, fgPct, fg3m, fg3a, fg3Pct, fg2m, fg2a, fg2Pct, ftm, fta, ftPct, efgPct, tsPct, plusMinus, gameScore`, plus advanced stats: `per, usgPct, astPct, rebPct, orebPct, drebPct, stlPct, blkPct, tovPct, fg3aRate, ftRate, ows, dws, ws, wsPer48, obpm, dbpm, bpm, vorp, ortg, drtg`.

Data fixes in the response layer:
- `min` is a decimal number. Box scores also keep `minDisplay: "38:34"`.
- DNP rows get `status: "DNP"` and `dnpReason`, with `null` stats instead of zeros.
- Line scores become `periods: [{period:1,pts:13},...]`, which supports any number of overtimes instead of a fixed `ot1-ot3`.
- Games: `date` is the local game date (`YYYY-MM-DD`), `tipoffUtc` is the corrected UTC timestamp, and `season` and `seasonType` are added. Teams are objects: `home: {team, pts}`, `away: {team, pts}`.
- Multi-team rows (`2TM`, `3TM`): `team: null`, `isCombined: true`, and the teams listed. `team=` filters exclude combined rows unless `includeCombined=true`.
- Shots: `gameDate` (ISO), `period` (int), `clock`, `x/y`, `made`, a real `playerName`, and `gameId` where it can be matched.

### 7. Backend changes (additive only)
- **Migrations:** `games.season`, `games.game_date`, a corrected `games.tipoff_utc`; `player_shot_charts.game_date` and `period`. They're backfilled once by an `import-data` sub-step, and indexed on `(season, is_playoff)`, `(game_date)`, `(player_id, season)`.
- **New shared package `api/query`:** parses season, team, sort, pagination and fields into a typed struct and validates against a per-endpoint whitelist. Every handler uses it, which removes the four copies of filter, sort and paginate code.
- **DTO mappers:** move into `api/v1/dto`, separate from the GORM models.
- **Cache policy per request:** `Immutable` only when every requested season is completed; otherwise `SemiDynamic`. This fixes stale current-season games.
- Regenerate swagger. Add table-driven tests for the parser and handler tests for each endpoint.

### 8. Postman
- New collection, **"NBA Stats API v1"**, in the `nba-stats` workspace, with one folder per resource. The current collection is renamed "(Deprecated) NBA Statistics Go OVH" and points to the new one.
- Collection variables: `baseUrl`, `season`, `playerId`, `team`, `gameId`. `base_url_2` and `new_base` are removed.
- **The HTML visualizers are kept.** The existing `pm.visualizer` scripts are moved onto the matching v1 requests and adapted to the new field names.
- Example queries that tell a story:
  1. Last night's scores: `/v1/games?date={{yesterday}}`
  2. 2026 Finals box score: `/v1/games/202606130SAS`
  3. Scoring leaders, per game: `/v1/stats/players?season=current&per=game&sort=-pts&minGames=20`
  4. Wembanyama's career advanced stats: `/v1/stats/players?player=wembavi01&measure=advanced&seasonType=all`
  5. Jokić's made threes, 2025: `/v1/shots?player=jokicni01&season=2025&shotType=3PT&made=true`
  6. Knicks vs Spurs head-to-head: `/v1/games?team=NYK&opponent=SAS&seasonType=all`
  7. Find a player: `/v1/players?q=brunson`
- The collection description is rewritten to match the actual envelope. This fixes the "cursor-based" wording and the `totalPages` mismatch.

## Rollout
1. Write the query parser and its tests, plus the migrations and backfill.
2. `/v1/stats/players` and `/v1/teams`, the simplest endpoints and the ones that prove the parser.
3. `/v1/games` and `/v1/games/{id}` (date fix, periods, DNP handling).
4. `/v1/shots`, `/v1/players`.
5. Deprecation headers on `/api/*`, swagger, then the Postman v1 collection and visualizers.
6. Deploy and run the load-test suite (`test/loadtest.go`) against v1.

## Open, left to the builder
- Exact sunset date for `/api/*` (suggested: 6 months after v1 launches).
- Whether a stats endpoint `per=36` is worth adding now; it's cheap once `min` is numeric.
- Verify the BR abbreviations actually stored in each table before finalizing the alias map.
