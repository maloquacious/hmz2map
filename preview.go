// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package hmz2map

import (
	"fmt"
	"image"
	"math"

	"github.com/maloquacious/hmz2bio"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2riv"
	"github.com/maloquacious/hmz2ter"
)

// RenderPreview draws the map with hmz2bio's preview, on the map's own
// layout, with hexes apothem pixels tall. Rivers are drawn from the hexes'
// river lists.
func RenderPreview(m *Map, apothem, scale int) (image.Image, error) {
	side := 2 * float64(apothem) / math.Sqrt(3)
	// The raster ends at the last column's east corner.
	g, err := hmz2ele.NewGrid(apothem, int(math.Ceil(2*side+1.5*side*float64(m.Columns-1))), 2*apothem*m.Rows+1)
	if err != nil {
		return nil, err
	}
	if g.Columns != m.Columns || g.Rows != m.Rows {
		return nil, fmt.Errorf("preview grid is %d × %d, want %d × %d", g.Columns, g.Rows, m.Columns, m.Rows)
	}
	hexes := make([]hmz2bio.HexJSON, len(m.Hexes))
	var edges []hmz2riv.EdgeJSON
	for i := range m.Hexes {
		h := &m.Hexes[i]
		col := h.Col
		if !g.Contains(col, h.Row) {
			return nil, fmt.Errorf("preview grid lacks hex (%d, %d)", h.Col, h.Row)
		}
		j := &hexes[i].HexJSON
		j.Col, j.Row = col, h.Row
		j.Landform = hmz2ter.Landform(h.Landform)
		j.Surface = hmz2ter.Surface(h.Surface)
		j.Biome = hmz2ter.Biome(h.Biome)
		j.Depth = hmz2ter.Depth(h.Depth)
		for _, f := range h.Flags {
			j.Flags = append(j.Flags, hmz2ter.Flag(f))
		}
		for _, r := range h.Rivers {
			a, b := r.Side.Corners()
			from, to := a, b
			if r.Flow == a {
				from, to = b, a
			}
			fk := hmz2ele.CornerOwner(col, h.Row, cornerIndex(from))
			tk := hmz2ele.CornerOwner(col, h.Row, cornerIndex(to))
			edges = append(edges, hmz2riv.EdgeJSON{
				From: hmz2riv.VertexJSON{Col: fk.Col, Row: fk.Row, Corner: fk.Corner},
				To:   hmz2riv.VertexJSON{Col: tk.Col, Row: tk.Row, Corner: tk.Corner},
			})
		}
	}
	return hmz2bio.RenderPreview(g, hexes, edges, scale)
}

func cornerIndex(c Corner) int {
	for i, d := range Corners {
		if d == c {
			return i
		}
	}
	panic("invalid corner " + string(c))
}
