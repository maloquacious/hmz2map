// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2map

import (
	"fmt"
	"slices"

	"github.com/maloquacious/hmz2bio"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
	"github.com/maloquacious/hmz2ter"
)

// BorderWidth is the width, in hexes, of the border that the -border flag
// adds. It is even, so the border keeps every column's parity.
const BorderWidth = 4

// Salt-water elevations, in meters, by depth band. Inland seas are at sea
// level, and border hexes are deep.
const (
	ShallowElevation   = -5
	OpenElevation      = -50
	DeepElevation      = -500
	InlandSeaElevation = 0
)

// Options are the settings of a map.
type Options struct {
	// Border is the border's width in hexes: 0 or BorderWidth.
	Border int
	HexKm  float64
	// ClimateFile and RiversFile are the input files' base names.
	ClimateFile, RiversFile string
}

// Kind says where a hex on the map came from.
type Kind int

const (
	// KindSource is a hex that the source grid has.
	KindSource Kind = iota
	// KindFill is a hex inside the border that the source grid lacks.
	KindFill
	// KindBorder is a hex in the decorative border.
	KindBorder
)

// Report counts what Build did beyond copying hexes.
type Report struct {
	// Kinds counts the map's hexes by kind.
	Kinds [3]int
	// CliffsFilled counts the cliffs and badlands with no median, and
	// CliffPasses the passes needed to give them an elevation.
	CliffsFilled, CliffPasses int
	// DepthChanges counts the source hexes that would get a different depth
	// band if their distance were recomputed over the map.
	DepthChanges int
	// EdgesTouching counts the river edges listed on fill and border hexes,
	// by kind; EdgesOffMap counts the edges with a hex off the map.
	EdgesTouching [3]int
	EdgesOffMap   int
}

// frame converts between source and map coordinates.
type frame struct {
	g             hmz2ele.Grid
	border        int
	columns, rows int
}

func newFrame(g hmz2ele.Grid, border int) frame {
	return frame{g: g, border: border, columns: g.Columns + 1 + 2*border, rows: g.Rows + 2*border}
}

// toMap returns the map coordinates of a source hex.
func (f frame) toMap(col, row int) (int, int) {
	return col + 1 + f.border, row + f.border
}

// toSource returns the source coordinates of a map hex.
func (f frame) toSource(col, row int) (int, int) {
	return col - 1 - f.border, row - f.border
}

func (f frame) onMap(col, row int) bool {
	return col >= 0 && row >= 0 && col < f.columns && row < f.rows
}

func (f frame) kind(col, row int) Kind {
	b := f.border
	if col < b || row < b || col >= f.columns-b || row >= f.rows-b {
		return KindBorder
	}
	if f.g.Contains(f.toSource(col, row)) {
		return KindSource
	}
	return KindFill
}

