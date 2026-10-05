package api

import (
	"testing"

	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/common"
)

func kastMatch(kills ...*Kill) *Match {
	return &Match{Rounds: []*Round{{Number: 1}}, Kills: kills}
}

func TestKASTCountsDeathByTeammateAsDeath(t *testing.T) {
	m := kastMatch(&Kill{
		RoundNumber: 1, KillerSteamID64: 2, VictimSteamID64: 1,
		KillerSide: common.TeamTerrorists, VictimSide: common.TeamTerrorists,
	})
	if got := (&Player{SteamID64: 1, match: m}).KAST(); got != 0 {
		t.Fatalf("KAST=%g after a teamkill death, want 0", got)
	}
}

func TestKASTCountsSuicideAsDeath(t *testing.T) {
	m := kastMatch(&Kill{
		RoundNumber: 1, KillerSteamID64: 1, VictimSteamID64: 1,
		KillerSide: common.TeamTerrorists, VictimSide: common.TeamTerrorists,
	})
	if got := (&Player{SteamID64: 1, match: m}).KAST(); got != 0 {
		t.Fatalf("KAST=%g after a suicide, want 0", got)
	}
}

func TestKASTCredits(t *testing.T) {
	enemyKill := func(killer, victim uint64) *Kill {
		return &Kill{RoundNumber: 1, KillerSteamID64: killer, VictimSteamID64: victim, KillerSide: common.TeamTerrorists, VictimSide: common.TeamCounterTerrorists}
	}
	cases := []struct {
		name string
		kill *Kill
		want float32
	}{
		{"survived", enemyKill(9, 8), 100},
		{"killed with a kill", enemyKill(1, 8), 100},
		{"killed without any credit", &Kill{RoundNumber: 1, KillerSteamID64: 9, VictimSteamID64: 1, KillerSide: common.TeamCounterTerrorists, VictimSide: common.TeamTerrorists}, 0},
		{"assist", &Kill{RoundNumber: 1, KillerSteamID64: 9, VictimSteamID64: 8, AssisterSteamID64: 1, KillerSide: common.TeamTerrorists, VictimSide: common.TeamCounterTerrorists}, 100},
		{"traded death", &Kill{RoundNumber: 1, KillerSteamID64: 9, VictimSteamID64: 1, KillerSide: common.TeamCounterTerrorists, VictimSide: common.TeamTerrorists, IsTradeDeath: true}, 100},
	}
	for _, tc := range cases {
		if got := (&Player{SteamID64: 1, match: kastMatch(tc.kill)}).KAST(); got != tc.want {
			t.Errorf("%s: KAST=%g, want %g", tc.name, got, tc.want)
		}
	}
}

func TestClutchCountsBelongToTheClutcher(t *testing.T) {
	m := &Match{Clutches: []*Clutch{
		{ClutcherSteamID64: 1, OpponentCount: 1, HasWon: true},
		{ClutcherSteamID64: 2, OpponentCount: 2, HasWon: true},
		{ClutcherSteamID64: 1, OpponentCount: 2},
	}}
	p := &Player{SteamID64: 1, match: m}
	if got := p.OneVsOneCount(); got != 1 {
		t.Fatalf("1v1 count=%d, want 1", got)
	}
	if got := p.OneVsTwoCount(); got != 1 {
		t.Fatalf("1v2 count=%d, want 1 (the other player's 1v2 is not mine)", got)
	}
	if got := p.OneVsTwoWonCount(); got != 0 {
		t.Fatalf("1v2 won=%d, want 0", got)
	}
	if got := p.OneVsTwoLostCount(); got != 1 {
		t.Fatalf("1v2 lost=%d, want 1", got)
	}
	if got := p.OneVsThreeCount(); got != 0 {
		t.Fatalf("1v3 count=%d, want 0", got)
	}
}
