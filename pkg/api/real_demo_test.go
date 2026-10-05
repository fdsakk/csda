package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Tests on real demos. They are skipped unless CSDA_TEST_DEMO points to a CS2
// demo, or to several demos separated by commas (the geometry is read from
// ../../tris):
//
//	CSDA_TEST_DEMO=/path/to/match.dem go test ./pkg/api -run TestRealDemo -v
func realDemos(t *testing.T) []string {
	t.Helper()
	value := os.Getenv("CSDA_TEST_DEMO")
	if value == "" {
		t.Skip("CSDA_TEST_DEMO not set")
	}
	return strings.Split(value, ",")
}

func realDemo(t *testing.T) string {
	t.Helper()
	return realDemos(t)[0]
}

func analyzeRealDemo(t *testing.T, path string) (*Match, DemoStats) {
	t.Helper()
	result := analyzeOneDemoForStats(context.Background(), path, PlayerStatsBuildOptions{TrisDir: "../../tris", VisibilityConfirmationTicks: 3}, "test")
	if result.err != nil {
		t.Fatal(result.err)
	}
	return result.match, result.stats
}

func TestRealDemoStatsAreConsistentWithTheMatch(t *testing.T) {
	for _, path := range realDemos(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			checkStatsConsistentWithMatch(t, path)
		})
	}
}

func checkStatsConsistentWithMatch(t *testing.T, path string) {
	match, stats := analyzeRealDemo(t, path)

	rounds := make(map[int]bool)
	for _, round := range match.Rounds {
		rounds[round.Number] = true
	}
	for _, encounter := range stats.Encounters {
		if !rounds[encounter.RoundNumber] {
			t.Fatalf("encounter in round %d which is not part of the match", encounter.RoundNumber)
		}
		if encounter.ReactionTimeMS > encounter.TTDMS+0.001 && encounter.ReactionTimeMS >= 0 {
			t.Fatalf("reaction %.1f ms is longer than the time to damage %.1f ms: %+v", encounter.ReactionTimeMS, encounter.TTDMS, encounter)
		}
		if encounter.ConfirmedTick < encounter.FirstSpottedTick || encounter.DamageTick < encounter.ConfirmedTick {
			t.Fatalf("encounter ticks are not ordered: %+v", encounter)
		}
	}
	shotsFromWeapons := make(map[uint64]int)
	for id, weapons := range stats.Weapons {
		for _, weapon := range weapons {
			if weapon.HitShots > weapon.Shots {
				t.Fatalf("%d %s: %d hits out of %d shots", id, weapon.WeaponName, weapon.HitShots, weapon.Shots)
			}
			shotsFromWeapons[id] += weapon.Shots
		}
	}
	for id, player := range stats.Players {
		if player.HitShots > player.Shots {
			t.Fatalf("%d: %d hits out of %d shots", id, player.HitShots, player.Shots)
		}
		if shotsFromWeapons[id] != player.Shots {
			t.Fatalf("%d: weapon shots %d != player shots %d", id, shotsFromWeapons[id], player.Shots)
		}
	}
	// Shots and damage counted by the collector must be exactly those of the
	// match: both cover the rounds the match kept.
	wantShots, wantDamage := 0, 0
	for _, shot := range match.Shots {
		if shot.PlayerSteamID64 != 0 && validAimWeapon(shot.WeaponName) {
			wantShots++
		}
	}
	for _, damage := range match.Damages {
		if damage.AttackerSteamID64 != 0 && damage.AttackerSteamID64 != damage.VictimSteamID64 && damage.AttackerSide != damage.VictimSide && validAimWeapon(damage.WeaponName) {
			wantDamage++
		}
	}
	gotShots, gotDamage := 0, 0
	for _, player := range stats.Players {
		gotShots += player.Shots
		gotDamage += player.DamageEvents
	}
	if gotShots != wantShots || gotDamage != wantDamage {
		t.Fatalf("collector shots=%d damage=%d, match shots=%d damage=%d", gotShots, gotDamage, wantShots, wantDamage)
	}
	// The collector must see exactly the kills of the accepted rounds.
	kills := make(map[uint64]int)
	for _, kill := range match.Kills {
		if !rounds[kill.RoundNumber] {
			t.Fatalf("kill in round %d which is not part of the match", kill.RoundNumber)
		}
	}
	for _, player := range match.Players() {
		kills[player.SteamID64] = player.KillCount()
		if got := stats.Players[player.SteamID64].Kills; got != kills[player.SteamID64] {
			t.Fatalf("%s: collector kills %d != match kills %d", player.Name, got, kills[player.SteamID64])
		}
	}
	t.Logf("rounds=%d players=%d encounters=%d", len(match.Rounds), len(stats.Players), len(stats.Encounters))
}

