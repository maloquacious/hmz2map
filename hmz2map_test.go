// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2map

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/maloquacious/hmz2bio"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
	"github.com/maloquacious/hmz2ter"
)

// The Panama files, used by the tests that need real data. They are
// skipped when the files are missing.
const (
	climateFile = "../var/pandemokh-a48-climate.json"
	riversFile  = "../var/pandemokh-a48-rivers.json"
)

// directions pairs each side with hmz2ele's direction.
var directions = map[Side]hmz2ele.Direction{
	SideN: hmz2ele.North, SideNE: hmz2ele.NorthEast, SideSE: hmz2ele.SouthEast,
	SideS: hmz2ele.South, SideSW: hmz2ele.SouthWest, SideNW: hmz2ele.NorthWest,
}

// TestParity checks that shifting source columns by 1 (and the border by
// 4) keeps every neighbor: the neighbors of a hex in map coordinates are
// the map coordinates of its hmz2ele neighbors.
func TestParity(t *testing.T) {
	g, err := hmz2ele.NewGrid(48, 8868, 21307)
	if err != nil {
		t.Fatal(err)
	}
	for _, border := range []int{0, BorderWidth} {
		f := newFrame(g, border)
		for col := -2; col < g.Columns+2; col++ {
			for row := -2; row < g.Rows+2; row++ {
				mc, mr := f.toMap(col, row)
				for s, d := range directions {
					nc, nr := hmz2ele.Neighbor(col, row, d)
					wc, wr := f.toMap(nc, nr)
					if gc, gr := Neighbor(mc, mr, s); gc != wc || gr != wr {
						t.Fatalf("border %d: source (%d, %d) %s is (%d, %d) = map (%d, %d), but Neighbor(%d, %d) = (%d, %d)",
							border, col, row, s, nc, nr, wc, wr, mc, mr, gc, gr)
					}
				}
			}
		}
	}
}

// TestLayout checks the user's layout directly: (1, 0) is half a hex below
// (0, 0) and (2, 0), which share a row.
func TestLayout(t *testing.T) {
	for _, tc := range []struct {
		col, row int
		s        Side
		wc, wr   int
	}{
		{0, 0, SideSE, 1, 0}, // (1, 0) is below-right of (0, 0)
		{1, 0, SideNE, 2, 0}, // (2, 0) is above-right of (1, 0)
		{2, 0, SideSW, 1, 0},
		{1, 0, SideNW, 0, 0},
		{1, 0, SideSE, 2, 1},
		{2, 1, SideNW, 1, 0},
		{0, 1, SideNE, 1, 0},
	} {
		if c, r := Neighbor(tc.col, tc.row, tc.s); c != tc.wc || r != tc.wr {
			t.Errorf("Neighbor(%d, %d, %s) = (%d, %d), want (%d, %d)", tc.col, tc.row, tc.s, c, r, tc.wc, tc.wr)
		}
	}
	for _, s := range Sides {
		nc, nr := Neighbor(5, 5, s)
		if bc, br := Neighbor(nc, nr, s.Opposite()); bc != 5 || br != 5 {
			t.Errorf("side %s then %s from (5, 5) lands on (%d, %d)", s, s.Opposite(), bc, br)
		}
	}
}

// TestSideCorners checks the side and corner names against hmz2ele's
// corner numbering: side i lies between corners i + 4 and i + 5.
func TestSideCorners(t *testing.T) {
	want := map[Side][2]Corner{
		SideN: {CornerNW, CornerNE}, SideNE: {CornerNE, CornerE}, SideSE: {CornerE, CornerSE},
		SideS: {CornerSE, CornerSW}, SideSW: {CornerSW, CornerW}, SideNW: {CornerW, CornerNW},
	}
	for s, w := range want {
		if a, b := s.Corners(); a != w[0] || b != w[1] {
			t.Errorf("%s.Corners() = %s, %s; want %s, %s", s, a, b, w[0], w[1])
		}
	}
	// hmz2ele's owned sides: north is corners 4 and 5, and so on.
	for side, own := range ownedSides {
		a, b := hmz2ele.EdgeVertices(hmz2ele.EdgeKey{Col: 3, Row: 3, Side: side})
		ca, cb := Sides[own.index].Corners()
		if hmz2ele.CornerOwner(3, 3, cornerIndex(ca)) != a || hmz2ele.CornerOwner(3, 3, cornerIndex(cb)) != b {
			t.Errorf("hmz2ele side %v isn't %s", side, Sides[own.index])
		}
	}
}

