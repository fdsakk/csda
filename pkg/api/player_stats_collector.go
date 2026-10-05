package api

import (
	"fmt"
	"math"
	"sort"

	"github.com/fdsakk/csda/pkg/api/constants"
	"github.com/fdsakk/csda/pkg/vis"
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
	round    int
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

// roundStats holds everything the collector measured during one round. Keeping
// it per round lets a round that is later discarded (backup restore, incomplete
// round, knife/warmup restart) be dropped without leaving traces in the totals.
type roundStats struct {
	players    map[uint64]*DemoPlayerStats
	weapons    map[uint64]map[string]*DemoWeaponStats
	encounters []DemoEncounter
	evidence   []DemoEvidence
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
	// deferredDamage holds the damage events of the current tick whose
	// encounter is resolved once the tick is complete: the demos emit
	// player_hurt before weapon_fire within a tick, so the shot that caused a
	// hit is only known after the damage event.
	deferredDamage []*Damage
	// liveRound is the round the live state above belongs to; the state is
	// dropped as soon as the analyzer moves on to another round.
	liveRound     *Round
	lastFrameTick int
	rounds        map[int]*roundStats
	names         map[uint64]string
	result        DemoStats
}

func newDemoStatsCollector(confirmationTicks int, trisDir string) *demoStatsCollector {
	if confirmationTicks < 1 {
		confirmationTicks = 3
	}
	c := &demoStatsCollector{
		confirmationTicks: confirmationTicks,
		trisDir:           trisDir,
		rounds:            make(map[int]*roundStats),
		names:             make(map[uint64]string),
	}
	c.resetLiveState()
	return c
}

func (c *demoStatsCollector) resetLiveState() {
	c.lastFrameTick = 0
	c.smokes = nil
	c.occluders = nil
	c.visCache = make(map[encounterKey]visCacheEntry)
	c.frames = make(map[uint64]playerFrameState)
	c.encounters = make(map[encounterKey]*encounterState)
	c.shots = make(map[uint64][]*trackedShot)
	c.pendingDamage = make(map[uint64][]*Damage)
	c.deferredDamage = nil
}

// syncRound drops the live state (running encounters, shot buffers, smokes)
// when the analyzer started a new round.
func (c *demoStatsCollector) syncRound(analyzer *Analyzer) {
	if c.liveRound != analyzer.currentRound {
		// damage received in the old round is resolved with the old round's state
		c.resolveDeferredDamage(analyzer)
		c.liveRound = analyzer.currentRound
		c.resetLiveState()
	}
}

// reset forgets everything, used when the analyzer restarts the whole match.
func (c *demoStatsCollector) reset() {
	c.rounds = make(map[int]*roundStats)
	c.resetLiveState()
}

// resetRound forgets one round, used when the round is restored from a backup.
func (c *demoStatsCollector) resetRound(roundNumber int) {
	delete(c.rounds, roundNumber)
	c.resetLiveState()
}

func (c *demoStatsCollector) round(number int) *roundStats {
	round := c.rounds[number]
	if round == nil {
		round = &roundStats{
			players: make(map[uint64]*DemoPlayerStats),
			weapons: make(map[uint64]map[string]*DemoWeaponStats),
		}
		c.rounds[number] = round
	}
	return round
}

func (c *demoStatsCollector) weapon(roundNumber int, id uint64, name constants.WeaponName) *DemoWeaponStats {
	round := c.round(roundNumber)
	weapons := round.weapons[id]
	if weapons == nil {
		weapons = make(map[string]*DemoWeaponStats)
		round.weapons[id] = weapons
	}
	key := name.String()
	stats := weapons[key]
	if stats == nil {
		stats = &DemoWeaponStats{SteamID64: id, WeaponName: key}
		weapons[key] = stats
	}
	return stats
}

func (c *demoStatsCollector) player(roundNumber int, id uint64, name string) *DemoPlayerStats {
	if name != "" {
		c.names[id] = name
	}
	round := c.round(roundNumber)
	stats := round.players[id]
	if stats == nil {
		stats = &DemoPlayerStats{SteamID64: id}
		round.players[id] = stats
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
	c.syncRound(analyzer)
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
	// All events of the finished tick are known now: resolve its damage before
	// this frame changes the encounter state.
	c.resolveDeferredDamage(analyzer)
	c.syncRound(analyzer)
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
	// A tick can be followed by extra frames; sample each tick once so that
	// consecutive samples are consecutive ticks.
	if tick == c.lastFrameTick {
		return
	}
	c.lastFrameTick = tick
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
		current[p.SteamID64] = playerState(p)
		byID[p.SteamID64] = p
		c.names[p.SteamID64] = p.Name
	}

	for attackerID, attacker := range byID {
		a := current[attackerID]
		for targetID, target := range byID {
			t := current[targetID]
			if attackerID == targetID || a.team == t.team {
				continue
			}
			c.observePair(encounterKey{attacker: attackerID, target: targetID}, tick, a, t, func() bool {
				return c.visible(a, t, attacker, target)
			})
		}
	}
	// A player that left the server ends all of its encounters.
	for key := range c.encounters {
		if _, ok := current[key.attacker]; !ok {
			c.dropEncounter(key)
		} else if _, ok := current[key.target]; !ok {
			c.dropEncounter(key)
		}
	}
	c.frames = current
}

// observePair advances the encounter of one attacker/target pair for the
// current frame. visible reports whether the attacker sees the target; it is the
// expensive part and is only evaluated when the pair can actually see anything.
func (c *demoStatsCollector) observePair(key encounterKey, tick int, a, t playerFrameState, visible func() bool) {
	if !a.alive || !t.alive {
		// A dead player ends the encounter; it must not carry its state over
		// to the next life.
		c.dropEncounter(key)
		return
	}
	// A flashed attacker cannot see the target: that is plain occlusion,
	// handled by the same grace period as a wall.
	spotted := false
	if !a.flashed {
		// Raycasts are the analysis hot spot; reuse each pair's result for one
		// extra tick (~16ms error on encounter anchors, well below the
		// confirmation window).
		if entry, ok := c.visCache[key]; ok && tick-entry.tick < 2 {
			spotted = entry.visible
		} else {
			spotted = visible()
			c.visCache[key] = visCacheEntry{tick: tick, visible: spotted}
		}
	}
	c.observeEncounter(key, tick, spotted, a, t)
}

func (c *demoStatsCollector) dropEncounter(key encounterKey) {
	delete(c.encounters, key)
	delete(c.visCache, key)
}

// observeEncounter feeds one visibility sample of an attacker/target pair into
// the encounter state machine. An encounter starts at the first sample where the
// target is visible and survives short occlusions (graceTicks samples), but the
// exposure is only confirmed after confirmationTicks consecutive visible samples.
// a and t are the attacker and target states at the time of the sample.
func (c *demoStatsCollector) observeEncounter(key encounterKey, tick int, spotted bool, a, t playerFrameState) {
	state := c.encounters[key]
	if !spotted {
		if state == nil {
			return
		}
		state.spottedTicks = 0
		state.unspottedTicks++
		if state.unspottedTicks >= c.graceTicks {
			c.dropEncounter(key)
		}
		return
	}
	if state == nil {
		state = &encounterState{firstTick: tick, firstShotTick: -1, firstAngle: angularError(a, t), distance: distanceMeters(a, t)}
		c.encounters[key] = state
	}
	state.unspottedTicks = 0
	state.spottedTicks++
	if state.confirmedTick == 0 && state.spottedTicks >= c.confirmationTicks {
		state.confirmedTick = tick
		state.confirmedAngle = angularError(a, t)
	}
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
	c.syncRound(analyzer)
	if shot.PlayerSteamID64 == 0 || !validAimWeapon(shot.WeaponName) {
		return
	}
	stats := c.player(shot.RoundNumber, shot.PlayerSteamID64, shot.PlayerName)
	stats.Shots++
	c.weapon(shot.RoundNumber, shot.PlayerSteamID64, shot.WeaponName).Shots++
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
	c.shots[shot.PlayerSteamID64] = append(c.shots[shot.PlayerSteamID64], &trackedShot{round: shot.RoundNumber, tick: shot.Tick, weaponID: shot.WeaponID, moving: moving, airborne: frame.airborne, flashed: frame.flashed, scoped: frame.scoped})
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
		c.registerFirstShot(analyzer, selectedKey, selected, shot.RoundNumber, shot.Tick, selectedAttacker, selectedTarget)
	}
}

