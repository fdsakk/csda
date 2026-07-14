package vis

import (
	"math"
	"sort"

	"github.com/golang/geo/r3"
)

type aabb struct {
	min, max r3.Vector
}

func (b *aabb) expand(t Triangle) {
	b.min.X = math.Min(b.min.X, math.Min(t.P1.X, math.Min(t.P2.X, t.P3.X)))
	b.min.Y = math.Min(b.min.Y, math.Min(t.P1.Y, math.Min(t.P2.Y, t.P3.Y)))
	b.min.Z = math.Min(b.min.Z, math.Min(t.P1.Z, math.Min(t.P2.Z, t.P3.Z)))
	b.max.X = math.Max(b.max.X, math.Max(t.P1.X, math.Max(t.P2.X, t.P3.X)))
	b.max.Y = math.Max(b.max.Y, math.Max(t.P1.Y, math.Max(t.P2.Y, t.P3.Y)))
	b.max.Z = math.Max(b.max.Z, math.Max(t.P1.Z, math.Max(t.P2.Z, t.P3.Z)))
}

func emptyAABB() aabb {
	inf := math.Inf(1)
	return aabb{min: r3.Vector{X: inf, Y: inf, Z: inf}, max: r3.Vector{X: -inf, Y: -inf, Z: -inf}}
}

// intersectsSegment reports whether the segment origin + t*dir, t in [0, tMax],
// passes through the box (slab test). Plain comparisons instead of
// math.Min/Max: those guard NaN semantics we don't need and dominated the CPU
// profile as non-inlined calls.
func (b *aabb) intersectsSegment(origin, invDir r3.Vector, tMax float64) bool {
	t1 := (b.min.X - origin.X) * invDir.X
	t2 := (b.max.X - origin.X) * invDir.X
	if t1 > t2 {
		t1, t2 = t2, t1
	}
	tmin, tmax := t1, t2

	t1 = (b.min.Y - origin.Y) * invDir.Y
	t2 = (b.max.Y - origin.Y) * invDir.Y
	if t1 > t2 {
		t1, t2 = t2, t1
	}
	if t1 > tmin {
		tmin = t1
	}
	if t2 < tmax {
		tmax = t2
	}

	t1 = (b.min.Z - origin.Z) * invDir.Z
	t2 = (b.max.Z - origin.Z) * invDir.Z
	if t1 > t2 {
		t1, t2 = t2, t1
	}
	if t1 > tmin {
		tmin = t1
	}
	if t2 < tmax {
		tmax = t2
	}

	return tmax >= tmin && tmax >= 0 && tmin <= tMax
}

type bvhNode struct {
	bounds aabb
	// Leaf when count > 0: triangles[first:first+count].
	// Internal when count == 0: children at left and left+1... left holds the
	// index of the first child node.
	first, count int
	left         int
}

// BVH is a bounding volume hierarchy over map triangles supporting fast
// segment-occlusion queries.
type BVH struct {
	triangles []Triangle
	centroids []r3.Vector
	nodes     []bvhNode
}

const bvhLeafSize = 4

// NewBVH builds a BVH over the given triangles.
func NewBVH(triangles []Triangle) *BVH {
	b := &BVH{triangles: triangles}
	if len(triangles) == 0 {
		return b
	}
	b.centroids = make([]r3.Vector, len(triangles))
	for i, t := range triangles {
		b.centroids[i] = r3.Vector{
			X: (t.P1.X + t.P2.X + t.P3.X) / 3,
			Y: (t.P1.Y + t.P2.Y + t.P3.Y) / 3,
			Z: (t.P1.Z + t.P2.Z + t.P3.Z) / 3,
		}
	}
	b.nodes = make([]bvhNode, 0, 2*len(triangles)/bvhLeafSize+2)
	b.nodes = append(b.nodes, bvhNode{})
	b.build(0, 0, len(triangles))
	b.centroids = nil
	return b
}

const sahBinCount = 16

func axisValue(v r3.Vector, axis int) float64 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}

func surfaceArea(b aabb) float64 {
	d := b.max.Sub(b.min)
	if d.X < 0 || d.Y < 0 || d.Z < 0 {
		return 0
	}
	return 2 * (d.X*d.Y + d.Y*d.Z + d.Z*d.X)
}

