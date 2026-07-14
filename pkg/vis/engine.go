package vis

import (
	"math"
	"sync"

	"github.com/golang/geo/r3"
)

// Sphere approximates a dynamic occluder (an active smoke cloud).
type Sphere struct {
	Center r3.Vector
	Radius float64
}

// blocksSegment reports whether the segment from a to b passes through the
// sphere.
func (s Sphere) blocksSegment(from, to r3.Vector) bool {
	d := to.Sub(from)
	f := from.Sub(s.Center)
	a := d.Dot(d)
	if a == 0 {
		return f.Norm() <= s.Radius
	}
	t := -f.Dot(d) / a
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	closest := from.Add(d.Mul(t))
	return closest.Sub(s.Center).Norm() <= s.Radius
}

// Engine answers visibility queries for one map.
type Engine struct {
	MapName string
	bvh     *BVH
}

// NewEngine builds a visibility engine from map triangles.
func NewEngine(mapName string, triangles []Triangle) *Engine {
	return &Engine{MapName: mapName, bvh: NewBVH(triangles)}
}

// Half-angles of the CS2 first-person frustum at 16:9 (90° horizontal at 4:3).
const (
	horizontalHalfFOVDegrees = 53.14
	verticalHalfFOVDegrees   = 36.87
)

// InFOV reports whether target lies inside the viewer frustum for a viewer at
// eye with the given view angles (degrees; Source convention, pitch positive
// looking down).
func InFOV(eye r3.Vector, yawDegrees, pitchDegrees float64, target r3.Vector) bool {
	delta := target.Sub(eye)
	yawTo := math.Atan2(delta.Y, delta.X) * 180 / math.Pi
	pitchTo := -math.Atan2(delta.Z, math.Hypot(delta.X, delta.Y)) * 180 / math.Pi
	dy := math.Abs(normalizeDegrees(yawTo - yawDegrees))
	dp := math.Abs(normalizeDegrees(pitchTo - pitchDegrees))
	return dy <= horizontalHalfFOVDegrees && dp <= verticalHalfFOVDegrees
}

func normalizeDegrees(angle float64) float64 {
	for angle > 180 {
		angle -= 360
	}
	for angle < -180 {
		angle += 360
	}
	return angle
}

// LineOfSight reports whether the segment from a to b is clear of map geometry
// and of every occluder sphere.
func (e *Engine) LineOfSight(from, to r3.Vector, occluders []Sphere) bool {
	for _, s := range occluders {
		if s.blocksSegment(from, to) {
			return false
		}
	}
	return !e.bvh.SegmentBlocked(from, to)
}

// engineCache shares loaded maps between concurrently analyzed demos.
var engineCache = struct {
	sync.Mutex
	engines map[string]*Engine
	errors  map[string]error
}{engines: map[string]*Engine{}, errors: map[string]error{}}

// LoadEngine returns a cached (or freshly built) engine for mapName using
// triangle data from dir. Failures are cached too, so a missing map is only
// probed once.
func LoadEngine(dir, mapName string) (*Engine, error) {
	key := dir + "\x00" + mapName
	engineCache.Lock()
	defer engineCache.Unlock()
	if engine, ok := engineCache.engines[key]; ok {
		return engine, nil
	}
	if err, ok := engineCache.errors[key]; ok {
		return nil, err
	}
	triangles, err := LoadMapTriangles(dir, mapName)
	if err != nil {
		engineCache.errors[key] = err
		return nil, err
	}
	engine := NewEngine(mapName, triangles)
	engineCache.engines[key] = engine
	return engine, nil
}