// registerFirstShot records tick as the first shot fired at an encounter, and
// derives the first-bullet and snap statistics from the aim at that moment.
func (c *demoStatsCollector) registerFirstShot(analyzer *Analyzer, key encounterKey, encounter *encounterState, round, tick int, attacker, target playerFrameState) {
	encounter.firstShotTick = tick
	encounter.firstShotAngle = angularError(attacker, target)
	c.player(round, key.attacker, "").FirstBulletEncounters++
	durationMS := c.tickDeltaMS(analyzer, encounter.firstTick, tick)
	if encounter.firstAngle-encounter.firstShotAngle >= 15 && durationMS <= 100 && encounter.firstShotAngle <= 2 {
		encounter.snap = true
		c.player(round, key.attacker, "").SnapEvents++
		c.round(round).evidence = append(c.round(round).evidence, DemoEvidence{
			RoundNumber: round, Tick: tick, SteamID64: key.attacker, VictimID: key.target,
			Kind: "snap", Value: encounter.firstAngle - encounter.firstShotAngle, Details: "aim reduction in <=100ms before first shot",
		})
	}
}

// firedInTick reports whether the player fired a recorded shot in tick.
func (c *demoStatsCollector) firedInTick(attacker uint64, round, tick int) bool {
	shots := c.shots[attacker]
	for i := len(shots) - 1; i >= 0 && shots[i].tick >= tick; i-- {
		if shots[i].tick == tick && shots[i].round == round {
			return true
		}
	}
	return false
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
			player := c.player(shot.round, damage.AttackerSteamID64, "")
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
			c.weapon(shot.round, damage.AttackerSteamID64, damage.WeaponName).HitShots++
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
	c.syncRound(analyzer)
	if damage.AttackerSteamID64 == 0 || damage.AttackerSteamID64 == damage.VictimSteamID64 || damage.AttackerSide == damage.VictimSide || !validAimWeapon(damage.WeaponName) {
		return
	}
	stats := c.player(damage.RoundNumber, damage.AttackerSteamID64, "")
	stats.DamageEvents++
	weaponStats := c.weapon(damage.RoundNumber, damage.AttackerSteamID64, damage.WeaponName)
	weaponStats.DamageEvents++
	if damage.HitGroup == events.HitGroupHead {
		stats.HeadHitEvents++
		weaponStats.HeadHitEvents++
	}
	if !c.markHitShot(damage) {
		c.pendingDamage[damage.AttackerSteamID64] = append(c.pendingDamage[damage.AttackerSteamID64], damage)
	}

	c.deferredDamage = append(c.deferredDamage, damage)
}

