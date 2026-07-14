// Package vis provides geometric visibility checks against CS2 map collision
// geometry. Triangle data comes from awpy-style .tri files (a raw sequence of
// nine little-endian float32 values per triangle), loaded either from
// <dir>/<map>.tri or from a <dir>/tris.zip archive member.
package vis

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/golang/geo/r3"
)

// Triangle is a single collision triangle in map coordinates.
type Triangle struct {
	P1, P2, P3 r3.Vector
}

const triangleByteSize = 36 // 9 float32

// ReadTriangles parses awpy .tri binary data: consecutive triangles of nine
// little-endian float32 values (p1.x p1.y p1.z p2.x ... p3.z).
func ReadTriangles(r io.Reader) ([]Triangle, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data)%triangleByteSize != 0 {
		return nil, fmt.Errorf("tri data size %d is not a multiple of %d", len(data), triangleByteSize)
	}
	triangles := make([]Triangle, 0, len(data)/triangleByteSize)
	for offset := 0; offset+triangleByteSize <= len(data); offset += triangleByteSize {
		var values [9]float64
		for i := range 9 {
			bits := binary.LittleEndian.Uint32(data[offset+i*4:])
			value := float64(math.Float32frombits(bits))
			if math.IsNaN(value) || math.IsInf(value, 0) {
				value = 0
			}
			values[i] = value
		}
		triangles = append(triangles, Triangle{
			P1: r3.Vector{X: values[0], Y: values[1], Z: values[2]},
			P2: r3.Vector{X: values[3], Y: values[4], Z: values[5]},
			P3: r3.Vector{X: values[6], Y: values[7], Z: values[8]},
		})
	}
	return triangles, nil
}

// LoadMapTriangles loads triangles for mapName from dir, trying
// <dir>/<mapName>.tri first and falling back to the <mapName>.tri member of
// <dir>/tris.zip.
func LoadMapTriangles(dir, mapName string) ([]Triangle, error) {
	triPath := filepath.Join(dir, mapName+".tri")
	if file, err := os.Open(triPath); err == nil {
		defer file.Close()
		return ReadTriangles(file)
	}
	zipPath := filepath.Join(dir, "tris.zip")
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("no %s and no readable %s: %w", triPath, zipPath, err)
	}
	defer archive.Close()
	for _, member := range archive.File {
		if member.Name != mapName+".tri" {
			continue
		}
		reader, err := member.Open()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return ReadTriangles(reader)
	}
	return nil, fmt.Errorf("map %q not found in %s", mapName, zipPath)
}
