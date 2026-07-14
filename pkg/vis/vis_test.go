package vis

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
)

// quad returns two triangles forming an axis-aligned rectangle in the X=x
// plane spanning the given Y/Z ranges.
func quadAtX(x, y1, y2, z1, z2 float64) []Triangle {
	a := r3.Vector{X: x, Y: y1, Z: z1}
	b := r3.Vector{X: x, Y: y2, Z: z1}
	c := r3.Vector{X: x, Y: y2, Z: z2}
	d := r3.Vector{X: x, Y: y1, Z: z2}
	return []Triangle{{P1: a, P2: b, P3: c}, {P1: a, P2: c, P3: d}}
}

func TestSegmentBlockedByWall(t *testing.T) {
	wall := quadAtX(0, -100, 100, -100, 100)
	bvh := NewBVH(wall)

	if !bvh.SegmentBlocked(r3.Vector{X: -50}, r3.Vector{X: 50}) {
		t.Fatal("segment through wall should be blocked")
	}
	if bvh.SegmentBlocked(r3.Vector{X: -50, Y: 200}, r3.Vector{X: 50, Y: 200}) {
		t.Fatal("segment passing beside wall should be clear")
	}
	if bvh.SegmentBlocked(r3.Vector{X: -50}, r3.Vector{X: -10}) {
		t.Fatal("segment stopping before wall should be clear")
	}
	if bvh.SegmentBlocked(r3.Vector{X: 10}, r3.Vector{X: 50}) {
		t.Fatal("segment starting past wall should be clear")
	}
}

func TestSegmentBlockedManyTriangles(t *testing.T) {
	var triangles []Triangle
	// A row of separated wall slabs along Y; gaps between them are clear.
	for i := range 500 {
		y := float64(i) * 300
		triangles = append(triangles, quadAtX(0, y, y+100, -100, 100)...)
	}
	bvh := NewBVH(triangles)
	for i := range 500 {
		y := float64(i) * 300
		if !bvh.SegmentBlocked(r3.Vector{X: -50, Y: y + 50}, r3.Vector{X: 50, Y: y + 50}) {
			t.Fatalf("segment through slab %d should be blocked", i)
		}
		if bvh.SegmentBlocked(r3.Vector{X: -50, Y: y + 200}, r3.Vector{X: 50, Y: y + 200}) {
			t.Fatalf("segment through gap after slab %d should be clear", i)
		}
	}
}

func TestSphereBlocksSegment(t *testing.T) {
	s := Sphere{Center: r3.Vector{X: 0}, Radius: 50}
	if !s.blocksSegment(r3.Vector{X: -100}, r3.Vector{X: 100}) {
		t.Fatal("segment through sphere should be blocked")
	}
	if s.blocksSegment(r3.Vector{X: -100, Y: 80}, r3.Vector{X: 100, Y: 80}) {
		t.Fatal("segment missing sphere should be clear")
	}
	if s.blocksSegment(r3.Vector{X: 60}, r3.Vector{X: 100}) {
		t.Fatal("segment outside sphere should be clear")
	}
}

func TestInFOV(t *testing.T) {
	eye := r3.Vector{}
	if !InFOV(eye, 0, 0, r3.Vector{X: 100, Y: 10}) {
		t.Fatal("target ahead should be in FOV")
	}
	if InFOV(eye, 0, 0, r3.Vector{X: -100}) {
		t.Fatal("target behind should be out of FOV")
	}
	if InFOV(eye, 0, 0, r3.Vector{X: 10, Z: 100}) {
		t.Fatal("target straight above should be out of FOV")
	}
	// Source pitch: positive looks down, so a viewer at pitch 30 sees below.
	if !InFOV(eye, 0, 30, r3.Vector{X: 100, Z: -58}) {
		t.Fatal("target below should be in FOV when looking down")
	}
	// Yaw wrap-around: looking at 175°, target at -175° is 10° away.
	if !InFOV(eye, 175, 0, r3.Vector{X: -100, Y: -9}) {
		t.Fatal("target across the yaw wrap should be in FOV")
	}
}

func TestLoadMirage(t *testing.T) {
	engine, err := LoadEngine("../../tris", "de_mirage")
	if err != nil {
		t.Skipf("de_mirage triangles unavailable: %v", err)
	}
	triangles := engine.bvh.triangles
	if len(triangles) < 10000 {
		t.Fatalf("suspiciously few triangles: %d", len(triangles))
	}
	minZ, maxZ := math.Inf(1), math.Inf(-1)
	var sum r3.Vector
	for _, tri := range triangles {
		minZ = math.Min(minZ, tri.P1.Z)
		maxZ = math.Max(maxZ, tri.P1.Z)
		sum = sum.Add(tri.P1)
	}
	center := sum.Mul(1 / float64(len(triangles)))

	// A vertical segment spanning the whole map height must cross geometry.
	top := r3.Vector{X: center.X, Y: center.Y, Z: maxZ + 100}
	bottom := r3.Vector{X: center.X, Y: center.Y, Z: minZ - 100}
	if engine.LineOfSight(top, bottom, nil) {
		t.Fatal("segment through entire map should be blocked")
	}
	// A short segment far above the map is clear.
	if !engine.LineOfSight(top, top.Add(r3.Vector{X: 50}), nil) {
		t.Fatal("segment above map should be clear")
	}
}
