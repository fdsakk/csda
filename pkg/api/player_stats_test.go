package api

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
	"github.com/golang/geo/r3"
)

func TestPercentileInterpolates(t *testing.T) {
	values := []float64{100, 200, 300, 400}
	if got := percentile(values, .5); got != 250 {
		t.Fatalf("median = %v, want 250", got)
	}
	if got := percentile(values, .1); math.Abs(got-130) > .001 {
		t.Fatalf("p10 = %v, want 130", got)
	}
}

func TestRoundWeightedDemoMedian(t *testing.T) {
	groups := map[int64]*demoSamples{1: {rounds: 10, values: []float64{100, 100}}, 2: {rounds: 30, values: []float64{300, 300}}}
	if got := roundWeightedDemoMedian(groups); got != 250 {
		t.Fatalf("weighted median = %v, want 250", got)
	}
}

func demoTimingEncounters(players, samples int, ttd, reaction float64) []DemoEncounter {
	encounters := make([]DemoEncounter, 0, players*samples)
	for player := 1; player <= players; player++ {
		for sample := 0; sample < samples; sample++ {
			// Small deterministic variation prevents the fixture from depending on
			// an entirely identical, artificial distribution.
			variation := float64((sample%3)-1) * 5
			encounters = append(encounters, DemoEncounter{
				AttackerSteamID64: uint64(player),
				TTDMS:             ttd + variation,
				ReactionTimeMS:    reaction + variation,
				WeaponName:        "AK-47",
			})
		}
	}
	return encounters
}

func TestDemoQualityFlagsSystemicLowTiming(t *testing.T) {
	quality := assessDemoQuality(demoTimingEncounters(5, 10, 240, 140))
	if quality.Status != demoQualityStatusWarning {
		t.Fatalf("quality=%+v, want warning", quality)
	}
	if quality.Reason != "systemic_low_timing" {
		t.Fatalf("unexpected quality reason: %+v", quality)
	}
}

func TestDemoQualityDoesNotFlagOneFastPlayer(t *testing.T) {
	encounters := demoTimingEncounters(1, 10, 220, 130)
	for player := 2; player <= 5; player++ {
		for sample := 0; sample < 10; sample++ {
			encounters = append(encounters, DemoEncounter{AttackerSteamID64: uint64(player), TTDMS: 480 + float64(sample%3)*20, ReactionTimeMS: 270 + float64(sample%3)*15, WeaponName: "AK-47"})
		}
	}
	quality := assessDemoQuality(encounters)
	if quality.Status != demoQualityStatusOK {
		t.Fatalf("one fast player disabled the whole demo: %+v", quality)
	}
}

func TestFlagPlayerRequiresMinimumSample(t *testing.T) {
	config := DefaultSuspicionConfig()
	row := PlayerStatsReportRow{DemoCount: 2, Shots: 99, NonAWPTTDSamples: 20, NonAWPTTDWeightedMS: 300}
	flagPlayer(&row, config)
	if row.Eligible || row.Status != "insufficient_sample" {
		t.Fatalf("unexpected eligibility: %+v", row)
	}
}

func TestValidateSuspicionConfig(t *testing.T) {
	config := DefaultSuspicionConfig()
	if err := ValidateSuspicionConfig(config); err != nil {
		t.Fatalf("default config is invalid: %v", err)
	}

	config.TTDCheaterMS = config.TTDSuspiciousMS
	if err := ValidateSuspicionConfig(config); err == nil {
		t.Fatal("expected overlapping rifle TTD bands to be rejected")
	}

	config = DefaultSuspicionConfig()
	config.HeadHitWatchThreshold = 1.1
	if err := ValidateSuspicionConfig(config); err == nil {
		t.Fatal("expected rate above 1 to be rejected")
	}
}