// resolveDeferredDamage turns the damage events of the finished tick into
// encounter samples.
func (c *demoStatsCollector) resolveDeferredDamage(analyzer *Analyzer) {
	deferred := c.deferredDamage
	c.deferredDamage = nil
	for _, damage := range deferred {
		c.resolveEncounterDamage(analyzer, damage)
	}
}

func (c *demoStatsCollector) resolveEncounterDamage(analyzer *Analyzer, damage *Damage) {
	stats := c.player(damage.RoundNumber, damage.AttackerSteamID64, "")
	round := c.round(damage.RoundNumber)
	key := encounterKey{attacker: damage.AttackerSteamID64, target: damage.VictimSteamID64}
	encounter := c.encounters[key]
	if encounter == nil || encounter.confirmedTick == 0 {
		stats.UnspottedDamageEvents++
		round.evidence = append(round.evidence, DemoEvidence{
			RoundNumber: damage.RoundNumber, Tick: damage.Tick, SteamID64: damage.AttackerSteamID64, VictimID: damage.VictimSteamID64,
			Kind: "damage_without_confirmed_spot", Value: float64(damage.HealthDamage), Details: damage.WeaponName.String(),
		})
		return
	}
	if encounter.reacted {
		return
	}
	encounter.reacted = true
	if encounter.firstShotTick < 0 {
		// The shot may have been given to another target (the one closest to the
		// crosshair), or one bullet hit several players. The victim of a hit is
		// known, so take the shot fired in the tick of the hit.
		a, aok := c.frames[damage.AttackerSteamID64]
		t, tok := c.frames[damage.VictimSteamID64]
		if aok && tok && c.firedInTick(damage.AttackerSteamID64, damage.RoundNumber, damage.Tick) {
			encounter.shotCount++
			c.registerFirstShot(analyzer, key, encounter, damage.RoundNumber, damage.Tick, a, t)
		}
	}
	ttd := c.tickDeltaMS(analyzer, encounter.firstTick, damage.Tick)
	ttdConfirmed := c.tickDeltaMS(analyzer, encounter.confirmedTick, damage.Tick)
	if ttdConfirmed < 0 {
		return
	}
	firstShotMS := float64(-1)
	// -1 means no shot was attributed: 0° would read as a perfect aim.
	firstShotAngle := float64(-1)
	// Fall back to the damage tick when no shot was attributed to this
	// encounter (player_hurt can arrive before weapon_fire in the same tick,
	// or the shot was attributed to another encounter); a hitscan hit implies
	// a shot at the damage tick, keeping reaction <= TTD per encounter.
	reactionMS := ttd
	if encounter.firstShotTick >= 0 {
		firstShotAngle = encounter.firstShotAngle
		firstShotMS = c.tickDeltaMS(analyzer, encounter.confirmedTick, encounter.firstShotTick)
		reactionMS = c.tickDeltaMS(analyzer, encounter.firstTick, encounter.firstShotTick)
		if encounter.shotCount == 1 && damage.Tick-encounter.firstShotTick <= 2 && damage.HitGroup == events.HitGroupHead {
			stats.FirstBulletHeadHits++
		}
	}
	stats.TTDSamples++
	stats.TTDSumMS += ttd
	round.encounters = append(round.encounters, DemoEncounter{
		RoundNumber: damage.RoundNumber, AttackerSteamID64: damage.AttackerSteamID64, VictimSteamID64: damage.VictimSteamID64,
		FirstSpottedTick: encounter.firstTick, ConfirmedTick: encounter.confirmedTick, DamageTick: damage.Tick,
		TTDMS: ttd, TTDConfirmedMS: ttdConfirmed, FirstShotTimeMS: firstShotMS, ReactionTimeMS: reactionMS,
		FirstAngle: encounter.firstAngle, ConfirmedAngle: encounter.confirmedAngle, FirstShotAngle: firstShotAngle,
		DistanceMeters: encounter.distance, WeaponName: damage.WeaponName.String(), Snap: encounter.snap,
	})
	if ttd >= 0 && ttd <= 190 {
		round.evidence = append(round.evidence, DemoEvidence{
			RoundNumber: damage.RoundNumber, Tick: damage.Tick, SteamID64: damage.AttackerSteamID64, VictimID: damage.VictimSteamID64,
			Kind: "fast_ttd", Value: ttd, Details: damage.WeaponName.String(),
		})
	}
}