// TestSharedCorners checks the rule Validate relies on: the two hexes of an
// edge name its ends in opposite order.
func TestSharedCorners(t *testing.T) {
	for _, col := range []int{4, 5} {
		for _, s := range Sides {
			nc, nr := hmz2ele.Neighbor(col, 7, directions[s])
			a, b := s.Corners()
			oa, ob := s.Opposite().Corners()
			if hmz2ele.CornerOwner(col, 7, cornerIndex(a)) != hmz2ele.CornerOwner(nc, nr, cornerIndex(ob)) ||
				hmz2ele.CornerOwner(col, 7, cornerIndex(b)) != hmz2ele.CornerOwner(nc, nr, cornerIndex(oa)) {
				t.Errorf("column %d side %s: corners %s, %s don't meet the neighbor's %s, %s", col, s, a, b, ob, oa)
			}
		}
	}
}

func TestSizeOf(t *testing.T) {
	for _, tc := range []struct {
		km2  float64
		want RiverSize
	}{
		{50.2, SizeStream}, {249.9, SizeStream}, {250, SizeRiver}, {1999.9, SizeRiver}, {2000, SizeGreatRiver}, {10764.2, SizeGreatRiver},
	} {
		if got := SizeOf(tc.km2); got != tc.want {
			t.Errorf("SizeOf(%g) = %s, want %s", tc.km2, got, tc.want)
		}
	}
}

// TestEnumerations checks the model's values against the pipeline's.
func TestEnumerations(t *testing.T) {
	if !slices.Equal(strs(Surfaces), strs(hmz2bio.Surfaces)) {
		t.Errorf("Surfaces = %v, hmz2bio has %v", Surfaces, hmz2bio.Surfaces)
	}
	if !slices.Equal(strs(Biomes), strs(hmz2bio.Biomes)) {
		t.Errorf("Biomes = %v, hmz2bio has %v", Biomes, hmz2bio.Biomes)
	}
	for _, l := range Landforms {
		if l.IsLand() != hmz2ter.Landform(l).IsLand() {
			t.Errorf("%s: IsLand disagrees with hmz2ter", l)
		}
	}
	for _, f := range []hmz2ter.Flag{hmz2ter.Coast, hmz2ter.River, hmz2ter.Volcano, hmz2ter.InlandSea, hmz2ter.Impassable} {
		if !slices.Contains(Flags, Flag(f)) {
			t.Errorf("flag %s is missing", f)
		}
	}
	if slices.Contains(Flags, Flag(hmz2ter.FlatSurface)) {
		t.Errorf("flat-surface is a pipeline hint, not a map flag")
	}
}

func strs[T ~string](xs []T) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = string(x)
	}
	return out
}

// readFile returns the file's contents, or skips the test if it is missing.
func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("%s: not found", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// encode writes v as the tools write their JSON.
func encode(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestRoundTrip checks that hmz2bio's and hmz2riv's types decode their own
// output completely: re-encoding gives the same bytes.
func TestRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		path string
		v    any
	}{
		{climateFile, &hmz2bio.Document{}},
		{riversFile, &hmz2riv.Rivers{}},
	} {
		b := readFile(t, tc.path)
		if err := json.Unmarshal(b, tc.v); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if got := encode(t, tc.v); !bytes.Equal(got, b) {
			n := 0
			for n < min(len(got), len(b)) && got[n] == b[n] {
				n++
			}
			t.Errorf("%s: re-encoding differs at byte %d: %q vs %q", tc.path, n, got[n:min(n+80, len(got))], b[n:min(n+80, len(b))])
		}
	}
}

func load(t *testing.T) (hmz2ele.Grid, *hmz2bio.Document, *hmz2riv.Rivers) {
	t.Helper()
	var doc hmz2bio.Document
	var rivers hmz2riv.Rivers
	if err := json.Unmarshal(readFile(t, climateFile), &doc); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(readFile(t, riversFile), &rivers); err != nil {
		t.Fatal(err)
	}
	md := doc.Heightmap.Metadata
	g, err := hmz2ele.NewGrid(doc.Grid.ApothemPx, int(md.Width), int(md.Height))
	if err != nil {
		t.Fatal(err)
	}
	return g, &doc, &rivers
}

