// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2map

import (
	"errors"
	"fmt"
	"slices"
)

// maxErrors limits how many problems Validate reports.
const maxErrors = 20

// Validate checks the map against the terrain model's valid combinations
// and the map's own rules, and returns the problems found, or nil.
//
// Border hexes are the one exception to the model: they are deep salt
// water with no flags, even next to land.
func Validate(m *Map) error {
	var errs []error
	report := func(h *Hex, format string, args ...any) bool {
		if len(errs) == maxErrors {
			errs = append(errs, errors.New("more errors not shown"))
			return false
		}
		errs = append(errs, fmt.Errorf("hex (%d, %d) %s: %s", h.Col, h.Row, h.Landform, fmt.Sprintf(format, args...)))
		return true
	}
	if m.Border != 0 && m.Border != BorderWidth {
		return fmt.Errorf("border %d: must be 0 or %d", m.Border, BorderWidth)
	}
	if len(m.Hexes) != m.Columns*m.Rows {
		return fmt.Errorf("%d hexes, want %d × %d", len(m.Hexes), m.Columns, m.Rows)
	}
	for i := range m.Hexes {
		if h := &m.Hexes[i]; h.Row*m.Columns+h.Col != i || !m.onMap(h.Col, h.Row) {
			return fmt.Errorf("hex %d is (%d, %d), want (%d, %d)", i, h.Col, h.Row, i%m.Columns, i/m.Columns)
		}
	}
	for i := range m.Hexes {
		h := &m.Hexes[i]
		for _, msg := range m.check(h) {
			if !report(h, "%s", msg) {
				return errors.Join(errs...)
			}
		}
	}
	return errors.Join(errs...)
}

func (m *Map) onMap(col, row int) bool {
	return col >= 0 && row >= 0 && col < m.Columns && row < m.Rows
}

// IsBorder reports whether (col, row) is in the map's decorative border.
func (m *Map) IsBorder(col, row int) bool {
	b := m.Border
	return col < b || row < b || col >= m.Columns-b || row >= m.Rows-b
}

// neighbors returns the hexes next to h that are on the map and not in the
// border.
func (m *Map) neighbors(h *Hex) []*Hex {
	var out []*Hex
	for _, s := range Sides {
		c, r := Neighbor(h.Col, h.Row, s)
		if n := m.At(c, r); n != nil && !m.IsBorder(c, r) {
			out = append(out, n)
		}
	}
	return out
}