func TestFlagPlayerTiers(t *testing.T) {
	config := DefaultSuspicionConfig()
	base := PlayerStatsReportRow{DemoCount: 3, Shots: 100, NonAWPTTDSamples: 20, Kills: 10, Deaths: 20, Accuracy: .15, HeadHitRate: .20}

	cases := []struct {
		name   string
		mutate func(*PlayerStatsReportRow)
		want   string
	}{
		{"non-awp ttd below cheater line", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 300 }, "cheater"},
		{"non-awp ttd suspicious with mediocre stats", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 350 }, "watch"},
		{"non-awp ttd suspicious with elite kd stays watch", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 350; r.Kills, r.Deaths = 20, 10 }, "watch"},
		{"non-awp ttd suspicious with elite head accuracy stays watch", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 350; r.DamageEvents, r.HeadHitRate = 30, .42 }, "watch"},
		{"non-awp ttd above suspicious band is normal", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 360 }, "normal"},
		{"healthy non-awp ttd", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 500 }, "normal"},
		{"non-awp reaction below human floor", func(r *PlayerStatsReportRow) {
			r.NonAWPTTDWeightedMS = 500
			r.NonAWPReactionSamples, r.NonAWPReactionWeightedMS = 20, 180
		}, "cheater"},
		{"head hit rate alone stays normal", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 500; r.DamageEvents, r.HeadHitRate = 40, .62 }, "normal"},
		{"insufficient non-awp ttd sample stays normal", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 300; r.NonAWPTTDSamples = 5 }, "normal"},
		// AWP tier is independent of the rifle: clean rifle, fast AWP → flagged.
		{"awp ttd at cheater anchor", func(r *PlayerStatsReportRow) {
			r.NonAWPTTDWeightedMS = 500
			r.AWPTTDSamples, r.AWPTTDWeightedMS = 20, 180
		}, "cheater"},
		{"awp ttd watch band", func(r *PlayerStatsReportRow) {
			r.NonAWPTTDWeightedMS = 500
			r.AWPTTDSamples, r.AWPTTDWeightedMS = 20, 190
		}, "watch"},
		{"plausible skilled awp timing stays normal", func(r *PlayerStatsReportRow) {
			r.NonAWPTTDWeightedMS = 500
			r.AWPTTDSamples, r.AWPTTDWeightedMS = 20, 200
		}, "normal"},
		{"awp ttd normal", func(r *PlayerStatsReportRow) {
			r.NonAWPTTDWeightedMS = 500
			r.AWPTTDSamples, r.AWPTTDWeightedMS = 20, 300
		}, "normal"},
		{"awp flags without being an awper", func(r *PlayerStatsReportRow) {
			r.IsAWPer = false
			r.NonAWPTTDWeightedMS = 500
			r.AWPTTDSamples, r.AWPTTDWeightedMS = 20, 180
		}, "cheater"},
		{"insufficient awp sample stays normal", func(r *PlayerStatsReportRow) {
			r.NonAWPTTDWeightedMS = 500
			r.AWPTTDSamples, r.AWPTTDWeightedMS = 5, 200
		}, "normal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			row.TriggeredRules = nil
			tc.mutate(&row)
			flagPlayer(&row, config)
			if !row.Eligible {
				t.Fatal("expected eligible")
			}
			if row.Status != tc.want {
				t.Fatalf("status = %q, want %q (%+v)", row.Status, tc.want, row)
			}
		})
	}
}