// Build emits the map of a climate document and its rivers. It doesn't
// validate the result; see Validate.
func Build(g hmz2ele.Grid, doc *hmz2bio.Document, rivers *hmz2riv.Rivers, opt Options) (*Map, Report, error) {
	var rep Report
	if opt.Border != 0 && opt.Border != BorderWidth {
		return nil, rep, fmt.Errorf("border %d: must be 0 or %d", opt.Border, BorderWidth)
	}
	f := newFrame(g, opt.Border)

	// Index the source hexes by slot.
	src := make([]*hmz2bio.HexJSON, g.Columns*g.Rows)
	for i := range doc.Hexes {
		h := &doc.Hexes[i]
		if !g.Contains(h.Col, h.Row) {
			return nil, rep, fmt.Errorf("source hex (%d, %d) is outside the grid", h.Col, h.Row)
		}
		s := h.Row*g.Columns + h.Col
		if src[s] != nil {
			return nil, rep, fmt.Errorf("source hex (%d, %d) is listed twice", h.Col, h.Row)
		}
		src[s] = h
	}
	source := func(col, row int) *hmz2bio.HexJSON {
		if !g.Contains(col, row) {
			return nil
		}
		return src[row*g.Columns+col]
	}

	m := &Map{
		SchemaVersion:  SchemaVersion,
		Hmz2mapVersion: Version().String(),
		Layout:         Layout,
		Columns:        f.columns,
		Rows:           f.rows,
		Border:         opt.Border,
		HexKm:          opt.HexKm,
		Sources: []Source{
			{FileName: opt.ClimateFile, Tool: "hmz2bio", Version: doc.Hmz2bioVersion},
			{FileName: doc.TerrainFile.FileName, Tool: "hmz2ter", Version: doc.Hmz2terVersion},
			{FileName: opt.RiversFile, Tool: "hmz2riv", Version: rivers.Hmz2rivVersion},
		},
		Climate: Placement{TopLatDeg: doc.ClimateRules.TopLat, BottomLatDeg: doc.ClimateRules.BottomLat},
		Hexes:   make([]Hex, f.columns*f.rows),
	}
	kinds := make([]Kind, len(m.Hexes))
	for row := range f.rows {
		for col := range f.columns {
			i := row*f.columns + col
			h := &m.Hexes[i]
			h.Col, h.Row = col, row
			kinds[i] = f.kind(col, row)
			rep.Kinds[kinds[i]]++
			switch kinds[i] {
			case KindSource:
				s := source(f.toSource(col, row))
				if s == nil {
					return nil, rep, fmt.Errorf("source hex (%d, %d) is missing", col-1-f.border, row-f.border)
				}
				h.Landform = Landform(s.Landform)
				h.Surface = Surface(s.Surface)
				h.Biome = Biome(s.Biome)
				h.Depth = Depth(s.Depth)
				for _, fl := range s.Flags {
					if fl != hmz2ter.FlatSurface {
						h.Flags = append(h.Flags, Flag(fl))
					}
				}
			case KindFill, KindBorder:
				h.Landform, h.Surface, h.Biome = LandformSaltWater, SurfaceClear, BiomeClear
			}
		}
	}

	// Distance through salt water from the nearest hex that isn't salt
	// water, over source and fill hexes.
	dist := seaDistances(m, kinds)
	for i := range m.Hexes {
		h := &m.Hexes[i]
		switch kinds[i] {
		case KindSource:
			if h.Landform == LandformSaltWater {
				d := DepthDeep
				if dist[i] >= 0 {
					d = Depth(doc.Rules.Depth(dist[i]))
				}
				if d != h.Depth {
					rep.DepthChanges++
				}
			}
		case KindFill:
			h.Depth = DepthDeep
			if dist[i] >= 0 {
				h.Depth = Depth(doc.Rules.Depth(dist[i]))
			}
			if dist[i] == 1 {
				h.Flags = []Flag{FlagCoast}
			}
		case KindBorder:
			h.Depth = DepthDeep
		}
	}

	if err := setElevations(m, f, kinds, source, &rep); err != nil {
		return nil, rep, err
	}
	if err := addRivers(m, f, kinds, rivers.Edges, &rep); err != nil {
		return nil, rep, err
	}
	return m, rep, nil
}

