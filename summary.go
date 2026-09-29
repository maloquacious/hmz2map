// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2map

import "math"

// Summary counts a map's hexes. Surfaces and Biomes count land hexes only,
// and RiverEdges counts each edge once, by size class.
type Summary struct {
	Landforms, Surfaces, Biomes, Depths, Flags, RiverEdges map[string]int
	// LandMin and LandMax are the lowest and highest land elevations.
	LandMin, LandMax int
}

// Summarize counts the map's hexes.
func Summarize(m *Map) Summary {
	s := Summary{
		Landforms: map[string]int{}, Surfaces: map[string]int{}, Biomes: map[string]int{},
		Depths: map[string]int{}, Flags: map[string]int{}, RiverEdges: map[string]int{},
		LandMin: math.MaxInt, LandMax: math.MinInt,
	}
	for i := range m.Hexes {
		h := &m.Hexes[i]
		s.Landforms[string(h.Landform)]++
		if h.Depth != DepthNone {
			s.Depths[string(h.Depth)]++
		}
		for _, f := range h.Flags {
			s.Flags[string(f)]++
		}
		if h.Landform.IsLand() {
			s.Surfaces[string(h.Surface)]++
			s.Biomes[string(h.Biome)]++
			s.LandMin, s.LandMax = min(s.LandMin, h.Elevation), max(s.LandMax, h.Elevation)
		}
		for _, r := range h.Rivers {
			// Count an edge on its north, north-east, or south-east side,
			// or on its only hex.
			if sideIndex(r.Side) < 3 || m.At(Neighbor(h.Col, h.Row, r.Side)) == nil {
				s.RiverEdges[string(r.Size)]++
			}
		}
	}
	return s
}