func TestSuspicionScoreCalibration(t *testing.T) {
	config := DefaultSuspicionConfig()
	base := PlayerStatsReportRow{
		DemoCount: 3, Shots: 100, NonAWPTTDSamples: 20,
		NonAWPTTDWeightedMS: 500, Kills: 10, Deaths: 20,
		Accuracy: .15, DamageEvents: 40, HeadHitRate: .20,
	}
	cases := []struct {
		name   string
		mutate func(*PlayerStatsReportRow)
		status string
		min    float64
		max    float64
	}{
		{"healthy aggregate", func(*PlayerStatsReportRow) {}, "normal", 0, config.ScoreWatchThreshold},
		{"borderline rifle timing", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 350 }, "watch", config.ScoreWatchThreshold, config.ScoreCheaterThreshold},
		{"impossible rifle timing", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 300 }, "cheater", 85, 101},
		{"timing and accuracy remain review-only", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS, r.Accuracy = 350, .30 }, "watch", config.ScoreWatchThreshold, config.ScoreCheaterThreshold},
		{"impossible reaction", func(r *PlayerStatsReportRow) { r.NonAWPReactionSamples, r.NonAWPReactionWeightedMS = 20, 180 }, "cheater", 85, 101},
		{"extreme head-hit rate cannot flag alone", func(r *PlayerStatsReportRow) { r.HeadHitRate = .62 }, "normal", 0, config.ScoreWatchThreshold},
		{"K/D cannot flag alone", func(r *PlayerStatsReportRow) { r.Kills, r.Deaths = 40, 10 }, "normal", 0, config.ScoreWatchThreshold},
		{"plausible skilled AWP timing", func(r *PlayerStatsReportRow) { r.AWPTTDSamples, r.AWPTTDWeightedMS = 100, 200 }, "normal", 0, config.ScoreWatchThreshold},
		{"AWP cheater anchor", func(r *PlayerStatsReportRow) { r.AWPTTDSamples, r.AWPTTDWeightedMS = 100, 180 }, "cheater", config.ScoreCheaterThreshold, 101},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.mutate(&row)
			flagPlayer(&row, config)
			t.Logf("status=%s score=%.2f timing=%.2f precision=%.2f performance=%.2f", row.Status, row.SuspicionScore, row.TimingScore, row.PrecisionScore, row.PerformanceScore)
			if row.Status != tc.status {
				t.Fatalf("status=%s score=%.2f, want %s", row.Status, row.SuspicionScore, tc.status)
			}
			if row.SuspicionScore < tc.min || row.SuspicionScore >= tc.max {
				t.Fatalf("score=%.2f, want [%.2f, %.2f)", row.SuspicionScore, tc.min, tc.max)
			}
		})
	}
}

func TestAngularError(t *testing.T) {
	attacker := playerFrameState{pos: r3.Vector{}, yaw: 0, pitch: 0}
	target := playerFrameState{pos: r3.Vector{X: 100}}
	if got := angularError(attacker, target); got > 2 {
		t.Fatalf("angular error = %v, want near zero", got)
	}
	attacker.yaw = 90
	if got := angularError(attacker, target); got < 89 || got > 91 {
		t.Fatalf("angular error = %v, want near 90", got)
	}
}

func TestStoreAndAggregateMultipleDemosBySteamID(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	steamID := uint64(76561198000000001)
	for index, name := range []string{"Alice", "Alice2", "Alice"} {
		fileName := "demo" + string(rune('a'+index))
		match := &Match{Checksum: string(rune('a' + index)), DemoFilePath: fileName + ".dem", DemoFileName: fileName, MapName: "de_test", Date: time.Unix(int64(index), 0), TickRate: 64, BuildNumber: 1, Source: constants.DemoSourceValve}
		stats := DemoStats{
			Players: map[uint64]*DemoPlayerStats{steamID: {SteamID64: steamID, Name: name, Rounds: 12, Shots: 40, HitShots: 20, DamageEvents: 20, HeadHitEvents: 10, Kills: 15, Deaths: 9, FirstBulletEncounters: 20, FirstBulletHeadHits: 5, TTDSamples: 20, TTDSumMS: 3000}},
			Weapons: map[uint64]map[string]*DemoWeaponStats{steamID: {
				"AK-47": {SteamID64: steamID, WeaponName: "AK-47", Shots: 30, HitShots: 15, DamageEvents: 15, HeadHitEvents: 10, Kills: 10},
				"AWP":   {SteamID64: steamID, WeaponName: "AWP", Shots: 10, HitShots: 5, DamageEvents: 5, Kills: 5},
			}},
		}
		for n := 0; n < 20; n++ {
			weapon, ttd := "AK-47", 200.0
			if n < 5 {
				weapon, ttd = "AWP", 100
			}
			stats.Encounters = append(stats.Encounters, DemoEncounter{AttackerSteamID64: steamID, VictimSteamID64: 2, TTDMS: ttd, TTDConfirmedMS: 120, ReactionTimeMS: 100, ConfirmedAngle: 3, FirstShotAngle: 1, WeaponName: weapon})
		}
		stats.Encounters = append(stats.Encounters, DemoEncounter{AttackerSteamID64: steamID, VictimSteamID64: 2, TTDMS: 1500, ReactionTimeMS: 1500})
		if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	report, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(report.Players))
	}
	row := report.Players[0]
	if row.DemoCount != 3 || row.Shots != 120 || row.Kills != 45 || row.Deaths != 27 {
		t.Fatalf("bad aggregate: %+v", row)
	}
	if row.TTDWeightedMS != 200 || row.TTDSamples != 60 || row.ReactionWeightedMS != 100 || row.ReactionSamples != 60 || !row.Eligible {
		t.Fatalf("bad TTD/reaction/eligibility: %+v", row)
	}
	if !row.IsAWPer || row.AWPKills != 15 || math.Abs(row.AWPKillRate-1.0/3.0) > .001 || row.AWPTTDMedianMS != 100 || row.NonAWPTTDMedianMS != 200 {
		t.Fatalf("bad AWP role/TTD split: %+v", row)
	}
	if len(row.Names) != 2 {
		t.Fatalf("aliases = %#v, want two", row.Names)
	}
	if len(report.Weapons) != 2 {
		t.Fatalf("bad weapon aggregate: %+v", report.Weapons)
	}
}