// seaDistances returns, for each salt-water hex that isn't a border hex, the
// number of steps through salt water from the nearest source or fill hex
// that isn't salt water, or -1 if there is none. Other hexes get 0.
func seaDistances(m *Map, kinds []Kind) []int {
	dist := make([]int, len(m.Hexes))
	salt := func(i int) bool { return m.Hexes[i].Landform == LandformSaltWater }
	var queue []int
	for i := range m.Hexes {
		switch {
		case kinds[i] == KindBorder:
		case salt(i):
			dist[i] = -1
		default:
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		h := &m.Hexes[i]
		for _, s := range Sides {
			n := m.At(Neighbor(h.Col, h.Row, s))
			if n == nil {
				continue
			}
			j := n.Row*m.Columns + n.Col
			if kinds[j] != KindBorder && salt(j) && dist[j] < 0 {
				dist[j] = dist[i] + 1
				queue = append(queue, j)
			}
		}
	}
	return dist
}

// setElevations gives every hex its elevation.
func setElevations(m *Map, f frame, kinds []Kind, source func(col, row int) *hmz2bio.HexJSON, rep *Report) error {
	// Source elevations: the median, where there is one.
	elev := make(map[[2]int]int)
	isCut := func(l hmz2ter.Landform) bool { return l == hmz2ter.Cliffs || l == hmz2ter.Badlands }
	var missing [][2]int
	for i := range m.Hexes {
		if kinds[i] != KindSource {
			continue
		}
		sc, sr := f.toSource(m.Hexes[i].Col, m.Hexes[i].Row)
		s := source(sc, sr)
		switch {
		case s.Elevation != nil:
			elev[[2]int{sc, sr}] = int(s.Elevation.Median)
		case isCut(s.Landform):
			missing = append(missing, [2]int{sc, sr})
		}
	}
	rep.CliffsFilled = len(missing)

	// Cliffs and badlands with no median take the highest elevation among
	// their land, cliff, and badland neighbors that had one before the pass.
	for len(missing) > 0 {
		rep.CliffPasses++
		found := map[[2]int]int{}
		var still [][2]int
		for _, p := range missing {
			best, ok := 0, false
			for _, d := range hmz2ele.Directions {
				nc, nr := hmz2ele.Neighbor(p[0], p[1], d)
				n := source(nc, nr)
				if n == nil || !(n.Landform.IsLand() || isCut(n.Landform)) {
					continue
				}
				if e, has := elev[[2]int{nc, nr}]; has && (!ok || e > best) {
					best, ok = e, true
				}
			}
			if ok {
				found[p] = best
			} else {
				still = append(still, p)
			}
		}
		if len(found) == 0 {
			return fmt.Errorf("%d cliff or badland hexes have no elevation and no neighbor with one, such as source (%d, %d)",
				len(still), still[0][0], still[0][1])
		}
		for p, e := range found {
			elev[p] = e
		}
		missing = still
	}

	for i := range m.Hexes {
		h := &m.Hexes[i]
		switch {
		case kinds[i] == KindBorder:
			h.Elevation = DeepElevation
		case h.Landform == LandformSaltWater:
			switch {
			case h.HasFlag(FlagInlandSea):
				h.Elevation = InlandSeaElevation
			case h.Depth == DepthShallow:
				h.Elevation = ShallowElevation
			case h.Depth == DepthOpen:
				h.Elevation = OpenElevation
			default:
				h.Elevation = DeepElevation
			}
		default:
			sc, sr := f.toSource(h.Col, h.Row)
			e, ok := elev[[2]int{sc, sr}]
			if !ok {
				return fmt.Errorf("%s hex (%d, %d) has no median elevation", h.Landform, h.Col, h.Row)
			}
			h.Elevation = e
		}
	}
	return nil
}

// ownedSides are hmz2ele's owned sides: the side index in Sides and the
// direction of the hex across.
var ownedSides = map[hmz2ele.Side]struct {
	index int
	dir   hmz2ele.Direction
}{
	hmz2ele.NorthSide:     {0, hmz2ele.North},
	hmz2ele.NorthEastSide: {1, hmz2ele.NorthEast},
	hmz2ele.SouthEastSide: {2, hmz2ele.SouthEast},
}

// addRivers lists each river edge on both hexes that share it.
func addRivers(m *Map, f frame, kinds []Kind, edges []hmz2riv.EdgeJSON, rep *Report) error {
	for _, e := range edges {
		own, ok := ownedSides[e.Side]
		if !ok {
			return fmt.Errorf("river edge (%d, %d) %v: invalid side", e.Col, e.Row, e.Side)
		}
		from := hmz2ele.VertexKey{Col: e.From.Col, Row: e.From.Row, Corner: e.From.Corner}
		to := hmz2ele.VertexKey{Col: e.To.Col, Row: e.To.Row, Corner: e.To.Corner}
		nc, nr := hmz2ele.Neighbor(e.Col, e.Row, own.dir)
		for _, side := range [2]struct{ col, row, index int }{
			{e.Col, e.Row, own.index},
			{nc, nr, own.index + 3},
		} {
			mc, mr := f.toMap(side.col, side.row)
			h := m.At(mc, mr)
			if h == nil {
				rep.EdgesOffMap++
				continue
			}
			if kinds[mr*m.Columns+mc] != KindSource {
				rep.EdgesTouching[kinds[mr*m.Columns+mc]]++
			}
			// The side runs clockwise from corner a to corner b.
			a, b := (side.index+4)%6, (side.index+5)%6
			ka := hmz2ele.CornerOwner(side.col, side.row, a)
			kb := hmz2ele.CornerOwner(side.col, side.row, b)
			var flow Corner
			switch {
			case ka == to && kb == from:
				flow = Corners[a]
			case kb == to && ka == from:
				flow = Corners[b]
			default:
				return fmt.Errorf("river edge (%d, %d) %v: vertices %v → %v aren't the ends of side %s of source hex (%d, %d)",
					e.Col, e.Row, e.Side, from, to, Sides[side.index], side.col, side.row)
			}
			s := Sides[side.index]
			if slices.ContainsFunc(h.Rivers, func(r River) bool { return r.Side == s }) {
				return fmt.Errorf("hex (%d, %d): two river edges on side %s", h.Col, h.Row, s)
			}
			h.Rivers = append(h.Rivers, River{Side: s, Flow: flow, DrainageKm2: e.DrainageKm2, Size: SizeOf(e.DrainageKm2)})
		}
	}
	for i := range m.Hexes {
		slices.SortFunc(m.Hexes[i].Rivers, func(a, b River) int { return sideIndex(a.Side) - sideIndex(b.Side) })
	}
	return nil
}