func (m *Map) check(h *Hex) []string {
	var bad []string
	flagsWithin := func(allowed ...Flag) {
		for i, f := range h.Flags {
			if !slices.Contains(allowed, f) {
				bad = append(bad, fmt.Sprintf("flag %q not allowed", f))
			}
			if slices.Contains(h.Flags[:i], f) {
				bad = append(bad, fmt.Sprintf("flag %q twice", f))
			}
		}
	}
	clear := func() {
		if h.Surface != SurfaceClear || h.Biome != BiomeClear {
			bad = append(bad, fmt.Sprintf("surface %q and biome %q, want clear and clear", h.Surface, h.Biome))
		}
	}
	noDepth := func() {
		if h.Depth != DepthNone {
			bad = append(bad, fmt.Sprintf("depth %q on a hex that isn't salt water", h.Depth))
		}
	}
	neighbors := m.neighbors(h)
	nextTo := func(salt bool) bool {
		return slices.ContainsFunc(neighbors, func(n *Hex) bool { return (n.Landform == LandformSaltWater) == salt })
	}
	coastIf := func(want bool) {
		if h.HasFlag(FlagCoast) != want {
			bad = append(bad, fmt.Sprintf("coast flag is %v, want %v", h.HasFlag(FlagCoast), want))
		}
	}

	switch h.Landform {
	case LandformFreshWater:
		clear()
		noDepth()
		flagsWithin()
	case LandformSaltWater:
		clear()
		if !slices.Contains(Depths, h.Depth) {
			bad = append(bad, fmt.Sprintf("depth %q", h.Depth))
		}
		flagsWithin(FlagCoast, FlagInlandSea)
		if h.HasFlag(FlagCoast) && h.Depth != DepthShallow {
			bad = append(bad, fmt.Sprintf("coast water is %q, not shallow", h.Depth))
		}
		if m.IsBorder(h.Col, h.Row) {
			if h.Depth != DepthDeep || len(h.Flags) > 0 {
				bad = append(bad, fmt.Sprintf("border hex is %q with flags %v, want deep with none", h.Depth, h.Flags))
			}
		} else {
			coastIf(nextTo(false))
		}
		want := DeepElevation
		switch {
		case h.HasFlag(FlagInlandSea):
			want = InlandSeaElevation
		case h.Depth == DepthShallow:
			want = ShallowElevation
		case h.Depth == DepthOpen:
			want = OpenElevation
		}
		if h.Elevation != want {
			bad = append(bad, fmt.Sprintf("elevation %d, want %d", h.Elevation, want))
		}
	case LandformCliffs, LandformBadlands:
		clear()
		noDepth()
		flagsWithin(FlagImpassable)
		if !h.HasFlag(FlagImpassable) {
			bad = append(bad, "missing the impassable flag")
		}
	case LandformFlats, LandformPlains, LandformRollingPlains, LandformHills, LandformMountains, LandformPlateaus, LandformVolcanicHighlands:
		noDepth()
		flagsWithin(FlagCoast, FlagRiver, FlagVolcano, FlagImpassable)
		if !slices.Contains(Surfaces, h.Surface) {
			bad = append(bad, fmt.Sprintf("surface %q", h.Surface))
		}
		if !slices.Contains(Biomes, h.Biome) {
			bad = append(bad, fmt.Sprintf("biome %q", h.Biome))
		}
		if (h.Biome == BiomeClear) != (h.Surface == SurfaceGlacialIce) {
			bad = append(bad, fmt.Sprintf("biome %q with surface %q: clear if and only if glacial-ice", h.Biome, h.Surface))
		}
		switch h.Surface {
		case SurfaceMarshes, SurfaceSwamps, SurfaceBogs, SurfaceMangroves, SurfaceSaltFlats:
			if h.Landform != LandformFlats && h.Landform != LandformPlains {
				bad = append(bad, fmt.Sprintf("%s only on flats and plains", h.Surface))
			}
		}
		if h.Surface == SurfaceMangroves && !h.HasFlag(FlagCoast) {
			bad = append(bad, "mangroves without the coast flag")
		}
		coastIf(nextTo(true))
		if h.HasFlag(FlagRiver) != (len(h.Rivers) > 0) {
			bad = append(bad, fmt.Sprintf("river flag is %v with %d rivers", h.HasFlag(FlagRiver), len(h.Rivers)))
		}
	default:
		bad = append(bad, "unknown or unset landform")
	}
	return append(bad, m.checkRivers(h)...)
}

// checkRivers checks the hex's rivers, and that the hex across each one
// lists the same edge.
func (m *Map) checkRivers(h *Hex) []string {
	var bad []string
	for i, r := range h.Rivers {
		if !slices.Contains(Sides[:], r.Side) {
			bad = append(bad, fmt.Sprintf("river on side %q", r.Side))
			continue
		}
		if i > 0 && sideIndex(h.Rivers[i-1].Side) >= sideIndex(r.Side) {
			bad = append(bad, "rivers not in side order, or two on one side")
		}
		a, b := r.Side.Corners()
		if r.Flow != a && r.Flow != b {
			bad = append(bad, fmt.Sprintf("river on side %s flows to corner %q, not %s or %s", r.Side, r.Flow, a, b))
		}
		if r.Size != SizeOf(r.DrainageKm2) {
			bad = append(bad, fmt.Sprintf("river on side %s is %q, want %q for %g km²", r.Side, r.Size, SizeOf(r.DrainageKm2), r.DrainageKm2))
		}
		n := m.At(Neighbor(h.Col, h.Row, r.Side))
		if n == nil {
			continue
		}
		// Clockwise around one hex is counterclockwise around the other, so
		// this hex's first corner of the side is the neighbor's second.
		o := r.Side.Opposite()
		oa, ob := o.Corners()
		want := oa
		if r.Flow == a {
			want = ob
		}
		k := slices.IndexFunc(n.Rivers, func(nr River) bool { return nr.Side == o })
		if k < 0 {
			bad = append(bad, fmt.Sprintf("river on side %s isn't listed by hex (%d, %d)", r.Side, n.Col, n.Row))
			continue
		}
		if nr := n.Rivers[k]; nr.Flow != want || nr.DrainageKm2 != r.DrainageKm2 || nr.Size != r.Size {
			bad = append(bad, fmt.Sprintf("river on side %s is %v, but hex (%d, %d) lists it as %v", r.Side, r, n.Col, n.Row, nr))
		}
	}
	return bad
}
