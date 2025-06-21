# DB Indexes

```SQL
-- =========== Indexes for the 'player_advanced_stats' table ===========

-- This single composite index will make filtering by season AND playoff status extremely fast.
CREATE INDEX IF NOT EXISTS idx_advanced_stats_season_playoff ON public.player_advanced_stats (season, is_playoff);

-- This index will make sorting by win_shares much faster.
CREATE INDEX IF NOT EXISTS idx_advanced_stats_win_shares ON public.player_advanced_stats (win_shares DESC);

-- These are good to have for other potential queries.
CREATE INDEX IF NOT EXISTS idx_advanced_stats_player_id ON public.player_advanced_stats (player_id);
CREATE INDEX IF NOT EXISTS idx_advanced_stats_team ON public.player_advanced_stats (team);


-- =========== Indexes for the 'player_total_stats' table ===========

-- A composite index for season and playoff status on the totals table.
CREATE INDEX IF NOT EXISTS idx_total_stats_season_playoff ON public.player_total_stats (season, is_playoff);

-- This index will make sorting by points much faster.
CREATE INDEX IF NOT EXISTS idx_total_stats_points ON public.player_total_stats (points DESC);

-- These are good to have for other potential queries.
CREATE INDEX IF NOT EXISTS idx_total_stats_player_id ON public.player_total_stats (player_id);
CREATE INDEX IF NOT EXISTS idx_total_stats_team ON public.player_total_stats (team);



-- ======================================================= Indexes PART 2 =======================================================

-- =========== Indexes for the 'player_advanced_stats' table ===========

-- To make searching for a specific player by their name fast.
CREATE INDEX IF NOT EXISTS idx_advanced_stats_player_name ON public.player_advanced_stats (player_name);

-- To allow fast sorting for leaderboards based on Player Efficiency Rating (PER).
CREATE INDEX IF NOT EXISTS idx_advanced_stats_per ON public.player_advanced_stats (per DESC);

-- To allow fast sorting by Value Over Replacement Player (VORP).
CREATE INDEX IF NOT EXISTS idx_advanced_stats_vorp ON public.player_advanced_stats (vorp DESC);

-- To allow fast sorting by Usage Percentage.
CREATE INDEX IF NOT EXISTS idx_advanced_stats_usage_percent ON public.player_advanced_stats (usage_percent DESC);




-- =========== Indexes for the 'player_total_stats' table ===========

-- To make searching for a specific player by their name fast.
CREATE INDEX IF NOT EXISTS idx_total_stats_player_name ON public.player_total_stats (player_name);

-- To allow fast sorting for leaderboards based on total rebounds.
CREATE INDEX IF NOT EXISTS idx_total_stats_total_rb ON public.player_total_stats (total_rb DESC);

-- To allow fast sorting for leaderboards based on assists.
CREATE INDEX IF NOT EXISTS idx_total_stats_assists ON public.player_total_stats (assists DESC);

-- To allow fast sorting for leaderboards based on blocks.
CREATE INDEX IF NOT EXISTS idx_total_stats_blocks ON public.player_total_stats (blocks DESC);
```

## Advanced Tip: For Better Text Searches

The standard index on `player_name` is great for exact matches. If you ever want to implement a feature like "search for players whose name *contains* 'davis'", a standard index won't be very effective.

For that, PostgreSQL offers a powerful extension called `pg_trgm` for "trigram" matching.

**Example (Optional, for future use):**

```sql
-- Step 1: Enable the extension (only needs to be run once per database)
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Step 2: Create a GIN index on the player_name column
CREATE INDEX IF NOT EXISTS idx_advanced_stats_player_name_trgm ON public.player_advanced_stats USING gin (player_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_total_stats_player_name_trgm ON public.player_total_stats USING gin (player_name gin_trgm_ops);
```

With this type of index, queries using `ILIKE '%davis%'` become extremely fast. This is something to keep in mind as you add more features.

For now, running the standard `CREATE INDEX` commands listed above will give you excellent, comprehensive performance for a wide variety of common queries and sorting operations. You've built a truly robust and high-performance API.
