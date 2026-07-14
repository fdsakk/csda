package api

import (
	"fmt"
	"math"
	"sort"

	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
	"github.com/akiver/cs-demo-analyzer/pkg/vis"
	"github.com/golang/geo/r3"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/events"
)

type playerFrameState struct {
	steamID  uint64
	name     string
	team     common.Team
	alive    bool
	ducking  bool
	flashed  bool
	airborne bool
	scoped   bool
	pos      r3.Vector
	yaw      float64
	pitch    float64
	speed    float64
}

type encounterKey struct {
	attacker uint64
	target   uint64
}

type encounterState struct {
	firstTick      int
	confirmedTick  int
	spottedTicks   int
	unspottedTicks int
	firstAngle     float64
	confirmedAngle float64
	distance       float64
	firstShotTick  int
	shotCount      int
	firstShotAngle float64
	reacted        bool
	snap           bool
}

type trackedShot struct {
	tick     int
	weaponID string
	hit      bool
	moving   bool
	airborne bool
	flashed  bool
	scoped   bool
}

type DemoEncounter struct {
	RoundNumber       int     `json:"roundNumber"`
	AttackerSteamID64 uint64  `json:"attackerSteamId"`
	VictimSteamID64   uint64  `json:"victimSteamId"`
	FirstSpottedTick  int     `json:"firstSpottedTick"`
	ConfirmedTick     int     `json:"confirmedTick"`
	DamageTick        int     `json:"damageTick"`
	TTDMS             float64 `json:"ttdMs"`
	TTDConfirmedMS    float64 `json:"ttdConfirmedMs"`
	FirstShotTimeMS   float64 `json:"firstShotTimeMs"`
	ReactionTimeMS    float64 `json:"reactionTimeMs"`
	FirstAngle        float64 `json:"firstAngle"`
	ConfirmedAngle    float64 `json:"confirmedAngle"`
	FirstShotAngle    float64 `json:"firstShotAngle"`
	DistanceMeters    float64 `json:"distanceMeters"`
	WeaponName        string  `json:"weaponName"`
	Snap              bool    `json:"snap"`
}

type DemoReaction struct {
	RoundNumber       int     `json:"roundNumber"`
	AttackerSteamID64 uint64  `json:"attackerSteamId"`
	VictimSteamID64   uint64  `json:"victimSteamId"`
	FirstSpottedTick  int     `json:"firstSpottedTick"`
	ConfirmedTick     int     `json:"confirmedTick"`
	ShotTick          int     `json:"shotTick"`
	ReactionTimeMS    float64 `json:"reactionTimeMs"`
	ConfirmedTimeMS   float64 `json:"confirmedTimeMs"`
	FirstAngle        float64 `json:"firstAngle"`
	ShotAngle         float64 `json:"shotAngle"`
	WeaponName        string  `json:"weaponName"`
}

type DemoEvidence struct {
	RoundNumber int     `json:"roundNumber"`
	Tick        int     `json:"tick"`
	SteamID64   uint64  `json:"steamId"`
	VictimID    uint64  `json:"victimSteamId"`
	Kind        string  `json:"kind"`
	Value       float64 `json:"value"`
	Details     string  `json:"details"`
}

type DemoPlayerStats struct {
	SteamID64             uint64  `json:"steamId"`
	Name                  string  `json:"name"`
	Rounds                int     `json:"rounds"`
	Shots                 int     `json:"shots"`
	HitShots              int     `json:"hitShots"`
	DamageEvents          int     `json:"damageEvents"`
	HeadHitEvents         int     `json:"headHitEvents"`
	Kills                 int     `json:"kills"`
	Deaths                int     `json:"deaths"`
	HeadshotKills         int     `json:"headshotKills"`
	SmokeKills            int     `json:"smokeKills"`
	WallKills             int     `json:"wallKills"`
	UnspottedDamageEvents int     `json:"unspottedDamageEvents"`
	FirstBulletEncounters int     `json:"firstBulletEncounters"`
	FirstBulletHeadHits   int     `json:"firstBulletHeadHits"`
	SnapEvents            int     `json:"snapEvents"`
	TTDSamples            int     `json:"ttdSamples"`
	TTDSumMS              float64 `json:"ttdSumMs"`
	MovingShots           int     `json:"movingShots"`
	MovingHitShots        int     `json:"movingHitShots"`
	AirborneShots         int     `json:"airborneShots"`
	AirborneHitShots      int     `json:"airborneHitShots"`
	FlashedShots          int     `json:"flashedShots"`
	FlashedHitShots       int     `json:"flashedHitShots"`
	ScopedShots           int     `json:"scopedShots"`
	ScopedHitShots        int     `json:"scopedHitShots"`
}

