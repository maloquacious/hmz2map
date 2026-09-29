// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Package hmz2map emits the game engine's hex map. Its exported types are
// the map's JSON schema: the engine and the map converters import them.
package hmz2map

// SchemaVersion is the version of the map's JSON layout. It changes when a
// change to the layout would break a reader.
const SchemaVersion = 1

// Layout describes the map's coordinates.
const Layout = "flat-top, 0-based, odd columns down"

// Map is the hex map, as written to JSON.
type Map struct {
	SchemaVersion  int    `json:"schema_version"`
	Hmz2mapVersion string `json:"hmz2map_version"`
	Layout         string `json:"layout"`
	Columns        int    `json:"columns"`
	Rows           int    `json:"rows"`
	// Border is the width, in hexes, of the decorative deep-ocean band
	// around the map: 0 or 4.
	Border int `json:"border"`
	// HexKm is a hex's flat-to-flat size, in kilometers.
	HexKm   float64   `json:"hex_km"`
	Sources []Source  `json:"sources"`
	Climate Placement `json:"climate"`
	// Hexes lists every hex, ordered by row, then column: hex (c, r) is at
	// index r × Columns + c.
	Hexes []Hex `json:"hexes"`
}

// Source identifies an input file and the tool that wrote it.
type Source struct {
	FileName string `json:"file_name"`
	Tool     string `json:"tool"`
	Version  string `json:"version"`
}

// Placement is the climate step's placement: the latitudes of the source
// raster's top and bottom edges, north positive.
type Placement struct {
	TopLatDeg    float64 `json:"top_lat_deg"`
	BottomLatDeg float64 `json:"bottom_lat_deg"`
}

// Hex is one hex. Depth is set on salt water only.
type Hex struct {
	Col      int      `json:"col"`
	Row      int      `json:"row"`
	Landform Landform `json:"landform"`
	Surface  Surface  `json:"surface"`
	Biome    Biome    `json:"biome"`
	Depth    Depth    `json:"depth,omitempty"`
	Flags    []Flag   `json:"flags,omitempty"`
	// Elevation is in meters.
	Elevation int `json:"elevation"`
	// Rivers lists the hex's sides that carry a river, in side order.
	Rivers []River `json:"rivers,omitempty"`
}

// River is a river along one side of a hex. The same edge is listed by the
// hex on its other side, from that hex's point of view.
type River struct {
	Side Side `json:"side"`
	// Flow is the corner of this hex that the water flows toward.
	Flow Corner `json:"flow"`
	// DrainageKm2 is the area of real (source) ground draining through the
	// edge.
	DrainageKm2 float64   `json:"drainage_km2"`
	Size        RiverSize `json:"size"`
}

// At returns the hex at (col, row), or nil if it is off the map.
func (m *Map) At(col, row int) *Hex {
	if col < 0 || row < 0 || col >= m.Columns || row >= m.Rows {
		return nil
	}
	return &m.Hexes[row*m.Columns+col]
}

// HasFlag reports whether the hex has the flag.
func (h *Hex) HasFlag(f Flag) bool {
	for _, g := range h.Flags {
		if g == f {
			return true
		}
	}
	return false
}

// Landform is a hex's underlying physical geography.
type Landform string

const (
	LandformEmpty Landform = "" // unset; an error in the map

	// Water.
	LandformFreshWater Landform = "fresh-water" // lakes
	LandformSaltWater  Landform = "salt-water"  // seas, including inland seas

	// Land, by relief.
	LandformFlats             Landform = "flats"
	LandformPlains            Landform = "plains"
	LandformRollingPlains     Landform = "rolling-plains"
	LandformHills             Landform = "hills"
	LandformMountains         Landform = "mountains"
	LandformPlateaus          Landform = "plateaus"
	LandformVolcanicHighlands Landform = "volcanic-highlands"

	// Impassable by normal means.
	LandformCliffs   Landform = "cliffs"
	LandformBadlands Landform = "badlands"
)

// Landforms lists every landform, in the terrain model's order.
var Landforms = []Landform{
	LandformFreshWater, LandformSaltWater,
	LandformFlats, LandformPlains, LandformRollingPlains, LandformHills, LandformMountains, LandformPlateaus, LandformVolcanicHighlands,
	LandformCliffs, LandformBadlands,
}

// IsLand reports whether the landform is passable land shaped by relief.
func (l Landform) IsLand() bool {
	switch l {
	case LandformFlats, LandformPlains, LandformRollingPlains, LandformHills, LandformMountains, LandformPlateaus, LandformVolcanicHighlands:
		return true
	}
	return false
}

// Surface is a hex's surface or hydrological condition.
type Surface string

const (
	SurfaceEmpty      Surface = "" // unset
	SurfaceClear      Surface = "clear"
	SurfaceGlacialIce Surface = "glacial-ice"
	SurfaceMarshes    Surface = "marshes"
	SurfaceSwamps     Surface = "swamps"
	SurfaceBogs       Surface = "bogs"
	SurfaceMangroves  Surface = "mangroves"
	SurfaceSaltFlats  Surface = "salt-flats"
)

// Surfaces lists every surface, in the terrain model's order.
var Surfaces = []Surface{
	SurfaceClear, SurfaceGlacialIce, SurfaceMarshes, SurfaceSwamps, SurfaceBogs, SurfaceMangroves, SurfaceSaltFlats,
}

// Biome is a hex's ecology.
type Biome string