func TestStoreReplacesExistingDemoChecksum(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	match := &Match{Checksum: "same", DemoFilePath: "a.dem", DemoFileName: "a", Date: time.Now(), Source: constants.DemoSourceValve}
	stats := DemoStats{Players: map[uint64]*DemoPlayerStats{1: {SteamID64: 1, Name: "one", Shots: 10}}, Weapons: map[uint64]map[string]*DemoWeaponStats{}}
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	stats.Players[1].Shots = 25
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	var demos, shots int
	if err := db.QueryRow(`SELECT COUNT(*) FROM demos`).Scan(&demos); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT SUM(shots) FROM player_demo_stats`).Scan(&shots); err != nil {
		t.Fatal(err)
	}
	if demos != 1 || shots != 25 {
		t.Fatalf("demos=%d shots=%d, want 1/25", demos, shots)
	}
}

func TestStoreReplacesDemoWithSameFileNameAndMap(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	stats := DemoStats{Players: map[uint64]*DemoPlayerStats{1: {SteamID64: 1, Name: "one", Shots: 10}}, Weapons: map[uint64]map[string]*DemoWeaponStats{}}
	// Same demo file re-analyzed with a different checksum (e.g. the file size
	// changed between machines) must replace the previous analysis.
	first := &Match{Checksum: "old", DemoFilePath: "auto-1.dem", DemoFileName: "auto-1", MapName: "de_mirage", Date: time.Now(), Source: constants.DemoSourceValve}
	if err := storeAnalyzedDemo(ctx, db, first, stats); err != nil {
		t.Fatal(err)
	}
	stats.Players[1].Shots = 30
	second := &Match{Checksum: "new", DemoFilePath: "auto-1.dem", DemoFileName: "auto-1", MapName: "de_mirage", Date: time.Now(), Source: constants.DemoSourceValve}
	if err := storeAnalyzedDemo(ctx, db, second, stats); err != nil {
		t.Fatal(err)
	}
	// A demo with the same file name on another map must not be replaced.
	other := &Match{Checksum: "other", DemoFilePath: "auto-1.dem", DemoFileName: "auto-1", MapName: "de_dust2", Date: time.Now(), Source: constants.DemoSourceValve}
	if err := storeAnalyzedDemo(ctx, db, other, stats); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var demos int
	if err := db.QueryRow(`SELECT COUNT(*) FROM demos`).Scan(&demos); err != nil {
		t.Fatal(err)
	}
	if demos != 2 {
		t.Fatalf("demos=%d, want 2 (replaced same-name same-map, kept other map)", demos)
	}
	var shots int
	if err := db.QueryRow(`SELECT s.shots FROM player_demo_stats s JOIN demos d ON d.id=s.demo_id WHERE d.checksum='new'`).Scan(&shots); err != nil {
		t.Fatal(err)
	}
	if shots != 30 {
		t.Fatalf("shots=%d, want 30 from the newer analysis", shots)
	}
}

func TestDemosEnabledColumnDefaultsToOne(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	match := &Match{Checksum: "c1", DemoFilePath: "a.dem", DemoFileName: "a", Date: time.Now(), Source: constants.DemoSourceValve}
	stats := DemoStats{Players: map[uint64]*DemoPlayerStats{1: {SteamID64: 1, Name: "one", Shots: 10}}, Weapons: map[uint64]map[string]*DemoWeaponStats{}}
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	var enabled int
	if err := db.QueryRow(`SELECT enabled FROM demos WHERE checksum='c1'`).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("enabled=%d, want 1", enabled)
	}
}

func TestQualityWarningAutoDisablesDemoAndAllowsManualOverride(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	players := make(map[uint64]*DemoPlayerStats)
	for id := uint64(1); id <= 5; id++ {
		players[id] = &DemoPlayerStats{SteamID64: id, Name: "player", Rounds: 10, Shots: 30}
	}
	match := &Match{Checksum: "quality1", DemoFilePath: "quality.dem", DemoFileName: "quality", MapName: "de_test", Date: time.Now(), TickRate: 64, Source: constants.DemoSourceValve}
	stats := DemoStats{Players: players, Weapons: map[uint64]map[string]*DemoWeaponStats{}, Encounters: demoTimingEncounters(5, 10, 240, 140)}
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	var enabled bool
	var status string
	if err := db.QueryRow(`SELECT enabled,quality_status FROM demos WHERE checksum='quality1'`).Scan(&enabled, &status); err != nil {
		t.Fatal(err)
	}
	if enabled || status != demoQualityStatusWarning {
		t.Fatalf("enabled=%v status=%q, want false/warning", enabled, status)
	}
	db.Close()

	report, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Players) != 0 || len(report.Demos) != 1 || report.Demos[0].QualityStatus != demoQualityStatusWarning {
		t.Fatalf("warning demo leaked into stats or metadata is missing: %+v", report)
	}
	if err := SetDemoEnabled(ctx, dbPath, "quality1", true); err != nil {
		t.Fatal(err)
	}
	report, err = buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Players) != 5 || !report.Demos[0].Enabled || report.Demos[0].QualityStatus != demoQualityStatusWarning {
		t.Fatalf("manual override did not persist: %+v", report)
	}
}

func TestDeleteDemoRemovesAllStats(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	steamID := uint64(76561198000000001)
	match := &Match{Checksum: "del1", DemoFilePath: "/tmp/a.dem", DemoFileName: "a", MapName: "de_test", Date: time.Now(), Source: constants.DemoSourceValve}
	stats := DemoStats{
		Players:    map[uint64]*DemoPlayerStats{steamID: {SteamID64: steamID, Name: "Alice", Rounds: 10, Shots: 50}},
		Weapons:    map[uint64]map[string]*DemoWeaponStats{steamID: {"AK-47": {SteamID64: steamID, WeaponName: "AK-47", Shots: 50}}},
		Encounters: []DemoEncounter{{AttackerSteamID64: steamID, VictimSteamID64: 2, TTDMS: 150}},
		Evidence:   []DemoEvidence{{SteamID64: steamID, VictimID: 2, Kind: "test", Value: 1, Details: "{}"}},
	}
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	db.Close()

	path, err := DeleteDemo(ctx, dbPath, "del1")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/a.dem" {
		t.Fatalf("path=%q, want /tmp/a.dem", path)
	}
	if _, err := DeleteDemo(ctx, dbPath, "del1"); !errors.Is(err, ErrDemoNotFound) {
		t.Fatalf("err=%v, want ErrDemoNotFound", err)
	}
	db, err = openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"demos", "player_demo_stats", "encounters", "player_demo_weapon_stats", "evidence"} {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s still has %d rows after delete", table, count)
		}
	}
	report, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Players) != 0 {
		t.Fatalf("players=%d, want 0 after demo deletion", len(report.Players))
	}
}

func TestSetPlayerSaved(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	steamID := uint64(76561198000000001)
	match := &Match{Checksum: "s1", DemoFilePath: "a.dem", DemoFileName: "a", Date: time.Now(), Source: constants.DemoSourceValve}
	stats := DemoStats{Players: map[uint64]*DemoPlayerStats{steamID: {SteamID64: steamID, Name: "Alice", Rounds: 10, Shots: 10}}, Weapons: map[uint64]map[string]*DemoWeaponStats{}}
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := SetPlayerSaved(ctx, dbPath, steamID, true); err != nil {
		t.Fatal(err)
	}
	if err := SetPlayerSaved(ctx, dbPath, 42, true); !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("err = %v, want ErrPlayerNotFound", err)
	}
	report, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Players) != 1 || !report.Players[0].Saved {
		t.Fatalf("expected saved player, got %+v", report.Players)
	}
	if err := SetPlayerSaved(ctx, dbPath, steamID, false); err != nil {
		t.Fatal(err)
	}
	report, err = buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if report.Players[0].Saved {
		t.Fatalf("expected unsaved player, got %+v", report.Players[0])
	}
}

func TestMigrationAddsReactionTimeColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE encounters DROP COLUMN reaction_time_ms`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exists, err := sqliteColumnExists(db, "encounters", "reaction_time_ms")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("reaction_time_ms column missing after migration")
	}
}