type DemoWeaponStats struct {
	SteamID64     uint64 `json:"steamId"`
	WeaponName    string `json:"weaponName"`
	Shots         int    `json:"shots"`
	HitShots      int    `json:"hitShots"`
	DamageEvents  int    `json:"damageEvents"`
	HeadHitEvents int    `json:"headHitEvents"`
	Kills         int    `json:"kills"`
}

type DemoStats struct {
	Players    map[uint64]*DemoPlayerStats            `json:"players"`
	Encounters []DemoEncounter                        `json:"encounters"`
	Reactions  []DemoReaction                         `json:"reactions"`
	Evidence   []DemoEvidence                         `json:"evidence"`
	Weapons    map[uint64]map[string]*DemoWeaponStats `json:"weapons"`
}

type activeSmoke struct {
	entityID   int
	expireTick int
	occluder   vis.Sphere
}

type visCacheEntry struct {
	tick    int
	visible bool
}

type demoStatsCollector struct {
	confirmationTicks int
	trisDir           string
	visEngine         *vis.Engine
	visLoadAttempted  bool
	graceTicks        int
	smokes            []activeSmoke
	occluders         []vis.Sphere
	visCache          map[encounterKey]visCacheEntry
	frames            map[uint64]playerFrameState
	encounters        map[encounterKey]*encounterState
	shots             map[uint64][]*trackedShot
	pendingDamage     map[uint64][]*Damage
	result            DemoStats
}

func newDemoStatsCollector(confirmationTicks int, trisDir string) *demoStatsCollector {
	if confirmationTicks < 1 {
		confirmationTicks = 3
	}
	return &demoStatsCollector{
		confirmationTicks: confirmationTicks,
		trisDir:           trisDir,
		visCache:          make(map[encounterKey]visCacheEntry),
		frames:            make(map[uint64]playerFrameState),
		encounters:        make(map[encounterKey]*encounterState),
		shots:             make(map[uint64][]*trackedShot),
		pendingDamage:     make(map[uint64][]*Damage),
		result: DemoStats{
			Players: make(map[uint64]*DemoPlayerStats),
			Weapons: make(map[uint64]map[string]*DemoWeaponStats),
		},
	}
}

func (c *demoStatsCollector) weapon(id uint64, name constants.WeaponName) *DemoWeaponStats {
	weapons := c.result.Weapons[id]
	if weapons == nil {
		weapons = make(map[string]*DemoWeaponStats)
		c.result.Weapons[id] = weapons
	}
	key := name.String()
	stats := weapons[key]
	if stats == nil {
		stats = &DemoWeaponStats{SteamID64: id, WeaponName: key}
		weapons[key] = stats
	}
	return stats
}

func (c *demoStatsCollector) player(id uint64, name string) *DemoPlayerStats {
	stats := c.result.Players[id]
	if stats == nil {
		stats = &DemoPlayerStats{SteamID64: id, Name: name}
		c.result.Players[id] = stats
	} else if name != "" {
		stats.Name = name
	}
	return stats
}

func playerState(p *common.Player) playerFrameState {
	pos := p.Position()
	velocity := p.Velocity()
	return playerFrameState{
		steamID:  p.SteamID64,
		name:     p.Name,
		team:     p.Team,
		alive:    p.IsAlive(),
		ducking:  p.IsDucking(),
		flashed:  p.FlashDurationTimeRemaining() > 0,
		airborne: p.IsAirborne(),
		scoped:   p.IsScoped(),
		pos:      pos,
		yaw:      float64(p.ViewDirectionX()),
		pitch:    normalizePitch(float64(p.ViewDirectionY())),
		speed:    math.Hypot(velocity.X, velocity.Y),
	}
}

func normalizePitch(pitch float64) float64 {
	if pitch > 180 {
		return pitch - 360
	}
	return pitch
}

