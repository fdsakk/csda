package api

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/fdsakk/csda/pkg/api/constants"
)

func TestGetPlayerEncounters(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	db, err := openPlayerStatsDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	const alice, bob, carol = uint64(76561198000000001), uint64(76561198000000002), uint64(76561198000000003)
	players := map[uint64]*DemoPlayerStats{
		alice: {SteamID64: alice, Name: "Alice", Rounds: 10}, bob: {SteamID64: bob, Name: "Bob", Rounds: 10}, carol: {SteamID64: carol, Name: "Carol", Rounds: 10},
	}
	store := func(checksum string, day int, encounters []DemoEncounter) {
		match := &Match{Checksum: checksum, DemoFilePath: checksum + ".dem", DemoFileName: checksum, MapName: "de_test", Date: time.Unix(int64(day)*86400, 0), TickRate: 64, Source: constants.DemoSourceValve}
		stats := DemoStats{Players: players, Weapons: map[uint64]map[string]*DemoWeaponStats{}, Encounters: encounters}
		if err := storeAnalyzedDemo(ctx, db, match, stats, checksum); err != nil {
			t.Fatal(err)
		}
	}
	store("a", 1, []DemoEncounter{
		// shot recorded 10 ticks (156.25 ms) after the first spotted tick
		{RoundNumber: 2, AttackerSteamID64: alice, VictimSteamID64: bob, FirstSpottedTick: 1000, ConfirmedTick: 1003, DamageTick: 1014, TTDMS: 218.75, TTDConfirmedMS: 171.875,
			FirstShotTimeMS: 109.375, ReactionTimeMS: 156.25, ConfirmedAngle: 3.5, FirstShotAngle: 0, DistanceMeters: 12, WeaponName: "AK-47"},
		// no shot recorded: reaction is the time to damage; AWP; slower than the 1000 ms window
		{RoundNumber: 1, AttackerSteamID64: alice, VictimSteamID64: carol, FirstSpottedTick: 500, ConfirmedTick: 503, DamageTick: 580, TTDMS: 1250, TTDConfirmedMS: 1203,
			FirstShotTimeMS: -1, ReactionTimeMS: 1250, ConfirmedAngle: 8, FirstShotAngle: -1, DistanceMeters: 30, WeaponName: constants.WeaponAWP.String()},
		{RoundNumber: 1, AttackerSteamID64: bob, VictimSteamID64: alice, FirstSpottedTick: 400, DamageTick: 420, TTDMS: 100, FirstShotTimeMS: -1, ReactionTimeMS: 100, FirstShotAngle: -1, WeaponName: "AK-47"},
	})
	store("b", 2, []DemoEncounter{
		{RoundNumber: 1, AttackerSteamID64: alice, VictimSteamID64: bob, FirstSpottedTick: 100, DamageTick: 120, TTDMS: 300, FirstShotTimeMS: -1, ReactionTimeMS: 300, FirstShotAngle: -1, WeaponName: "AK-47"},
	})
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := SetDemoEnabled(ctx, dbPath, "b", false); err != nil {
		t.Fatal(err)
	}

	rows, err := GetPlayerEncounters(ctx, dbPath, alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("%d encounters, want the 2 of Alice in the enabled demo (not Bob's, not the disabled demo's)", len(rows))
	}
	// ordered by demo, then round
	first, second := rows[0], rows[1]
	if first.RoundNumber != 1 || second.RoundNumber != 2 {
		t.Fatalf("rounds %d,%d, want 1,2", first.RoundNumber, second.RoundNumber)
	}

	if !first.ReactionEstimated || first.FirstShotTick != nil || first.FirstShotAngle != nil || !first.AWP || first.Counted {
		t.Fatalf("encounter without a shot: %+v", first)
	}
	if first.ReactionMS == nil || *first.ReactionMS != 1250 {
		t.Fatalf("estimated reaction = %v, want the time to damage 1250", first.ReactionMS)
	}
	if first.VictimName != "Carol" || first.VictimSteamID != "76561198000000003" || first.DemoChecksum != "a" || first.MapName != "de_test" {
		t.Fatalf("identity fields: %+v", first)
	}

	if second.ReactionEstimated || !second.Counted || second.AWP {
		t.Fatalf("encounter with a shot: %+v", second)
	}
	if second.FirstShotTick == nil || *second.FirstShotTick != 1010 {
		t.Fatalf("first shot tick = %v, want 1000 + 10 ticks", second.FirstShotTick)
	}
	if second.FirstShotAngle == nil || *second.FirstShotAngle != 0 {
		t.Fatalf("first shot angle = %v, want a recorded 0°", second.FirstShotAngle)
	}
	if second.FirstSpottedTick != 1000 || second.ConfirmedTick != 1003 || second.DamageTick != 1014 || second.TTDMS != 218.75 {
		t.Fatalf("ticks/timings: %+v", second)
	}

	if _, err := GetPlayerEncounters(ctx, dbPath, 123); !errors.Is(err, ErrPlayerNotFound) {
		t.Fatalf("unknown player: err=%v, want ErrPlayerNotFound", err)
	}
}