// TestPanama builds both maps and checks them against the source: every
// source hex is where the coordinate rule puts it, every river edge is
// listed on exactly the two hexes that share it with the flow hmz2riv
// gives, and the map round-trips through its own types.
func TestPanama(t *testing.T) {
	g, doc, rivers := load(t)
	for _, border := range []int{0, BorderWidth} {
		m, rep, err := Build(g, doc, rivers, Options{Border: border, HexKm: 10})
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(m); err != nil {
			t.Fatalf("border %d: %v", border, err)
		}
		wantCols, wantRows := 106+2*border, 222+2*border
		if m.Columns != wantCols || m.Rows != wantRows || len(m.Hexes) != wantCols*wantRows {
			t.Fatalf("border %d: map is %d × %d with %d hexes, want %d × %d", border, m.Columns, m.Rows, len(m.Hexes), wantCols, wantRows)
		}
		if rep.Kinds[KindSource] != len(doc.Hexes) || rep.Kinds[KindFill] != 53 {
			t.Errorf("border %d: kinds %v", border, rep.Kinds)
		}
		for _, s := range doc.Hexes {
			h := m.At(s.Col+border, s.Row+border)
			if string(h.Landform) != string(s.Landform) || string(h.Biome) != string(s.Biome) || string(h.Depth) != string(s.Depth) {
				t.Fatalf("border %d: source (%d, %d) %s is map (%d, %d) %s", border, s.Col, s.Row, s.Landform, h.Col, h.Row, h.Landform)
			}
		}

		// Every river edge, on both of its hexes.
		listed := 0
		for i := range m.Hexes {
			listed += len(m.Hexes[i].Rivers)
		}
		if listed != 2*len(rivers.Edges) {
			t.Errorf("border %d: %d hex sides with rivers, want %d", border, listed, 2*len(rivers.Edges))
		}
		for _, e := range rivers.Edges {
			to := hmz2ele.VertexKey{Col: e.To.Col, Row: e.To.Row, Corner: e.To.Corner}
			own := ownedSides[e.Side]
			nc, nr := hmz2ele.Neighbor(e.Col, e.Row, own.dir)
			for _, sh := range [2]struct {
				col, row int
				side     Side
			}{{e.Col, e.Row, Sides[own.index]}, {nc, nr, Sides[own.index].Opposite()}} {
				h := m.At(sh.col+border, sh.row+border)
				k := slices.IndexFunc(h.Rivers, func(r River) bool { return r.Side == sh.side })
				if k < 0 {
					t.Fatalf("edge %+v: missing on source (%d, %d) side %s", e, sh.col, sh.row, sh.side)
				}
				r := h.Rivers[k]
				if hmz2ele.CornerOwner(sh.col, sh.row, cornerIndex(r.Flow)) != to || r.DrainageKm2 != e.DrainageKm2 {
					t.Fatalf("edge %+v: source (%d, %d) lists %+v", e, sh.col, sh.row, r)
				}
			}
		}

		// The map's types decode the map completely.
		b := encode(t, m)
		var back Map
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(encode(t, &back), b) {
			t.Errorf("border %d: the map doesn't round-trip", border)
		}
	}
}

// TestValidateRejects checks that Validate catches the mistakes the
// emitter must not make.
func TestValidateRejects(t *testing.T) {
	g, doc, rivers := load(t)
	m, _, err := Build(g, doc, rivers, Options{Border: BorderWidth, HexKm: 10})
	if err != nil {
		t.Fatal(err)
	}
	find := func(ok func(h *Hex) bool) *Hex {
		for i := range m.Hexes {
			if ok(&m.Hexes[i]) {
				return &m.Hexes[i]
			}
		}
		t.Fatal("no such hex")
		return nil
	}
	land := find(func(h *Hex) bool { return h.Landform.IsLand() && len(h.Rivers) > 0 })
	border := m.At(0, 0)
	for name, tc := range map[string]struct {
		h      *Hex
		change func(h *Hex)
	}{
		"flat-surface":    {land, func(h *Hex) { h.Flags = append(h.Flags, "flat-surface") }},
		"coast on border": {border, func(h *Hex) { h.Flags = []Flag{FlagCoast} }},
		"shallow border":  {border, func(h *Hex) { h.Depth = DepthShallow }},
		"river flag": {land, func(h *Hex) {
			h.Flags = slices.DeleteFunc(slices.Clone(h.Flags), func(f Flag) bool { return f == FlagRiver })
		}},
		"one-sided river": {land, func(h *Hex) { h.Rivers = append([]River(nil), h.Rivers...); h.Rivers[0].DrainageKm2++ }},
		"wrong size": {land, func(h *Hex) {
			h.Rivers = append([]River(nil), h.Rivers...)
			h.Rivers[0].Size = SizeGreatRiver
			h.Rivers[0].DrainageKm2 = 1
		}},
		"border elevation": {border, func(h *Hex) { h.Elevation = 0 }},
	} {
		saved := *tc.h
		tc.change(tc.h)
		if Validate(m) == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
		*tc.h = saved
	}
	if err := Validate(m); err != nil {
		t.Fatalf("restored map: %v", err)
	}
}
