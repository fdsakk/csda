package api

import (
	"testing"

	"github.com/fdsakk/csda/pkg/api/constants"
	"github.com/golang/geo/r3"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/common"
)

var (
	testAttacker = playerFrameState{steamID: 1, team: common.TeamTerrorists, alive: true, pos: r3.Vector{}}
	testTarget   = playerFrameState{steamID: 2, team: common.TeamCounterTerrorists, alive: true, pos: r3.Vector{X: 500}}
	testKey      = encounterKey{attacker: 1, target: 2}
)

func newTestCollector() *demoStatsCollector {
	c := newDemoStatsCollector(3, "")
	c.graceTicks = 32
	return c
}

// sample feeds one visibility sample per tick starting at tick 100.
func sample(c *demoStatsCollector, pattern string) {
	for i, r := range pattern {
		c.observeEncounter(testKey, 100+i, r == 'V', testAttacker, testTarget)
	}
}

func TestExposureNeedsConsecutiveVisibleSamples(t *testing.T) {
	c := newTestCollector()
	sample(c, "VVNV")
	if got := c.encounters[testKey].confirmedTick; got != 0 {
		t.Fatalf("V,V,N,V confirmed at tick %d, want no confirmation", got)
	}
	// Two more visible samples complete a fresh streak of three.
	c.observeEncounter(testKey, 104, true, testAttacker, testTarget)
	c.observeEncounter(testKey, 105, true, testAttacker, testTarget)
	if got := c.encounters[testKey].confirmedTick; got != 105 {
		t.Fatalf("confirmedTick=%d, want 105 (third consecutive visible sample)", got)
	}
}

func TestExposureIsConfirmedExactlyOnce(t *testing.T) {
	c := newTestCollector()
	sample(c, "VVVVVV")
	state := c.encounters[testKey]
	if state.firstTick != 100 || state.confirmedTick != 102 {
		t.Fatalf("firstTick=%d confirmedTick=%d, want 100/102", state.firstTick, state.confirmedTick)
	}
	// An occlusion inside the grace period keeps the encounter and its anchors.
	sample(c, "N")
	c.observeEncounter(testKey, 107, true, testAttacker, testTarget)
	c.observeEncounter(testKey, 108, true, testAttacker, testTarget)
	c.observeEncounter(testKey, 109, true, testAttacker, testTarget)
	state = c.encounters[testKey]
	if state.firstTick != 100 || state.confirmedTick != 102 {
		t.Fatalf("anchors moved after a short occlusion: first=%d confirmed=%d", state.firstTick, state.confirmedTick)
	}
}

func TestEncounterEndsAfterGracePeriod(t *testing.T) {
	c := newTestCollector()
	c.graceTicks = 3
	sample(c, "VVVNNN")
	if _, ok := c.encounters[testKey]; ok {
		t.Fatal("encounter must end after graceTicks occluded samples")
	}
}

func TestDeathEndsEncounter(t *testing.T) {
	c := newTestCollector()
	always := func() bool { return true }
	for tick := 100; tick < 104; tick++ {
		c.observePair(testKey, tick, testAttacker, testTarget, always)
	}
	c.encounters[testKey].reacted = true

	dead := testTarget
	dead.alive = false
	c.observePair(testKey, 104, testAttacker, dead, always)
	if _, ok := c.encounters[testKey]; ok {
		t.Fatal("encounter survived the death of the target")
	}
	// The next life starts a new encounter, not the reacted one.
	c.observePair(testKey, 600, testAttacker, testTarget, always)
	if state := c.encounters[testKey]; state == nil || state.reacted || state.firstTick != 600 {
		t.Fatalf("new life did not start a fresh encounter: %+v", state)
	}

	deadAttacker := testAttacker
	deadAttacker.alive = false
	c.observePair(testKey, 601, deadAttacker, testTarget, always)
	if _, ok := c.encounters[testKey]; ok {
		t.Fatal("encounter survived the death of the attacker")
	}
}

func TestFlashedAttackerSeesNothing(t *testing.T) {
	c := newTestCollector()
	c.graceTicks = 3
	always := func() bool { return true }
	c.observePair(testKey, 100, testAttacker, testTarget, always)
	flashed := testAttacker
	flashed.flashed = true
	for tick := 101; tick <= 103; tick++ {
		c.observePair(testKey, tick, flashed, testTarget, always)
	}
	if _, ok := c.encounters[testKey]; ok {
		t.Fatal("a flashed attacker kept the encounter alive")
	}
}

