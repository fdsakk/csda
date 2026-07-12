package api

import (
	"context"
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

func TestScorePlayerRequiresMinimumSampleAndIncludesThresholdBoundaries(t *testing.T) {
	config := DefaultSuspicionConfig()
	row := PlayerStatsReportRow{DemoCount: 2, Shots: 99, TTDSamples: 20, TTDWeightedMS: 190, TTDP10MS: 120, DamageEvents: 30, HeadHitRate: .40}
	scorePlayer(&row, config)
	if row.Eligible || row.Status != "insufficient_sample" {
		t.Fatalf("unexpected eligibility: %+v", row)
	}

	row.DemoCount, row.Shots = 3, 100
	scorePlayer(&row, config)
	if !row.Eligible {
		t.Fatal("expected row to be eligible")
	}
	if row.SuspicionScore != 55 {
		t.Fatalf("score = %d, want 55", row.SuspicionScore)
	}
	if row.Status != "review" {
		t.Fatalf("status = %q, want review", row.Status)
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
		match := &Match{Checksum: string(rune('a' + index)), DemoFilePath: "demo.dem", DemoFileName: "demo", MapName: "de_test", Date: time.Unix(int64(index), 0), TickRate: 64, BuildNumber: 1, Source: constants.DemoSourceValve}
		stats := DemoStats{
			Players: map[uint64]*DemoPlayerStats{steamID: {SteamID64: steamID, Name: name, Rounds: 12, Shots: 40, HitShots: 20, DamageEvents: 20, HeadHitEvents: 10, FirstBulletEncounters: 20, FirstBulletHeadHits: 5, TTDSamples: 20, TTDSumMS: 3000}},
			Weapons: map[uint64]map[string]*DemoWeaponStats{steamID: {"AK-47": {SteamID64: steamID, WeaponName: "AK-47", Shots: 40, HitShots: 20, DamageEvents: 20, HeadHitEvents: 10}}},
		}
		for n := 0; n < 20; n++ {
			stats.Encounters = append(stats.Encounters, DemoEncounter{AttackerSteamID64: steamID, VictimSteamID64: 2, TTDMS: 150, TTDConfirmedMS: 120, ConfirmedAngle: 3, FirstShotAngle: 1})
			stats.Reactions = append(stats.Reactions, DemoReaction{AttackerSteamID64: steamID, VictimSteamID64: 2, ReactionTimeMS: 100})
		}
		stats.Encounters = append(stats.Encounters, DemoEncounter{AttackerSteamID64: steamID, VictimSteamID64: 2, TTDMS: 1500})
		stats.Reactions = append(stats.Reactions, DemoReaction{AttackerSteamID64: steamID, VictimSteamID64: 2, ReactionTimeMS: 1500})
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
	if row.DemoCount != 3 || row.Shots != 120 {
		t.Fatalf("bad aggregate: %+v", row)
	}
	if row.TTDWeightedMS != 150 || row.TTDSamples != 60 || row.ReactionWeightedMS != 100 || row.ReactionSamples != 60 || !row.Eligible {
		t.Fatalf("bad TTD/reaction/eligibility: %+v", row)
	}
	if len(row.Names) != 2 {
		t.Fatalf("aliases = %#v, want two", row.Names)
	}
	if len(report.Weapons) != 1 || report.Weapons[0].Shots != 120 {
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