func (b *BVH) build(nodeIndex, first, count int) {
	bounds := emptyAABB()
	centroidBounds := emptyAABB()
	for i := first; i < first+count; i++ {
		bounds.expand(b.triangles[i])
		c := b.centroids[i]
		centroidBounds.min.X = math.Min(centroidBounds.min.X, c.X)
		centroidBounds.min.Y = math.Min(centroidBounds.min.Y, c.Y)
		centroidBounds.min.Z = math.Min(centroidBounds.min.Z, c.Z)
		centroidBounds.max.X = math.Max(centroidBounds.max.X, c.X)
		centroidBounds.max.Y = math.Max(centroidBounds.max.Y, c.Y)
		centroidBounds.max.Z = math.Max(centroidBounds.max.Z, c.Z)
	}
	b.nodes[nodeIndex].bounds = bounds
	if count <= bvhLeafSize {
		b.nodes[nodeIndex].first = first
		b.nodes[nodeIndex].count = count
		return
	}

	// Binned SAH split on the longest centroid axis; queries visit ~2x fewer
	// nodes than with a median split.
	size := centroidBounds.max.Sub(centroidBounds.min)
	axis := 0
	if size.Y > size.X && size.Y >= size.Z {
		axis = 1
	} else if size.Z > size.X && size.Z > size.Y {
		axis = 2
	}
	axisMin := axisValue(centroidBounds.min, axis)
	axisExtent := axisValue(size, axis)

	mid := first + count/2
	if axisExtent > 0 {
		binOf := func(i int) int {
			bin := int(float64(sahBinCount) * (axisValue(b.centroids[i], axis) - axisMin) / axisExtent)
			return min(bin, sahBinCount-1)
		}
		var binBounds [sahBinCount]aabb
		var binCounts [sahBinCount]int
		for i := range sahBinCount {
			binBounds[i] = emptyAABB()
		}
		for i := first; i < first+count; i++ {
			bin := binOf(i)
			binCounts[bin]++
			binBounds[bin].expand(b.triangles[i])
		}
		// Sweep the 15 candidate splits; cost = area-weighted triangle counts.
		var leftArea, rightArea [sahBinCount]float64
		var leftCount, rightCount [sahBinCount]int
		acc := emptyAABB()
		n := 0
		for i := range sahBinCount - 1 {
			if binCounts[i] > 0 {
				acc.min.X = math.Min(acc.min.X, binBounds[i].min.X)
				acc.min.Y = math.Min(acc.min.Y, binBounds[i].min.Y)
				acc.min.Z = math.Min(acc.min.Z, binBounds[i].min.Z)
				acc.max.X = math.Max(acc.max.X, binBounds[i].max.X)
				acc.max.Y = math.Max(acc.max.Y, binBounds[i].max.Y)
				acc.max.Z = math.Max(acc.max.Z, binBounds[i].max.Z)
			}
			n += binCounts[i]
			leftArea[i] = surfaceArea(acc)
			leftCount[i] = n
		}
		acc = emptyAABB()
		n = 0
		for i := sahBinCount - 1; i > 0; i-- {
			if binCounts[i] > 0 {
				acc.min.X = math.Min(acc.min.X, binBounds[i].min.X)
				acc.min.Y = math.Min(acc.min.Y, binBounds[i].min.Y)
				acc.min.Z = math.Min(acc.min.Z, binBounds[i].min.Z)
				acc.max.X = math.Max(acc.max.X, binBounds[i].max.X)
				acc.max.Y = math.Max(acc.max.Y, binBounds[i].max.Y)
				acc.max.Z = math.Max(acc.max.Z, binBounds[i].max.Z)
			}
			n += binCounts[i]
			rightArea[i-1] = surfaceArea(acc)
			rightCount[i-1] = n
		}
		bestSplit, bestCost := -1, math.Inf(1)
		for i := range sahBinCount - 1 {
			if leftCount[i] == 0 || rightCount[i] == 0 {
				continue
			}
			cost := leftArea[i]*float64(leftCount[i]) + rightArea[i]*float64(rightCount[i])
			if cost < bestCost {
				bestCost = cost
				bestSplit = i
			}
		}
		if bestSplit >= 0 {
			// Partition triangles in place around the chosen bin boundary.
			left, right := first, first+count-1
			for left <= right {
				if binOf(left) <= bestSplit {
					left++
				} else {
					b.triangles[left], b.triangles[right] = b.triangles[right], b.triangles[left]
					b.centroids[left], b.centroids[right] = b.centroids[right], b.centroids[left]
					right--
				}
			}
			mid = left
		}
	}
	if mid <= first || mid >= first+count {
		// Degenerate split (all centroids identical); fall back to the median.
		slice := triSorter{b: b, first: first, count: count, axis: axis}
		sort.Sort(slice)
		mid = first + count/2
	}

	left := len(b.nodes)
	b.nodes = append(b.nodes, bvhNode{}, bvhNode{})
	b.nodes[nodeIndex].left = left
	b.nodes[nodeIndex].count = 0
	b.build(left, first, mid-first)
	b.build(left+1, mid, first+count-mid)
}

