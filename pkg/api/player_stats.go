package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akiver/cs-demo-analyzer/internal/demo"
	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
	_ "modernc.org/sqlite"
)

const playerStatsAnalysisVersion = 5

type DemoImportError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type PlayerStatsBuildOptions struct {
	DatabasePath                string
	DemoPaths                   []string
	DemoDirectories             []string
	Recursive                   bool
	Source                      constants.DemoSource
	Jobs                        int
	VisibilityConfirmationTicks int
	// TrisDir holds awpy-style .tri map geometry (or a tris.zip archive) used
	// for geometric visibility checks. Defaults to "tris".
	TrisDir string
	// AllowNoGeometry permits analyzing demos for maps without .tri geometry,
	// falling back to the (much less accurate) server spotted flag. Disabled
	// by default because the fallback produces heavily skewed TTD/reaction
	// numbers that silently corrupt aggregates.
	AllowNoGeometry bool
	Force           bool
	// OnDemoProcessed, when set, is called after each demo finishes (imported,
	// skipped or failed) with the number processed so far and the total.
	OnDemoProcessed func(processed, total int)
	// OnDemoProgress, when set, receives per-demo parse progress as a 0..1
	// fraction; it is always called with 1 when a demo finishes or is skipped.
	OnDemoProgress func(path string, fraction float64)
}

type PlayerStatsBuildResult struct {
	Imported int               `json:"imported"`
	Skipped  int               `json:"skipped"`
	Failed   int               `json:"failed"`
	Errors   []DemoImportError `json:"errors,omitempty"`
}

// SuspicionConfig holds the thresholds for the two-tier flagging heuristic
// (watch = yellow, cheater = red). The tiers are driven by time-to-damage as a
// long-term average, cross-checked against performance stats. There is no
// numeric score any more — a player lands in the worst tier any rule triggers.
type SuspicionConfig struct {
	MinimumDemos      int `json:"minimumDemos"`
	MinimumShots      int `json:"minimumShots"`
	TTDMinimumSamples int `json:"ttdMinimumSamples"`

	// Time-to-damage (weighted long-term average, ms). Below the cheater line
	// the aim is not humanly reproducible over many games; the grey band above
	// it only flags red when the supporting stats are also elite.
	TTDCheaterMS       float64 `json:"ttdCheaterMs"`       // < this → cheater outright
	TTDSuspiciousMS    float64 `json:"ttdSuspiciousMs"`    // < this → at least watch, cheater if stats elite
	ReactionCheaterMS  float64 `json:"reactionCheaterMs"`  // reaction weighted avg below this → cheater

	// "Elite supporting stats" — any one of these promotes a suspicious-band
	// TTD (TTDCheaterMS..TTDSuspiciousMS) from watch to cheater.
	EliteKD           float64 `json:"eliteKd"`
	EliteHeadHitRate  float64 `json:"eliteHeadHitRate"`
	EliteAccuracy     float64 `json:"eliteAccuracy"`

	// Head-hit-rate standalone flags, gated by a minimum damage-event sample.
	HeadHitMinimumEvents   int     `json:"headHitMinimumEvents"`
	HeadHitWatchThreshold  float64 `json:"headHitWatchThreshold"`  // >= this → watch
	HeadHitCheaterThreshold float64 `json:"headHitCheaterThreshold"` // >= this → cheater
}

func DefaultSuspicionConfig() SuspicionConfig {
	return SuspicionConfig{
		MinimumDemos: 3, MinimumShots: 100,
		TTDMinimumSamples: 20,
		TTDCheaterMS:      320,
		TTDSuspiciousMS:   400,
		ReactionCheaterMS: 200,
		EliteKD:           1.3,
		EliteHeadHitRate:  .40,
		EliteAccuracy:     .30,
		HeadHitMinimumEvents:    30,
		HeadHitWatchThreshold:   .50,
		HeadHitCheaterThreshold: .60,
	}
}

type PlayerStatsReportOptions struct {
	DatabasePath string
	OutputPath   string
	Format       constants.ExportFormat
	Config       SuspicionConfig
}