const (
	BiomeEmpty               Biome = "" // unset
	BiomeClear               Biome = "clear"
	BiomeTundra              Biome = "tundra"
	BiomeAlpine              Biome = "alpine"
	BiomeDesert              Biome = "desert"
	BiomeScrubland           Biome = "scrubland"
	BiomeGrassland           Biome = "grassland"
	BiomeSteppe              Biome = "steppe"
	BiomeSavanna             Biome = "savanna"
	BiomeBorealForest        Biome = "boreal-forest"
	BiomeTemperateForest     Biome = "temperate-forest"
	BiomeTemperateRainforest Biome = "temperate-rainforest"
	BiomeTropicalDryForest   Biome = "tropical-dry-forest"
	BiomeTropicalRainforest  Biome = "tropical-rainforest"
	BiomeCloudForest         Biome = "cloud-forest"
)

// Biomes lists every biome, in the terrain model's order.
var Biomes = []Biome{
	BiomeClear, BiomeTundra, BiomeAlpine, BiomeDesert, BiomeScrubland, BiomeGrassland, BiomeSteppe, BiomeSavanna,
	BiomeBorealForest, BiomeTemperateForest, BiomeTemperateRainforest, BiomeTropicalDryForest, BiomeTropicalRainforest, BiomeCloudForest,
}

// Depth is a salt-water hex's depth band, from its distance to land.
type Depth string

const (
	DepthNone    Depth = ""        // fresh water and land
	DepthShallow Depth = "shallow" // up to 12 hexes from land
	DepthOpen    Depth = "open"    // 13 to 19 hexes
	DepthDeep    Depth = "deep"    // 20 hexes and more
)

// Depths lists every depth band, shallowest first.
var Depths = []Depth{DepthShallow, DepthOpen, DepthDeep}

// Flag marks a feature that combines with a landform.
type Flag string

const (
	FlagCoast      Flag = "coast"      // land next to salt water, or salt water next to land
	FlagRiver      Flag = "river"      // land with a river on at least one side
	FlagVolcano    Flag = "volcano"    // from the hand-made list
	FlagInlandSea  Flag = "inland-sea" // salt water in a body cut off from the open sea by a narrow strait
	FlagImpassable Flag = "impassable" // cliffs, badlands, and land along a border cut
)

// Flags lists every flag.
var Flags = []Flag{FlagCoast, FlagRiver, FlagVolcano, FlagInlandSea, FlagImpassable}

// Side is one of a flat-top hex's six sides.
type Side string

const (
	SideN  Side = "n"
	SideNE Side = "ne"
	SideSE Side = "se"
	SideS  Side = "s"
	SideSW Side = "sw"
	SideNW Side = "nw"
)

// Sides lists every side, clockwise from north.
var Sides = [6]Side{SideN, SideNE, SideSE, SideS, SideSW, SideNW}

// Corner is one of a flat-top hex's six corners.
type Corner string

const (
	CornerE  Corner = "e"
	CornerSE Corner = "se"
	CornerSW Corner = "sw"
	CornerW  Corner = "w"
	CornerNW Corner = "nw"
	CornerNE Corner = "ne"
)

// Corners lists every corner, clockwise from east.
var Corners = [6]Corner{CornerE, CornerSE, CornerSW, CornerW, CornerNW, CornerNE}

// Corners returns the two corners at the ends of the side, clockwise.
func (s Side) Corners() (Corner, Corner) {
	i := sideIndex(s)
	return Corners[(i+4)%6], Corners[(i+5)%6]
}

// Opposite returns the side that faces s: the same edge, seen from the
// neighbor across it.
func (s Side) Opposite() Side {
	return Sides[(sideIndex(s)+3)%6]
}

func sideIndex(s Side) int {
	for i, t := range Sides {
		if t == s {
			return i
		}
	}
	panic("invalid side " + string(s))
}

// Neighbor returns the hex across the given side of (col, row). Odd columns
// are pushed down half a hex. The result may be off the map.
func Neighbor(col, row int, s Side) (int, int) {
	odd := col&1 == 1
	switch s {
	case SideN:
		return col, row - 1
	case SideS:
		return col, row + 1
	case SideNE, SideNW:
		dc := 1
		if s == SideNW {
			dc = -1
		}
		if odd {
			return col + dc, row
		}
		return col + dc, row - 1
	case SideSE, SideSW:
		dc := 1
		if s == SideSW {
			dc = -1
		}
		if odd {
			return col + dc, row + 1
		}
		return col + dc, row
	}
	panic("invalid side " + string(s))
}

// RiverSize is a river's size class, from its drainage area.
type RiverSize string

const (
	SizeStream     RiverSize = "stream"      // under 250 km²
	SizeRiver      RiverSize = "river"       // 250 to 2,000 km²
	SizeGreatRiver RiverSize = "great-river" // 2,000 km² and more
)

// RiverSizes lists every size class, smallest first.
var RiverSizes = []RiverSize{SizeStream, SizeRiver, SizeGreatRiver}

// Size class breaks, in km² of real ground. An area exactly on a break
// takes the larger class.
const (
	RiverMinKm2      = 250
	GreatRiverMinKm2 = 2000
)

// SizeOf returns the size class of a river with the given drainage area.
func SizeOf(drainageKm2 float64) RiverSize {
	switch {
	case drainageKm2 >= GreatRiverMinKm2:
		return SizeGreatRiver
	case drainageKm2 >= RiverMinKm2:
		return SizeRiver
	}
	return SizeStream
}