type triSorter struct {
	b            *BVH
	first, count int
	axis         int
}

func (s triSorter) Len() int { return s.count }

func (s triSorter) Less(i, j int) bool {
	a := s.b.centroids[s.first+i]
	b := s.b.centroids[s.first+j]
	switch s.axis {
	case 0:
		return a.X < b.X
	case 1:
		return a.Y < b.Y
	default:
		return a.Z < b.Z
	}
}

func (s triSorter) Swap(i, j int) {
	f := s.first
	s.b.triangles[f+i], s.b.triangles[f+j] = s.b.triangles[f+j], s.b.triangles[f+i]
	s.b.centroids[f+i], s.b.centroids[f+j] = s.b.centroids[f+j], s.b.centroids[f+i]
}

const rayEpsilon = 1e-7

// segmentHitsTriangle runs Möller–Trumbore for the segment origin + t*dir,
// t in [0, tMax].
func segmentHitsTriangle(origin, dir r3.Vector, tMax float64, t Triangle) bool {
	edge1 := t.P2.Sub(t.P1)
	edge2 := t.P3.Sub(t.P1)
	h := dir.Cross(edge2)
	a := edge1.Dot(h)
	if a > -rayEpsilon && a < rayEpsilon {
		return false
	}
	f := 1 / a
	s := origin.Sub(t.P1)
	u := f * s.Dot(h)
	if u < 0 || u > 1 {
		return false
	}
	q := s.Cross(edge1)
	v := f * dir.Dot(q)
	if v < 0 || u+v > 1 {
		return false
	}
	tHit := f * edge2.Dot(q)
	return tHit > rayEpsilon && tHit < tMax
}

// SegmentBlocked reports whether the segment from a to b intersects any map
// triangle.
func (b *BVH) SegmentBlocked(from, to r3.Vector) bool {
	if len(b.nodes) == 0 {
		return false
	}
	dir := to.Sub(from)
	invDir := r3.Vector{X: safeInv(dir.X), Y: safeInv(dir.Y), Z: safeInv(dir.Z)}
	const tMax = 1.0

	// Fixed-size stack: the median-split tree is balanced, so depth stays well
	// under 64 even for multi-million-triangle maps. Avoids a heap allocation
	// per query, which dominated analysis time via GC pressure.
	var stack [128]int
	top := 0
	stack[top] = 0
	top++
	for top > 0 {
		top--
		nodeIndex := stack[top]
		node := &b.nodes[nodeIndex]
		if !node.bounds.intersectsSegment(from, invDir, tMax) {
			continue
		}
		if node.count > 0 {
			for i := node.first; i < node.first+node.count; i++ {
				if segmentHitsTriangle(from, dir, tMax, b.triangles[i]) {
					return true
				}
			}
			continue
		}
		if top+2 <= len(stack) {
			stack[top] = node.left
			stack[top+1] = node.left + 1
			top += 2
		}
	}
	return false
}

func safeInv(v float64) float64 {
	if v == 0 {
		return math.Inf(1)
	}
	return 1 / v
}