func openPlayerStatsDB(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("player stats database path is required")
	}
	if parent := filepath.Dir(path); parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if err = migratePlayerStatsDB(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func migratePlayerStatsDB(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS demos (
  id INTEGER PRIMARY KEY, checksum TEXT NOT NULL UNIQUE, path TEXT NOT NULL, file_name TEXT NOT NULL,
  map_name TEXT NOT NULL, demo_date TEXT NOT NULL, tick_rate REAL NOT NULL, build_number INTEGER NOT NULL,
  source TEXT NOT NULL, analysis_version INTEGER NOT NULL, imported_at TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS players (
  steam_id INTEGER PRIMARY KEY, latest_name TEXT NOT NULL, names TEXT NOT NULL, updated_at TEXT NOT NULL,
  saved INTEGER NOT NULL DEFAULT 0, banned INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS player_demo_stats (
  demo_id INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  steam_id INTEGER NOT NULL REFERENCES players(steam_id) ON DELETE CASCADE,
  rounds INTEGER NOT NULL, shots INTEGER NOT NULL, hit_shots INTEGER NOT NULL,
  damage_events INTEGER NOT NULL, head_hit_events INTEGER NOT NULL, kills INTEGER NOT NULL,
  deaths INTEGER NOT NULL DEFAULT 0,
  headshot_kills INTEGER NOT NULL, smoke_kills INTEGER NOT NULL, wall_kills INTEGER NOT NULL,
  unspotted_damage_events INTEGER NOT NULL, first_bullet_encounters INTEGER NOT NULL,
  first_bullet_head_hits INTEGER NOT NULL, snap_events INTEGER NOT NULL,
  ttd_samples INTEGER NOT NULL, ttd_sum_ms REAL NOT NULL,
  moving_shots INTEGER NOT NULL, moving_hit_shots INTEGER NOT NULL,
  airborne_shots INTEGER NOT NULL, airborne_hit_shots INTEGER NOT NULL,
  flashed_shots INTEGER NOT NULL, flashed_hit_shots INTEGER NOT NULL,
  scoped_shots INTEGER NOT NULL, scoped_hit_shots INTEGER NOT NULL,
  PRIMARY KEY (demo_id, steam_id)
);
CREATE TABLE IF NOT EXISTS encounters (
  id INTEGER PRIMARY KEY, demo_id INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  round_number INTEGER NOT NULL, attacker_steam_id INTEGER NOT NULL, victim_steam_id INTEGER NOT NULL,
  first_spotted_tick INTEGER NOT NULL, confirmed_tick INTEGER NOT NULL, damage_tick INTEGER NOT NULL,
  ttd_ms REAL NOT NULL, ttd_confirmed_ms REAL NOT NULL, first_shot_time_ms REAL NOT NULL,
  reaction_time_ms REAL NOT NULL DEFAULT -1,
  first_angle REAL NOT NULL, confirmed_angle REAL NOT NULL, first_shot_angle REAL NOT NULL,
  distance_meters REAL NOT NULL, weapon_name TEXT NOT NULL, snap INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS encounters_attacker_idx ON encounters(attacker_steam_id);
CREATE TABLE IF NOT EXISTS reactions (
  id INTEGER PRIMARY KEY, demo_id INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  round_number INTEGER NOT NULL, attacker_steam_id INTEGER NOT NULL, victim_steam_id INTEGER NOT NULL,
  first_spotted_tick INTEGER NOT NULL, confirmed_tick INTEGER NOT NULL, shot_tick INTEGER NOT NULL,
  reaction_time_ms REAL NOT NULL, confirmed_time_ms REAL NOT NULL,
  first_angle REAL NOT NULL, shot_angle REAL NOT NULL, weapon_name TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS reactions_attacker_idx ON reactions(attacker_steam_id);
CREATE TABLE IF NOT EXISTS player_demo_weapon_stats (
  demo_id INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  steam_id INTEGER NOT NULL REFERENCES players(steam_id) ON DELETE CASCADE,
  weapon_name TEXT NOT NULL, shots INTEGER NOT NULL, hit_shots INTEGER NOT NULL,
  damage_events INTEGER NOT NULL, head_hit_events INTEGER NOT NULL, kills INTEGER NOT NULL,
  PRIMARY KEY (demo_id, steam_id, weapon_name)
);
CREATE TABLE IF NOT EXISTS evidence (
  id INTEGER PRIMARY KEY, demo_id INTEGER NOT NULL REFERENCES demos(id) ON DELETE CASCADE,
  round_number INTEGER NOT NULL, tick INTEGER NOT NULL, steam_id INTEGER NOT NULL,
  victim_steam_id INTEGER NOT NULL, kind TEXT NOT NULL, value REAL NOT NULL, details TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS evidence_player_idx ON evidence(steam_id);
INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, CURRENT_TIMESTAMP);
`)
	if err != nil {
		return err
	}
	legacyRenames := [][3]string{{"player_demo_stats", "ttm_samples", "ttd_samples"}, {"player_demo_stats", "ttm_sum_ms", "ttd_sum_ms"}, {"encounters", "ttm_first_ms", "ttd_ms"}, {"encounters", "ttm_confirmed_ms", "ttd_confirmed_ms"}}
	for _, rename := range legacyRenames {
		exists, checkErr := sqliteColumnExists(db, rename[0], rename[1])
		if checkErr != nil {
			return checkErr
		}
		if exists {
			if _, renameErr := db.Exec(`ALTER TABLE ` + rename[0] + ` RENAME COLUMN ` + rename[1] + ` TO ` + rename[2]); renameErr != nil {
				return renameErr
			}
		}
	}
	enabledExists, err := sqliteColumnExists(db, "demos", "enabled")
	if err != nil {
		return err
	}
	if !enabledExists {
		if _, err = db.Exec(`ALTER TABLE demos ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1`); err != nil {
			return err
		}
	}
	reactionExists, err := sqliteColumnExists(db, "encounters", "reaction_time_ms")
	if err != nil {
		return err
	}
	if !reactionExists {
		// -1 marks rows analyzed before the column existed; the report skips them.
		if _, err = db.Exec(`ALTER TABLE encounters ADD COLUMN reaction_time_ms REAL NOT NULL DEFAULT -1`); err != nil {
			return err
		}
	}
	addedColumns := [][3]string{
		{"player_demo_stats", "deaths", `INTEGER NOT NULL DEFAULT 0`},
		{"players", "saved", `INTEGER NOT NULL DEFAULT 0`},
		{"players", "banned", `INTEGER NOT NULL DEFAULT 0`},
	}
	for _, column := range addedColumns {
		exists, checkErr := sqliteColumnExists(db, column[0], column[1])
		if checkErr != nil {
			return checkErr
		}
		if !exists {
			if _, err = db.Exec(`ALTER TABLE ` + column[0] + ` ADD COLUMN ` + column[1] + ` ` + column[2]); err != nil {
				return err
			}
		}
	}
	_, err = db.Exec(`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (2, CURRENT_TIMESTAMP), (3, CURRENT_TIMESTAMP), (4, CURRENT_TIMESTAMP), (5, CURRENT_TIMESTAMP)`)
	return err
}

func sqliteColumnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func discoverDemoPaths(options PlayerStatsBuildOptions) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string
	add := func(path string) error {
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(filepath.Ext(abs), ".dem") {
			return nil
		}
		if !seen[abs] {
			seen[abs] = true
			paths = append(paths, abs)
		}
		return nil
	}
	for _, path := range options.DemoPaths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, fmt.Errorf("%q is a directory; use DemoDirectories", path)
		}
		if err := add(path); err != nil {
			return nil, err
		}
	}
	for _, root := range options.DemoDirectories {
		root = filepath.Clean(root)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && path != root && !options.Recursive {
				return filepath.SkipDir
			}
			if entry.Type().IsRegular() {
				return add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, errors.New("no .dem files found")
	}
	return paths, nil
}

type analyzedDemoStats struct {
	path  string
	match *Match
	stats DemoStats
	err   error
}

func analyzeOneDemoForStats(path string, options PlayerStatsBuildOptions) analyzedDemoStats {
	collector := newDemoStatsCollector(options.VisibilityConfirmationTicks, options.TrisDir)
	analyzeOptions := AnalyzeDemoOptions{Source: options.Source, statsCollector: collector}
	if options.OnDemoProgress != nil {
		analyzeOptions.onProgress = func(fraction float64) { options.OnDemoProgress(path, fraction) }
	}
	match, err := analyzeDemo(path, analyzeOptions)
	if err == nil && !options.AllowNoGeometry && collector.visEngine == nil {
		mapName := ""
		if match != nil {
			mapName = match.MapName
		}
		err = fmt.Errorf("no map geometry for %q in %q: refusing to fall back to the inaccurate spotted-flag visibility (add %s/%s.tri or tris.zip, or enable AllowNoGeometry)", mapName, options.TrisDir, options.TrisDir, mapName)
		return analyzedDemoStats{path: path, err: err}
	}
	return analyzedDemoStats{path: path, match: match, stats: collector.result, err: err}
}

func BuildPlayerStatsDatabase(ctx context.Context, options PlayerStatsBuildOptions) (*PlayerStatsBuildResult, error) {
	paths, err := discoverDemoPaths(options)
	if err != nil {
		return nil, err
	}
	db, err := openPlayerStatsDB(options.DatabasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if options.Jobs <= 0 {
		options.Jobs = 4
	}
	if options.VisibilityConfirmationTicks <= 0 {
		options.VisibilityConfirmationTicks = 3
	}
	if options.TrisDir == "" {
		options.TrisDir = "tris"
	}

	result := &PlayerStatsBuildResult{}
	jobs := make(chan string)
	results := make(chan analyzedDemoStats)
	var wg sync.WaitGroup
	workerCount := options.Jobs
	if workerCount > len(paths) {
		workerCount = len(paths)
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				info, infoErr := demo.GetDemoFromPath(path)
				if infoErr == nil && !options.Force {
					var version int
					err := db.QueryRowContext(ctx, `SELECT analysis_version FROM demos WHERE checksum = ?`, info.Checksum).Scan(&version)
					if err == nil && version == playerStatsAnalysisVersion {
						results <- analyzedDemoStats{path: path}
						continue
					}
				}
				results <- analyzeOneDemoForStats(path, options)
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()
	go func() {
		defer close(jobs)
		for _, path := range paths {
			select {
			case jobs <- path:
			case <-ctx.Done():
				return
			}
		}
	}()

	processed := 0
	total := len(paths)
	for analyzed := range results {
		processed++
		if options.OnDemoProcessed != nil {
			options.OnDemoProcessed(processed, total)
		}
		if options.OnDemoProgress != nil {
			options.OnDemoProgress(analyzed.path, 1)
		}
		if analyzed.err != nil {
			result.Failed++
			result.Errors = append(result.Errors, DemoImportError{Path: analyzed.path, Error: analyzed.err.Error()})
			continue
		}
		if analyzed.match == nil {
			result.Skipped++
			continue
		}
		if err := storeAnalyzedDemo(ctx, db, analyzed.match, analyzed.stats); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, DemoImportError{Path: analyzed.path, Error: err.Error()})
			continue
		}
		result.Imported++
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, nil
}

func storeAnalyzedDemo(ctx context.Context, db *sql.DB, match *Match, stats DemoStats) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// A re-analyzed demo replaces the previous analysis. Match by checksum,
	// but also by file name + map: the header checksum includes the file size,
	// so the same demo re-uploaded through a lossy path (e.g. a different
	// machine) can carry a different checksum and would otherwise duplicate.
	if _, err = tx.ExecContext(ctx, `DELETE FROM demos WHERE checksum = ? OR (file_name = ? AND map_name = ?)`, match.Checksum, match.DemoFileName, match.MapName); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO demos(checksum,path,file_name,map_name,demo_date,tick_rate,build_number,source,analysis_version,imported_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		match.Checksum, match.DemoFilePath, match.DemoFileName, match.MapName, match.Date.UTC().Format(time.RFC3339), match.TickRate, match.BuildNumber, match.Source.String(), playerStatsAnalysisVersion, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}
	demoID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	ids := make([]uint64, 0, len(stats.Players))
	for id := range stats.Players {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		p := stats.Players[id]
		if id == 0 {
			continue
		}
		var oldNames string
		_ = tx.QueryRowContext(ctx, `SELECT names FROM players WHERE steam_id = ?`, id).Scan(&oldNames)
		names := mergePlayerName(oldNames, p.Name)
		_, err = tx.ExecContext(ctx, `INSERT INTO players(steam_id,latest_name,names,updated_at) VALUES(?,?,?,?) ON CONFLICT(steam_id) DO UPDATE SET latest_name=excluded.latest_name,names=excluded.names,updated_at=excluded.updated_at`, id, p.Name, names, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO player_demo_stats(demo_id,steam_id,rounds,shots,hit_shots,damage_events,head_hit_events,kills,deaths,headshot_kills,smoke_kills,wall_kills,unspotted_damage_events,first_bullet_encounters,first_bullet_head_hits,snap_events,ttd_samples,ttd_sum_ms,moving_shots,moving_hit_shots,airborne_shots,airborne_hit_shots,flashed_shots,flashed_hit_shots,scoped_shots,scoped_hit_shots) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			demoID, id, p.Rounds, p.Shots, p.HitShots, p.DamageEvents, p.HeadHitEvents, p.Kills, p.Deaths, p.HeadshotKills, p.SmokeKills, p.WallKills, p.UnspottedDamageEvents, p.FirstBulletEncounters, p.FirstBulletHeadHits, p.SnapEvents, p.TTDSamples, p.TTDSumMS, p.MovingShots, p.MovingHitShots, p.AirborneShots, p.AirborneHitShots, p.FlashedShots, p.FlashedHitShots, p.ScopedShots, p.ScopedHitShots)
		if err != nil {
			return err
		}
	}
	for _, e := range stats.Encounters {
		_, err = tx.ExecContext(ctx, `INSERT INTO encounters(demo_id,round_number,attacker_steam_id,victim_steam_id,first_spotted_tick,confirmed_tick,damage_tick,ttd_ms,ttd_confirmed_ms,first_shot_time_ms,reaction_time_ms,first_angle,confirmed_angle,first_shot_angle,distance_meters,weapon_name,snap) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			demoID, e.RoundNumber, e.AttackerSteamID64, e.VictimSteamID64, e.FirstSpottedTick, e.ConfirmedTick, e.DamageTick, e.TTDMS, e.TTDConfirmedMS, e.FirstShotTimeMS, e.ReactionTimeMS, e.FirstAngle, e.ConfirmedAngle, e.FirstShotAngle, e.DistanceMeters, e.WeaponName, e.Snap)
		if err != nil {
			return err
		}
	}
	for _, reaction := range stats.Reactions {
		_, err = tx.ExecContext(ctx, `INSERT INTO reactions(demo_id,round_number,attacker_steam_id,victim_steam_id,first_spotted_tick,confirmed_tick,shot_tick,reaction_time_ms,confirmed_time_ms,first_angle,shot_angle,weapon_name) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			demoID, reaction.RoundNumber, reaction.AttackerSteamID64, reaction.VictimSteamID64, reaction.FirstSpottedTick, reaction.ConfirmedTick, reaction.ShotTick, reaction.ReactionTimeMS, reaction.ConfirmedTimeMS, reaction.FirstAngle, reaction.ShotAngle, reaction.WeaponName)
		if err != nil {
			return err
		}
	}
	for steamID, weapons := range stats.Weapons {
		for _, w := range weapons {
			_, err = tx.ExecContext(ctx, `INSERT INTO player_demo_weapon_stats(demo_id,steam_id,weapon_name,shots,hit_shots,damage_events,head_hit_events,kills) VALUES(?,?,?,?,?,?,?,?)`, demoID, steamID, w.WeaponName, w.Shots, w.HitShots, w.DamageEvents, w.HeadHitEvents, w.Kills)
			if err != nil {
				return err
			}
		}
	}
	for _, e := range stats.Evidence {
		_, err = tx.ExecContext(ctx, `INSERT INTO evidence(demo_id,round_number,tick,steam_id,victim_steam_id,kind,value,details) VALUES(?,?,?,?,?,?,?,?)`, demoID, e.RoundNumber, e.Tick, e.SteamID64, e.VictimID, e.Kind, e.Value, e.Details)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func mergePlayerName(existing, name string) string {
	set := make(map[string]bool)
	for _, n := range strings.Split(existing, "\n") {
		if n != "" {
			set[n] = true
		}
	}
	if name != "" {
		set[name] = true
	}
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, "\n")
}