func addPlayerStats(dst, src *DemoPlayerStats) {
	dst.Shots += src.Shots
	dst.HitShots += src.HitShots
	dst.DamageEvents += src.DamageEvents
	dst.HeadHitEvents += src.HeadHitEvents
	dst.UnspottedDamageEvents += src.UnspottedDamageEvents
	dst.FirstBulletEncounters += src.FirstBulletEncounters
	dst.FirstBulletHeadHits += src.FirstBulletHeadHits
	dst.SnapEvents += src.SnapEvents
	dst.TTDSamples += src.TTDSamples
	dst.TTDSumMS += src.TTDSumMS
	dst.MovingShots += src.MovingShots
	dst.MovingHitShots += src.MovingHitShots
	dst.AirborneShots += src.AirborneShots
	dst.AirborneHitShots += src.AirborneHitShots
	dst.FlashedShots += src.FlashedShots
	dst.FlashedHitShots += src.FlashedHitShots
	dst.ScopedShots += src.ScopedShots
	dst.ScopedHitShots += src.ScopedHitShots
}

// finalize builds the demo result from the rounds the match kept. Rounds that
// were dropped from the match (incomplete, restored, restarted) contribute
// nothing, so shots, damage and encounters cover the same events as the match.
func (c *demoStatsCollector) finalize(match *Match) {
	accepted := make(map[int]bool, len(match.Rounds))
	for _, round := range match.Rounds {
		accepted[round.Number] = true
	}
	result := DemoStats{
		Players: make(map[uint64]*DemoPlayerStats),
		Weapons: make(map[uint64]map[string]*DemoWeaponStats),
	}
	player := func(id uint64) *DemoPlayerStats {
		stats := result.Players[id]
		if stats == nil {
			stats = &DemoPlayerStats{SteamID64: id, Name: c.names[id]}
			result.Players[id] = stats
		}
		return stats
	}
	weapon := func(id uint64, name string) *DemoWeaponStats {
		weapons := result.Weapons[id]
		if weapons == nil {
			weapons = make(map[string]*DemoWeaponStats)
			result.Weapons[id] = weapons
		}
		stats := weapons[name]
		if stats == nil {
			stats = &DemoWeaponStats{SteamID64: id, WeaponName: name}
			weapons[name] = stats
		}
		return stats
	}

	roundNumbers := make([]int, 0, len(c.rounds))
	for number := range c.rounds {
		if accepted[number] {
			roundNumbers = append(roundNumbers, number)
		}
	}
	sort.Ints(roundNumbers)
	for _, number := range roundNumbers {
		round := c.rounds[number]
		for id, stats := range round.players {
			addPlayerStats(player(id), stats)
		}
		for id, weapons := range round.weapons {
			for name, stats := range weapons {
				total := weapon(id, name)
				total.Shots += stats.Shots
				total.HitShots += stats.HitShots
				total.DamageEvents += stats.DamageEvents
				total.HeadHitEvents += stats.HeadHitEvents
			}
		}
		result.Encounters = append(result.Encounters, round.encounters...)
		result.Evidence = append(result.Evidence, round.evidence...)
	}

	rounds := len(match.Rounds)
	for _, matchPlayer := range match.Players() {
		stats := player(matchPlayer.SteamID64)
		stats.Name = matchPlayer.Name
		stats.Rounds = rounds
		stats.Kills = matchPlayer.KillCount()
		stats.Deaths = matchPlayer.DeathCount()
		stats.HeadshotKills = matchPlayer.HeadshotCount()
	}
	for _, kill := range match.Kills {
		if kill.KillerSteamID64 == 0 || kill.IsSuicide() || kill.IsTeamKill() {
			continue
		}
		stats := player(kill.KillerSteamID64)
		if kill.IsThroughSmoke {
			stats.SmokeKills++
		}
		if kill.PenetratedObjects > 0 {
			stats.WallKills++
		}
		if validAimWeapon(kill.WeaponName) {
			weapon(kill.KillerSteamID64, kill.WeaponName.String()).Kills++
		}
	}
	// Keep report and database output stable.
	sort.SliceStable(result.Encounters, func(i, j int) bool {
		a, b := result.Encounters[i], result.Encounters[j]
		if a.RoundNumber != b.RoundNumber {
			return a.RoundNumber < b.RoundNumber
		}
		if a.DamageTick != b.DamageTick {
			return a.DamageTick < b.DamageTick
		}
		return a.AttackerSteamID64 < b.AttackerSteamID64
	})
	c.result = result
}