func normalizeAngle(angle float64) float64 {
	for angle > 180 {
		angle -= 360
	}
	for angle < -180 {
		angle += 360
	}
	return angle
}

func eyePosition(s playerFrameState) r3.Vector {
	z := 64.0
	if s.ducking {
		z = 46.0
	}
	return s.pos.Add(r3.Vector{Z: z})
}

func aimPoint(s playerFrameState) r3.Vector {
	z := 62.0
	if s.ducking {
		z = 44.0
	}
	return s.pos.Add(r3.Vector{Z: z})
}

func angularError(attacker, target playerFrameState) float64 {
	delta := aimPoint(target).Sub(eyePosition(attacker))
	desiredYaw := math.Atan2(delta.Y, delta.X) * 180 / math.Pi
	desiredPitch := -math.Atan2(delta.Z, math.Hypot(delta.X, delta.Y)) * 180 / math.Pi
	dy := normalizeAngle(desiredYaw - attacker.yaw)
	dp := normalizeAngle(desiredPitch - attacker.pitch)
	return math.Hypot(dy, dp)
}

func distanceMeters(a, b playerFrameState) float64 {
	d := b.pos.Sub(a.pos)
	return math.Sqrt(d.X*d.X+d.Y*d.Y+d.Z*d.Z) * 0.01905
}

// Approximation of the CS2 volumetric smoke cloud used to block vision rays.
const (
	smokeRadius          = 155.0
	smokeCenterZOffset   = 55.0
	smokeLifetimeSeconds = 22.0
)

func (c *demoStatsCollector) onSmokeStart(analyzer *Analyzer, entityID int, position r3.Vector) {
	lifetimeTicks := int(smokeLifetimeSeconds * analyzer.parser.TickRate())
	c.smokes = append(c.smokes, activeSmoke{
		entityID:   entityID,
		expireTick: analyzer.currentTick() + lifetimeTicks,
		occluder:   vis.Sphere{Center: position.Add(r3.Vector{Z: smokeCenterZOffset}), Radius: smokeRadius},
	})
}

func (c *demoStatsCollector) onSmokeExpired(entityID int) {
	for i, smoke := range c.smokes {
		if smoke.entityID == entityID {
			c.smokes = append(c.smokes[:i], c.smokes[i+1:]...)
			return
		}
	}
}

// visible reports whether the attacker can see the target: at least one target
// body point inside the attacker FOV with a line of sight clear of map
// geometry and smokes. Falls back to the server spotted flag when no map
// geometry is available.
func (c *demoStatsCollector) visible(a, t playerFrameState, attacker, target *common.Player) bool {
	if c.visEngine == nil {
		return attacker.HasSpotted(target)
	}
	eye := eyePosition(a)
	points := [3]r3.Vector{eyePosition(t), aimPoint(t), t.pos.Add(r3.Vector{Z: 12})}
	for _, point := range points {
		if vis.InFOV(eye, a.yaw, a.pitch, point) && c.visEngine.LineOfSight(eye, point, c.occluders) {
			return true
		}
	}
	return false
}