func TestReportExcludesDisabledDemos(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	steamID := uint64(76561198000000001)
	for _, checksum := range []string{"d1", "d2"} {
		match := &Match{Checksum: checksum, DemoFilePath: checksum + ".dem", DemoFileName: checksum, MapName: "de_test", Date: time.Now(), TickRate: 64, Source: constants.DemoSourceValve}
		stats := DemoStats{
			Players:    map[uint64]*DemoPlayerStats{steamID: {SteamID64: steamID, Name: "Alice", Rounds: 10, Shots: 50, HitShots: 25}},
			Weapons:    map[uint64]map[string]*DemoWeaponStats{steamID: {"AK-47": {SteamID64: steamID, WeaponName: "AK-47", Shots: 50, HitShots: 25}}},
			Encounters: []DemoEncounter{{AttackerSteamID64: steamID, VictimSteamID64: 2, TTDMS: 150, ReactionTimeMS: 100}},
			Evidence:   []DemoEvidence{{SteamID64: steamID, VictimID: 2, Kind: "test", Value: 1, Details: "{}"}},
		}
		if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE demos SET enabled=0 WHERE checksum='d2'`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	report, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig(), IncludeEvidence: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Players) != 1 || report.Players[0].Shots != 50 || report.Players[0].DemoCount != 1 {
		t.Fatalf("expected stats from one demo only, got %+v", report.Players)
	}
	if report.Players[0].TTDSamples != 1 || report.Players[0].ReactionSamples != 1 {
		t.Fatalf("expected 1 ttd/reaction sample, got %+v", report.Players[0])
	}
	if len(report.Weapons) != 1 || report.Weapons[0].Shots != 50 {
		t.Fatalf("bad weapons: %+v", report.Weapons)
	}
	if len(report.Evidence) != 1 {
		t.Fatalf("evidence = %d, want 1", len(report.Evidence))
	}
	if len(report.Demos) != 2 {
		t.Fatalf("demo list = %d, want 2 (disabled demos stay listed)", len(report.Demos))
	}
	for _, d := range report.Demos {
		wantEnabled := d.Checksum == "d1"
		if d.Enabled != wantEnabled {
			t.Fatalf("demo %s enabled=%v", d.Checksum, d.Enabled)
		}
		if d.FileName == "" || d.ImportedAt == "" || d.Players != 1 || d.Rounds != 10 {
			t.Fatalf("bad demo metadata: %+v", d)
		}
	}
}

func TestSetDemoEnabled(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	match := &Match{Checksum: "t1", DemoFilePath: "a.dem", DemoFileName: "a", Date: time.Now(), Source: constants.DemoSourceValve}
	stats := DemoStats{Players: map[uint64]*DemoPlayerStats{1: {SteamID64: 1, Name: "one"}}, Weapons: map[uint64]map[string]*DemoWeaponStats{}}
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := SetDemoEnabled(ctx, dbPath, "t1", false); err != nil {
		t.Fatal(err)
	}
	report, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: dbPath, Config: DefaultSuspicionConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Demos) != 1 || report.Demos[0].Enabled {
		t.Fatalf("demo should be disabled: %+v", report.Demos)
	}
	if err := SetDemoEnabled(ctx, dbPath, "missing", true); !errors.Is(err, ErrDemoNotFound) {
		t.Fatalf("err = %v, want ErrDemoNotFound", err)
	}
}

func TestExportImportRoundtrip(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	db, err := openPlayerStatsDB(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	steamID := uint64(76561198000000001)
	match := &Match{Checksum: "x1", DemoFilePath: "x.dem", DemoFileName: "x", MapName: "de_test", Date: time.Unix(100, 0), TickRate: 64, BuildNumber: 2, Source: constants.DemoSourceValve}
	stats := DemoStats{
		Players:    map[uint64]*DemoPlayerStats{steamID: {SteamID64: steamID, Name: "Alice", Rounds: 10, Shots: 50, HitShots: 25, DamageEvents: 20, HeadHitEvents: 5}},
		Weapons:    map[uint64]map[string]*DemoWeaponStats{steamID: {"AK-47": {SteamID64: steamID, WeaponName: "AK-47", Shots: 50, HitShots: 25}}},
		Encounters: []DemoEncounter{{AttackerSteamID64: steamID, VictimSteamID64: 2, TTDMS: 150, ReactionTimeMS: 100, ConfirmedAngle: 3, WeaponName: "AK-47", Snap: true}},
		Evidence:   []DemoEvidence{{SteamID64: steamID, VictimID: 2, Kind: "test", Value: 1, Details: "{}"}},
	}
	if err := storeAnalyzedDemo(ctx, db, match, stats); err != nil {
		t.Fatal(err)
	}
	// disabled demos are excluded from exports
	match2 := &Match{Checksum: "x2", DemoFilePath: "y.dem", DemoFileName: "y", Date: time.Unix(200, 0), Source: constants.DemoSourceValve}
	if err := storeAnalyzedDemo(ctx, db, match2, DemoStats{Players: map[uint64]*DemoPlayerStats{}, Weapons: map[uint64]map[string]*DemoWeaponStats{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE demos SET enabled=0 WHERE checksum='x2'`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	export, err := ExportPlayerStatsData(ctx, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if export.Format != PlayerStatsExportFormat || export.Version != PlayerStatsExportVersion {
		t.Fatalf("bad envelope: %+v", export)
	}
	if len(export.Demos) != 1 || export.Demos[0].Checksum != "x1" {
		t.Fatalf("expected only enabled demo, got %+v", export.Demos)
	}

	targetPath := filepath.Join(t.TempDir(), "target.db")
	result, err := ImportPlayerStatsData(ctx, targetPath, export)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 1 || result.Skipped != 0 {
		t.Fatalf("result = %+v, want 1/0", result)
	}
	// re-import dedupes by checksum
	result, err = ImportPlayerStatsData(ctx, targetPath, export)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 0 || result.Skipped != 1 {
		t.Fatalf("result = %+v, want 0/1", result)
	}

	original, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: sourcePath, Config: DefaultSuspicionConfig(), IncludeEvidence: true})
	if err != nil {
		t.Fatal(err)
	}
	imported, err := buildPlayerStatsReport(ctx, PlayerStatsReportOptions{DatabasePath: targetPath, Config: DefaultSuspicionConfig(), IncludeEvidence: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Players) != 1 {
		t.Fatalf("players = %d, want 1", len(imported.Players))
	}
	a, b := original.Players[0], imported.Players[0]
	if a.Shots != b.Shots || a.TTDSamples != b.TTDSamples || a.ReactionSamples != b.ReactionSamples || a.Name != b.Name || a.SnapEvents != b.SnapEvents {
		t.Fatalf("roundtrip mismatch:\noriginal %+v\nimported %+v", a, b)
	}
	if len(imported.Weapons) != 1 || imported.Weapons[0].Shots != 50 {
		t.Fatalf("weapons mismatch: %+v", imported.Weapons)
	}
	if len(imported.Evidence) != 1 {
		t.Fatalf("evidence = %d, want 1", len(imported.Evidence))
	}
}

func TestImportRejectsBadEnvelope(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	if _, err := ImportPlayerStatsData(ctx, dbPath, &PlayerStatsExport{Format: "wrong", Version: 1}); err == nil {
		t.Fatal("expected format error")
	}
	if _, err := ImportPlayerStatsData(ctx, dbPath, &PlayerStatsExport{Format: PlayerStatsExportFormat, Version: 99}); err == nil {
		t.Fatal("expected version error")
	}
	if _, err := ImportPlayerStatsData(ctx, dbPath, &PlayerStatsExport{Format: PlayerStatsExportFormat, Version: 1, Demos: []ExportedDemo{{}}}); err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestFlagModeDefaultsAndValidation(t *testing.T) {
	config := DefaultSuspicionConfig()
	if config.FlagMode != "score" {
		t.Fatalf("default flag mode = %q, want score", config.FlagMode)
	}
	config.FlagMode = "manual"
	if err := ValidateSuspicionConfig(config); err != nil {
		t.Fatalf("manual mode rejected: %v", err)
	}
	config.FlagMode = "banana"
	if err := ValidateSuspicionConfig(config); err == nil {
		t.Fatal("expected invalid flag mode to be rejected")
	}
}

func TestFlagPlayerManualTiers(t *testing.T) {
	config := DefaultSuspicionConfig()
	config.FlagMode = "manual"
	base := PlayerStatsReportRow{DemoCount: 3, Shots: 100, NonAWPTTDSamples: 20, NonAWPTTDWeightedMS: 500, Kills: 10, Deaths: 20, Accuracy: .15, HeadHitRate: .20}

	cases := []struct {
		name   string
		mutate func(*PlayerStatsReportRow)
		want   string
	}{
		{"clean baseline", func(*PlayerStatsReportRow) {}, "normal"},
		{"ttd below cheater line", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 300 }, "cheater"},
		{"ttd in watch band", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 350 }, "watch"},
		// kd 4.0 >= eliteKdCheater fires its own cheater signal; no elite-stat promotion in manual mode
		{"ttd watch band with cheater kd", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 350; r.Kills, r.Deaths = 40, 10 }, "cheater"},
		{"insufficient ttd sample ignored", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 300; r.NonAWPTTDSamples = 5 }, "normal"},
		{"reaction below cheater line", func(r *PlayerStatsReportRow) { r.NonAWPReactionSamples, r.NonAWPReactionWeightedMS = 20, 180 }, "cheater"},
		{"reaction in watch band", func(r *PlayerStatsReportRow) { r.NonAWPReactionSamples, r.NonAWPReactionWeightedMS = 20, 220 }, "watch"},
		{"awp ttd below cheater line", func(r *PlayerStatsReportRow) { r.AWPTTDSamples, r.AWPTTDWeightedMS = 20, 170 }, "cheater"},
		{"awp ttd in watch band", func(r *PlayerStatsReportRow) { r.AWPTTDSamples, r.AWPTTDWeightedMS = 20, 200 }, "watch"},
		{"head hit alone flags in manual mode", func(r *PlayerStatsReportRow) { r.DamageEvents, r.HeadHitRate = 40, .62 }, "cheater"},
		{"head hit watch band", func(r *PlayerStatsReportRow) { r.DamageEvents, r.HeadHitRate = 40, .55 }, "watch"},
		{"head hit below sample gate ignored", func(r *PlayerStatsReportRow) { r.DamageEvents, r.HeadHitRate = 20, .62 }, "normal"},
		{"accuracy cheater", func(r *PlayerStatsReportRow) { r.Accuracy = .50 }, "cheater"},
		{"accuracy watch", func(r *PlayerStatsReportRow) { r.Accuracy = .35 }, "watch"},
		{"kd watch", func(r *PlayerStatsReportRow) { r.Kills, r.Deaths = 20, 10 }, "watch"},
		{"worst tier wins", func(r *PlayerStatsReportRow) { r.NonAWPTTDWeightedMS = 350; r.NonAWPReactionSamples, r.NonAWPReactionWeightedMS = 20, 180 }, "cheater"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			tc.mutate(&row)
			flagPlayer(&row, config)
			if !row.Eligible {
				t.Fatal("expected eligible")
			}
			if row.Status != tc.want {
				t.Fatalf("status = %q, want %q (%+v)", row.Status, tc.want, row)
			}
			if row.SuspicionScore != 0 {
				t.Fatalf("manual mode must not produce a score, got %.2f", row.SuspicionScore)
			}
			if row.Status != "normal" && len(row.TriggeredRules) == 0 {
				t.Fatal("flagged row must record triggered rules")
			}
		})
	}
}

func TestFlagPlayerManualRequiresMinimumSample(t *testing.T) {
	config := DefaultSuspicionConfig()
	config.FlagMode = "manual"
	row := PlayerStatsReportRow{DemoCount: 2, Shots: 99, NonAWPTTDSamples: 20, NonAWPTTDWeightedMS: 300}
	flagPlayer(&row, config)
	if row.Eligible || row.Status != "insufficient_sample" {
		t.Fatalf("unexpected eligibility: %+v", row)
	}
}