func TestNewRoundDropsLiveState(t *testing.T) {
	c := newTestCollector()
	analyzer := &Analyzer{currentRound: &Round{Number: 1}}
	c.syncRound(analyzer)
	sample(c, "VVVV")
	c.encounters[testKey].reacted = true
	c.shots[1] = []*trackedShot{{tick: 100}}
	c.pendingDamage[1] = nil

	c.syncRound(analyzer)
	if len(c.encounters) != 1 {
		t.Fatal("the live state must survive within the same round")
	}
	analyzer.currentRound = &Round{Number: 2}
	c.syncRound(analyzer)
	if len(c.encounters) != 0 || len(c.shots) != 0 || len(c.visCache) != 0 {
		t.Fatalf("round change left live state behind: %d encounters, %d shots", len(c.encounters), len(c.shots))
	}
}

func matchWithRounds(accepted int, discarded ...int) *Match {
	m := &Match{PlayersBySteamID: map[uint64]*Player{}, Kills: []*Kill{}}
	for number := 1; number <= accepted; number++ {
		m.Rounds = append(m.Rounds, &Round{Number: number, WinnerName: "A"})
	}
	for _, number := range discarded {
		m.Rounds = append(m.Rounds, &Round{Number: number})
	}
	return m
}

func addRoundActivity(c *demoStatsCollector, round int, shots int) {
	c.player(round, 1, "A").Shots += shots
	c.weapon(round, 1, constants.WeaponAK47).Shots += shots
	r := c.round(round)
	r.encounters = append(r.encounters, DemoEncounter{RoundNumber: round, AttackerSteamID64: 1, TTDMS: 100})
	r.evidence = append(r.evidence, DemoEvidence{RoundNumber: round, SteamID64: 1, Kind: "fast_ttd"})
}

func TestIncompleteRoundLeavesNoTraceInCollector(t *testing.T) {
	c := newTestCollector()
	addRoundActivity(c, 1, 4)
	addRoundActivity(c, 2, 7) // the round never ended
	m := matchWithRounds(1, 2)
	m.deleteIncompleteRounds()
	c.finalize(m)
	if got := c.result.Players[1].Shots; got != 4 {
		t.Fatalf("shots=%d, want only the 4 of the completed round", got)
	}
	if len(c.result.Encounters) != 1 || c.result.Encounters[0].RoundNumber != 1 || len(c.result.Evidence) != 1 {
		t.Fatalf("encounters=%+v evidence=%+v, want only round 1", c.result.Encounters, c.result.Evidence)
	}
	for _, weapons := range c.result.Weapons {
		for _, w := range weapons {
			if w.Shots != 4 {
				t.Fatalf("weapon shots=%d, want 4", w.Shots)
			}
		}
	}
}

func TestRestoredRoundCountsOnlyTheAcceptedAttempt(t *testing.T) {
	c := newTestCollector()
	addRoundActivity(c, 1, 3)
	addRoundActivity(c, 2, 10) // first attempt, cancelled by a backup restore
	c.resetRound(2)
	addRoundActivity(c, 2, 5) // the replayed round
	c.finalize(matchWithRounds(2))
	if got := c.result.Players[1].Shots; got != 8 {
		t.Fatalf("shots=%d, want 3+5", got)
	}
	if len(c.result.Encounters) != 2 {
		t.Fatalf("encounters=%d, want 2", len(c.result.Encounters))
	}

	// Restoring the same round twice yields the same totals.
	c2 := newTestCollector()
	addRoundActivity(c2, 1, 3)
	for attempt := 0; attempt < 3; attempt++ {
		addRoundActivity(c2, 2, 5)
		if attempt < 2 {
			c2.resetRound(2)
		}
	}
	c2.finalize(matchWithRounds(2))
	if got := c2.result.Players[1].Shots; got != 8 {
		t.Fatalf("shots after repeated restores=%d, want 8", got)
	}
}

func TestMatchRestartForgetsEverything(t *testing.T) {
	c := newTestCollector()
	addRoundActivity(c, 1, 3)
	addRoundActivity(c, 2, 3)
	c.reset()
	addRoundActivity(c, 1, 6)
	c.finalize(matchWithRounds(1))
	if got := c.result.Players[1].Shots; got != 6 {
		t.Fatalf("shots=%d, want 6 from the match after the restart", got)
	}
	if len(c.result.Encounters) != 1 {
		t.Fatalf("encounters=%d, want 1", len(c.result.Encounters))
	}
}

func TestFirstShotAngleMedianKeepsZeroDegrees(t *testing.T) {
	s := newPlayerEncounterSamples()
	s.add(1, 10, 100, 50, 0, 0, false)
	s.add(1, 10, 200, 100, 10, 10, false)
	s.add(1, 10, 300, 100, 10, -1, false) // no first shot attributed
	row := &PlayerStatsReportRow{}
	s.apply(row)
	if row.FirstShotMedianAngle != 5 || row.FirstShotAngleSamples != 2 {
		t.Fatalf("median=%v samples=%d, want 5 over 2 samples", row.FirstShotMedianAngle, row.FirstShotAngleSamples)
	}
}