func (c *demoStatsCollector) onFrame(analyzer *Analyzer) {
	if !c.visLoadAttempted {
		c.visLoadAttempted = true
		engine, err := vis.LoadEngine(c.trisDir, analyzer.match.MapName)
		if err != nil {
			fmt.Printf("player stats: no map geometry for %q (%v), falling back to spotted flag\n", analyzer.match.MapName, err)
		} else {
			c.visEngine = engine
		}
	}
	if c.graceTicks == 0 {
		// Tolerate up to ~0.5s of occlusion before an encounter resets.
		c.graceTicks = max(3, int(analyzer.parser.TickRate()/2))
	}
	tick := analyzer.currentTick()
	if len(c.smokes) > 0 {
		remaining := c.smokes[:0]
		for _, smoke := range c.smokes {
			if smoke.expireTick > tick {
				remaining = append(remaining, smoke)
			}
		}
		c.smokes = remaining
	}
	c.occluders = c.occluders[:0]
	for _, smoke := range c.smokes {
		c.occluders = append(c.occluders, smoke.occluder)
	}

	players := analyzer.parser.GameState().Participants().Playing()
	current := make(map[uint64]playerFrameState, len(players))
	byID := make(map[uint64]*common.Player, len(players))
	for _, p := range players {
		if p.SteamID64 == 0 {
			continue
		}
		state := playerState(p)
		current[p.SteamID64] = state
		byID[p.SteamID64] = p
		c.player(p.SteamID64, p.Name)
	}

	for attackerID, attacker := range byID {
		a := current[attackerID]
		if !a.alive || a.flashed {
			continue
		}
		for targetID, target := range byID {
			t := current[targetID]
			if attackerID == targetID || !t.alive || a.team == t.team {
				continue
			}
			key := encounterKey{attacker: attackerID, target: targetID}
			state := c.encounters[key]
			// Raycasts are the analysis hot spot; reuse each pair's result for
			// one extra tick (~16ms error on encounter anchors, well below the
			// 3-tick confirmation window).
			spotted, cached := false, false
			if entry, ok := c.visCache[key]; ok && tick-entry.tick < 2 {
				spotted, cached = entry.visible, true
			}
			if !cached {
				spotted = c.visible(a, t, attacker, target)
				c.visCache[key] = visCacheEntry{tick: tick, visible: spotted}
			}
			if spotted {
				if state == nil || state.unspottedTicks >= c.graceTicks {
					state = &encounterState{firstTick: tick, firstShotTick: -1, firstAngle: angularError(a, t), distance: distanceMeters(a, t)}
					c.encounters[key] = state
				}
				state.unspottedTicks = 0
				state.spottedTicks++
				if state.spottedTicks == c.confirmationTicks {
					state.confirmedTick = tick
					state.confirmedAngle = angularError(a, t)
				}
			} else if state != nil {
				state.unspottedTicks++
				if state.unspottedTicks >= c.graceTicks {
					delete(c.encounters, key)
				}
			}
		}
	}
	c.frames = current
}

func validAimWeapon(name constants.WeaponName) bool {
	switch name {
	case constants.WeaponUnknown, constants.WeaponWorld, constants.WeaponKnife, constants.WeaponZeus,
		constants.WeaponBomb, constants.WeaponDefuseKit, constants.WeaponKevlar, constants.WeaponHelmet,
		constants.WeaponDecoy, constants.WeaponFlashbang, constants.WeaponHEGrenade,
		constants.WeaponIncendiary, constants.WeaponMolotov, constants.WeaponSmoke:
		return false
	default:
		return true
	}
}

func (c *demoStatsCollector) onShot(analyzer *Analyzer, shot *Shot) {
	if shot.PlayerSteamID64 == 0 || !validAimWeapon(shot.WeaponName) {
		return
	}
	stats := c.player(shot.PlayerSteamID64, shot.PlayerName)
	stats.Shots++
	c.weapon(shot.PlayerSteamID64, shot.WeaponName).Shots++
	frame := c.frames[shot.PlayerSteamID64]
	moving := frame.speed > 80
	if moving {
		stats.MovingShots++
	}
	if frame.airborne {
		stats.AirborneShots++
	}
	if frame.flashed {
		stats.FlashedShots++
	}
	if frame.scoped {
		stats.ScopedShots++
	}
	c.shots[shot.PlayerSteamID64] = append(c.shots[shot.PlayerSteamID64], &trackedShot{tick: shot.Tick, weaponID: shot.WeaponID, moving: moving, airborne: frame.airborne, flashed: frame.flashed, scoped: frame.scoped})
	if pending := c.pendingDamage[shot.PlayerSteamID64]; len(pending) > 0 {
		remaining := pending[:0]
		for _, damage := range pending {
			if absInt(damage.Tick-shot.Tick) <= 2 && c.markHitShot(damage) {
				continue
			}
			remaining = append(remaining, damage)
		}
		c.pendingDamage[shot.PlayerSteamID64] = remaining
	}

	var selectedKey encounterKey
	var selected *encounterState
	var selectedAttacker, selectedTarget playerFrameState
	bestAngle := math.Inf(1)
	for key, encounter := range c.encounters {
		if key.attacker != shot.PlayerSteamID64 || encounter.confirmedTick == 0 || encounter.reacted {
			continue
		}
		a, aok := c.frames[key.attacker]
		t, tok := c.frames[key.target]
		if !aok || !tok {
			continue
		}
		angle := angularError(a, t)
		if angle < bestAngle {
			bestAngle = angle
			selectedKey, selected = key, encounter
			selectedAttacker, selectedTarget = a, t
		}
	}
	if selected != nil {
		selected.shotCount++
		if selected.firstShotTick >= 0 {
			return
		}
		selected.firstShotTick = shot.Tick
		selected.firstShotAngle = angularError(selectedAttacker, selectedTarget)
		stats.FirstBulletEncounters++
		c.result.Reactions = append(c.result.Reactions, DemoReaction{
			RoundNumber: analyzer.currentRound.Number, AttackerSteamID64: selectedKey.attacker, VictimSteamID64: selectedKey.target,
			FirstSpottedTick: selected.firstTick, ConfirmedTick: selected.confirmedTick, ShotTick: shot.Tick,
			ReactionTimeMS: c.tickDeltaMS(analyzer, selected.firstTick, shot.Tick), ConfirmedTimeMS: c.tickDeltaMS(analyzer, selected.confirmedTick, shot.Tick),
			FirstAngle: selected.firstAngle, ShotAngle: selected.firstShotAngle, WeaponName: shot.WeaponName.String(),
		})
		durationMS := c.tickDeltaMS(analyzer, selected.firstTick, shot.Tick)
		if selected.firstAngle-selected.firstShotAngle >= 15 && durationMS <= 100 && selected.firstShotAngle <= 2 {
			selected.snap = true
			stats.SnapEvents++
			c.result.Evidence = append(c.result.Evidence, DemoEvidence{
				RoundNumber: analyzer.currentRound.Number, Tick: shot.Tick, SteamID64: selectedKey.attacker, VictimID: selectedKey.target,
				Kind: "snap", Value: selected.firstAngle - selected.firstShotAngle, Details: "aim reduction in <=100ms before first shot",
			})
		}
	}
}