func TestRealDemoAnalysisIsDeterministic(t *testing.T) {
	path := realDemo(t)
	_, first := analyzeRealDemo(t, path)
	_, second := analyzeRealDemo(t, path)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two analyses of the same demo gave different statistics")
	}
}

func TestRealDemoAnalysisIsCancellable(t *testing.T) {
	path := realDemo(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)
	start := time.Now()
	_, err := analyzeDemo(path, AnalyzeDemoOptions{Context: ctx, statsCollector: newDemoStatsCollector(3, "../../tris")})
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v after %v, want context.Canceled", err, elapsed)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("cancellation took %v", elapsed)
	}
	t.Logf("cancelled after %v", elapsed)
}

func TestRealDemoWithoutGeometryFailsBeforeParsing(t *testing.T) {
	path := realDemo(t)
	started := time.Now()
	result := processDemo(context.Background(), nil, path, PlayerStatsBuildOptions{
		TrisDir: t.TempDir(), VisibilityConfirmationTicks: 3, Force: true,
	})
	if result.err == nil || !strings.Contains(result.err.Error(), "no map geometry") {
		t.Fatalf("err=%v, want a missing geometry error", result.err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("took %s, the geometry check must run before the parser", elapsed)
	}
}

// player_hurt precedes weapon_fire within a tick, so the shot that caused a hit
// is only known after the damage event. Every hit must still be attributed to
// the shot fired in its own tick.
func TestRealDemoHitsAreAttributedToTheirShot(t *testing.T) {
	for _, path := range realDemos(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			match, stats := analyzeRealDemo(t, path)
			type attackerTick struct {
				attacker uint64
				tick     int
			}
			shotTicks := make(map[attackerTick]bool)
			for _, shot := range match.Shots {
				shotTicks[attackerTick{shot.PlayerSteamID64, shot.Tick}] = true
			}
			// One bullet can hit several players (penetration): it is attributed to
			// one encounter only, so the others legitimately stay without a shot.
			hitsInTick := make(map[attackerTick]int)
			for _, e := range stats.Encounters {
				hitsInTick[attackerTick{e.AttackerSteamID64, e.DamageTick}]++
			}
			withoutShot, unexplained := 0, 0
			for _, e := range stats.Encounters {
				if e.FirstShotTimeMS >= 0 {
					if e.ReactionTimeMS > e.TTDMS {
						t.Errorf("round %d: reaction %.1f ms is longer than the time to damage %.1f ms", e.RoundNumber, e.ReactionTimeMS, e.TTDMS)
					}
					continue
				}
				withoutShot++
				if key := (attackerTick{e.AttackerSteamID64, e.DamageTick}); shotTicks[key] && hitsInTick[key] < 2 {
					unexplained++
				}
			}
			if unexplained > 0 {
				t.Errorf("%d of %d encounters without a shot have a shot of the attacker in the damage tick and no other hit in it", unexplained, withoutShot)
			}
			t.Logf("%d encounters, %d without an attributed shot", len(stats.Encounters), withoutShot)
		})
	}
}