func (c *demoStatsCollector) tickDeltaMS(analyzer *Analyzer, from, to int) float64 {
	return float64(to-from) * analyzer.parser.TickTime().Seconds() * 1000
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (c *demoStatsCollector) markHitShot(damage *Damage) bool {
	shots := c.shots[damage.AttackerSteamID64]
	mark := func(requireWeaponID bool) bool {
		for i := len(shots) - 1; i >= 0; i-- {
			shot := shots[i]
			if absInt(damage.Tick-shot.tick) > 2 || shot.hit {
				continue
			}
			if requireWeaponID && (damage.WeaponUniqueID == "" || shot.weaponID != damage.WeaponUniqueID) {
				continue
			}
			shot.hit = true
			player := c.player(damage.AttackerSteamID64, "")
			player.HitShots++
			if shot.moving {
				player.MovingHitShots++
			}
			if shot.airborne {
				player.AirborneHitShots++
			}
			if shot.flashed {
				player.FlashedHitShots++
			}
			if shot.scoped {
				player.ScopedHitShots++
			}
			c.weapon(damage.AttackerSteamID64, damage.WeaponName).HitShots++
			return true
		}
		return false
	}
	// Community-server demos may expose different entity IDs in weapon_fire and
	// player_hurt. Prefer the exact match and fall back to the closest shot.
	if mark(true) {
		return true
	}
	return mark(false)
}

func (c *demoStatsCollector) onDamage(analyzer *Analyzer, damage *Damage) {
	if damage.AttackerSteamID64 == 0 || damage.AttackerSteamID64 == damage.VictimSteamID64 || damage.AttackerSide == damage.VictimSide || !validAimWeapon(damage.WeaponName) {
		return
	}
	stats := c.player(damage.AttackerSteamID64, "")
	stats.DamageEvents++
	weaponStats := c.weapon(damage.AttackerSteamID64, damage.WeaponName)
	weaponStats.DamageEvents++
	if damage.HitGroup == events.HitGroupHead {
		stats.HeadHitEvents++
		weaponStats.HeadHitEvents++
	}
	if !c.markHitShot(damage) {
		c.pendingDamage[damage.AttackerSteamID64] = append(c.pendingDamage[damage.AttackerSteamID64], damage)
	}

	key := encounterKey{attacker: damage.AttackerSteamID64, target: damage.VictimSteamID64}
	encounter := c.encounters[key]
	if encounter == nil || encounter.confirmedTick == 0 {
		stats.UnspottedDamageEvents++
		c.result.Evidence = append(c.result.Evidence, DemoEvidence{
			RoundNumber: damage.RoundNumber, Tick: damage.Tick, SteamID64: damage.AttackerSteamID64, VictimID: damage.VictimSteamID64,
			Kind: "damage_without_confirmed_spot", Value: float64(damage.HealthDamage), Details: damage.WeaponName.String(),
		})
		return
	}
	if encounter.reacted {
		return
	}
	encounter.reacted = true
	ttd := c.tickDeltaMS(analyzer, encounter.firstTick, damage.Tick)
	ttdConfirmed := c.tickDeltaMS(analyzer, encounter.confirmedTick, damage.Tick)
	if ttdConfirmed < 0 {
		return
	}
	firstShotMS := float64(-1)
	// Fall back to the damage tick when no shot was attributed to this
	// encounter (player_hurt can arrive before weapon_fire in the same tick,
	// or the shot was attributed to another encounter); a hitscan hit implies
	// a shot at the damage tick, keeping reaction <= TTD per encounter.
	reactionMS := ttd
	if encounter.firstShotTick >= 0 {
		firstShotMS = c.tickDeltaMS(analyzer, encounter.confirmedTick, encounter.firstShotTick)
		reactionMS = c.tickDeltaMS(analyzer, encounter.firstTick, encounter.firstShotTick)
		if encounter.shotCount == 1 && damage.Tick-encounter.firstShotTick <= 2 && damage.HitGroup == events.HitGroupHead {
			stats.FirstBulletHeadHits++
		}
	}
	stats.TTDSamples++
	stats.TTDSumMS += ttd
	c.result.Encounters = append(c.result.Encounters, DemoEncounter{
		RoundNumber: damage.RoundNumber, AttackerSteamID64: damage.AttackerSteamID64, VictimSteamID64: damage.VictimSteamID64,
		FirstSpottedTick: encounter.firstTick, ConfirmedTick: encounter.confirmedTick, DamageTick: damage.Tick,
		TTDMS: ttd, TTDConfirmedMS: ttdConfirmed, FirstShotTimeMS: firstShotMS, ReactionTimeMS: reactionMS,
		FirstAngle: encounter.firstAngle, ConfirmedAngle: encounter.confirmedAngle, FirstShotAngle: encounter.firstShotAngle,
		DistanceMeters: encounter.distance, WeaponName: damage.WeaponName.String(), Snap: encounter.snap,
	})
	if ttd >= 0 && ttd <= 190 {
		c.result.Evidence = append(c.result.Evidence, DemoEvidence{
			RoundNumber: damage.RoundNumber, Tick: damage.Tick, SteamID64: damage.AttackerSteamID64, VictimID: damage.VictimSteamID64,
			Kind: "fast_ttd", Value: ttd, Details: damage.WeaponName.String(),
		})
	}
}

func (c *demoStatsCollector) finalize(match *Match) {
	rounds := len(match.Rounds)
	for _, player := range match.Players() {
		stats := c.player(player.SteamID64, player.Name)
		stats.Rounds = rounds
		stats.Kills = player.KillCount()
		stats.Deaths = player.DeathCount()
		stats.HeadshotKills = player.HeadshotCount()
	}
	for _, kill := range match.Kills {
		if kill.KillerSteamID64 == 0 || kill.IsSuicide() || kill.IsTeamKill() {
			continue
		}
		stats := c.player(kill.KillerSteamID64, kill.KillerName)
		if kill.IsThroughSmoke {
			stats.SmokeKills++
		}
		if kill.PenetratedObjects > 0 {
			stats.WallKills++
		}
		if validAimWeapon(kill.WeaponName) {
			c.weapon(kill.KillerSteamID64, kill.WeaponName).Kills++
		}
	}
	// Keep report and database output stable.
	sort.Slice(c.result.Encounters, func(i, j int) bool {
		a, b := c.result.Encounters[i], c.result.Encounters[j]
		if a.RoundNumber != b.RoundNumber {
			return a.RoundNumber < b.RoundNumber
		}
		if a.DamageTick != b.DamageTick {
			return a.DamageTick < b.DamageTick
		}
		return a.AttackerSteamID64 < b.AttackerSteamID64
	})
}
