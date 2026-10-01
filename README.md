# hmz2map

`hmz2map` is the last step of the campaign-map pipeline: the map emitter.
It reads an [`hmz2bio`](https://github.com/maloquacious/hmz2bio) climate file and an [`hmz2riv`](https://github.com/maloquacious/hmz2riv) rivers file and writes the game engine's hex map as JSON, with an optional PNG preview.

The JSON holds only what the engine needs: each hex's column, row, landform, surface, biome, depth, flags, elevation, and river edges.
Converting it to images, Worldographer, or other formats is a separate application's job.

The JSON's Go types are exported from this package, so the engine and the converter can import them.
The types are the schema; there is no separate JSON Schema document.

## Usage

```text
go run ./cmd/hmz2map [flags] <climate.json>
```

Flags:

- `-rivers <file>` is the `hmz2riv` JSON file whose river edges go on the map. Required; its grid must match the climate file's.
- `-border` adds a band of deep ocean, 4 hexes wide, around the map (see [Border](#border)).
- `-hex-km <km>` is a hex's flat-to-flat size, recorded in the output. The default is `10`.
- `-output <file>` is the JSON file to write. Required.
- `-preview <file>` is a PNG preview to write. Optional.
- `-preview-scale <n>` sets how many raster pixels, in each direction, one preview pixel covers. The default is `8`.
- `-version` prints the version.

The command prints the time taken by each phase and the emitted counts.
A full run on the Panama map takes under a second.

The climate file is read with `hmz2bio`'s types and the rivers file with `hmz2riv`'s; the grid geometry comes from the `hmz2ele` package, rebuilt from the heightmap dimensions and apothem recorded in the climate file.

## Coordinates

The emitted map is a rectangle of flat-top hexes, numbered from 0 by **column** (left to right) and **row** (top to bottom).
Hex (0, 0) is in the top-left corner.
**Odd columns are pushed down** half a hex: (1, 0) sits half a hex lower than (0, 0) and (2, 0).

The source grid, `hmz2ele`'s, has the same layout: 0-based, with its odd columns pushed down.
With a border width `b` (0, or 4 with `-border`), a source hex (`c`, `r`) is emitted as:

```text
column = c + b
row    = r + b
```

`b` is even, so every hex keeps its place: two hexes are neighbors on the map exactly when they are neighbors in the source grid.

A source grid of `C` columns and `R` rows gives a map of `C + 2b` columns and `R + 2b` rows, and every (column, row) in that rectangle is exactly one hex.
The Panama grid is 106 × 222, so the map is **106 × 222**, or **114 × 230** with the border.

### Neighbors

In map coordinates, the neighbors of hex (`c`, `r`) are:

| Side | `c` even (up)  | `c` odd (down) |
| ---- | -------------- | -------------- |
| `n`  | (c, r − 1)     | (c, r − 1)     |
| `ne` | (c + 1, r − 1) | (c + 1, r)     |
| `se` | (c + 1, r)     | (c + 1, r + 1) |
| `s`  | (c, r + 1)     | (c, r + 1)     |
| `sw` | (c − 1, r)     | (c − 1, r + 1) |
| `nw` | (c − 1, r − 1) | (c − 1, r)     |

`Neighbor` returns them; a neighbor may be off the map.

### Kinds of hex

Every hex on the map is one of three kinds:

- A **border** hex is within `b` hexes of the map's edge: column below `b` or at least `columns − b`, or row below `b` or at least `rows − b`. There are none without `-border`.
- A **source** hex is one that the source grid has.
- A **fill** hex is any other: the source grid has no hex there.

The fill hexes are only the slots the source grid lacks.
The source grid isn't a full rectangle: its hexes exist where their center pixel is inside the raster, so on Panama its even columns have 222 rows and its odd columns 221.
Its 53 odd columns lack row 221, which its even columns have.
Panama has **53** fill hexes.

## Hexes

Every hex has a landform, a surface, and a biome; salt water also has a depth band.
In this README, **land** means the seven landforms shaped by relief (`flats` through `volcanic-highlands`); cliffs, badlands, and lakes aren't land.

- A **source** hex keeps `hmz2bio`'s landform, surface, biome, and depth, and its flags without `flat-surface`, in the same order.
  `flat-surface` is `hmz2ter`'s hint to the climate step, not one of the game's flags.
- A **fill** hex is `salt-water`, with surface and biome `clear`.
  Its depth comes from its distance to land (below), and it has the `coast` flag if that distance is 1.
  It is never `inland-sea`: fill hexes are on the map's edge, open to the sea.
- A **border** hex is `salt-water`, `clear`, `clear`, and always `deep`, with no flags.

### Fill-hex depth

A fill hex's distance is the number of steps, through salt water, from the nearest hex that isn't salt water (land, lakes, cliffs, or badlands), counting over the map's source and fill hexes (border hexes don't count) with the six neighbors above.
It is `hmz2ter`'s rule, with its bands: `shallow` for 1–12, `open` for 13–19, and `deep` for 20 or more.
A fill hex that no hex outside salt water reaches is `deep`, as in `hmz2ter`.
Source hexes keep `hmz2ter`'s depth; the command reports how many source hexes would get a different band from the same calculation on the map (0 on Panama).

Like `hmz2ter`, a fill hex's distance counts cliffs and badlands as not salt water, so a fill hex next to a cliff has the `coast` flag; cliffs don't get it.
A land hex next to a fill hex would need the `coast` flag, which `hmz2ter` didn't give it, so `hmz2map` rejects that instead of changing a source hex's flags (see [Validation](#validation)); on Panama, fill hexes touch only salt water and 1 cliff hex, so 1 fill hex is `coast`.

### Border

With `-border`, a band of deep ocean 4 hexes wide surrounds the map: it is decoration, so the players' maps look nicer.
Land and cliffs reach the source's edges (the top row has a hills hex and cliffs, and the last column has cliffs), and the deep border runs right up to them.
Border hexes **never** get the `coast` flag and never change a neighbor's flags or depth.
That is a documented exception to the terrain model, which says `coast` salt water is always `shallow`: deep water next to land is allowed only for border hexes.

### Elevation

Every hex has one integer `elevation`, in meters:

| Hex | Elevation |
| --- | --------- |
| Land (`flats` to `volcanic-highlands`), cliffs, and badlands | the hex's median elevation (`hmz2ter`'s `elevation.median`) |
| Fresh water (lakes) | the hex's median, which is its lake's level |
| Salt water with the `inland-sea` flag | 0 (sea level) |
| Other salt water, fill hexes included | by depth band: `shallow` −5, `open` −50, `deep` −500 |
| Border | −500 |

Lake depth isn't tracked.
Inland seas are at sea level because their medians are useless: most of their hexes have no valid pixels, and the rest come from shore pixels.

Some cliff hexes have no median, because every pixel in them is no-data (73 of Panama's 195).
A cliff or badland with no median takes the **highest** median among its neighbors that are land, cliffs, or badlands (not lakes) and have an elevation: a cliff stands at least as tall as the ground it walls off.
This is done in passes: each pass gives every cliff or badland still without an elevation the highest elevation among its neighbors that had one before the pass, so the result doesn't depend on the order of the hexes.
Passes repeat until every cliff and badland has an elevation; if a pass resolves none, `hmz2map` fails.

## Rivers

`hmz2riv` stores each river edge once, by the hex that owns it (its `n`, `ne`, or `se` side), with its two vertices in flow order and its drainage area.
The map lists each edge on **both** hexes that share it, each describing it from its own side, in a `rivers` list:

- `side`: the hex's side the river runs along: `n`, `ne`, `se`, `s`, `sw`, or `nw`.
- `flow`: the corner of *this* hex the water flows toward: `e`, `se`, `sw`, `w`, `nw`, or `ne`.
- `drainage_km2`: the drainage area, in km² of real ground, as `hmz2riv` wrote it.
- `size`: the size class, from the drainage area.

Each side lies between two corners, numbered clockwise from east as in `hmz2ele` (0 `e`, 1 `se`, 2 `sw`, 3 `w`, 4 `nw`, 5 `ne`, y down):

| Side | Corners    |
| ---- | ---------- |
| `n`  | `nw`, `ne` |
| `ne` | `ne`, `e`  |
| `se` | `e`, `se`  |
| `s`  | `se`, `sw` |
| `sw` | `sw`, `w`  |
| `nw` | `w`, `nw`  |

The owner's `n`, `ne`, and `se` sides are the neighbor's `s`, `sw`, and `nw` sides.
A vertex is shared by three hexes and has a different corner name in each, so the two hexes of an edge name the same downstream vertex differently: water flowing east along the owner's `n` side flows toward its `ne` corner, which is the `se` corner of the hex to the north.
In the [Output](#output) example, hex (50, 4)'s `n` side flows to its `ne` corner; hex (50, 3) lists the same edge as its `s` side, flowing to its `se` corner.
`flow` is the corner of the side that is the edge's downstream (`to`) vertex.

A hex's `rivers` are listed in side order (`n`, `ne`, `se`, `s`, `sw`, `nw`) and omitted when empty.
Rivers are listed on every hex that has the edge, whatever its landform, because river mouths touch the sea.
The `river` flag stays land-only, as the terrain model says: a land hex has it exactly when its `rivers` list isn't empty.

An edge whose other hex is a fill or border hex is listed on that hex too; an edge whose other hex is off the map is listed once.
On Panama, every river edge lies between two source hexes, so each of the 4,604 edges is listed exactly twice.

### Size classes

| Drainage area      | `size`        | Edges on Panama |
| ------------------ | ------------- | --------------: |
| under 250 km²      | `stream`      |   2,952 (64.1%) |
| 250 to 2,000 km²   | `river`       |   1,437 (31.2%) |
| 2,000 km² and more | `great-river` |     215 (4.7%)  |

A drainage area exactly on a break takes the larger class.
The breaks come from the distribution of drainage areas (from 50 to 10,764 km², median 163 km²): a stream is below about the 64th percentile and a great river at about the 95th, which picks out the trunks of the largest rivers (the lower Tuira, Bayano, Chagres, and Santa María).
Drainage areas are real, not campaign: 2,000 km² of real ground is about 23,000 km² on the campaign map.

## Validation

Before writing, `hmz2map` checks every hex, fill and border hexes included, and exits with an error if any breaks the rules.
It checks the terrain model's valid combinations (see the root README's "Terrain model" section), as `hmz2bio` does, and also:

- no hex has the `flat-surface` flag;
- a salt-water hex that isn't a border hex has the `coast` flag exactly when a neighbor on the map, other than a border hex, isn't salt water;
- a land hex has the `coast` flag exactly when a neighbor on the map, other than a border hex, is salt water;
- a land hex has the `river` flag exactly when it has rivers;
- border hexes are deep salt water with no flags;
- every edge a hex lists is also listed by the hex across it, if that hex is on the map, with the same drainage area and size and the same downstream vertex;
- the map is `columns × rows` hexes in row-then-column order.

`coast` salt water must be `shallow`, as the model says; border hexes are the one exception, and only because they never have the flag.

## Results

On the Panama climate (`hmz2bio` v0.3.0) and rivers (`hmz2riv` v0.3.0):

| Measurement | Without border | With `-border` |
| ----------- | -------------: | -------------: |
| Map | 106 × 222 | 114 × 230 |
| Hexes | 23,532 | 26,220 |
| Source hexes | 23,479 | 23,479 |
| Fill hexes | 53 | 53 |
| Border hexes | 0 | 2,688 |
| JSON | 5.6 MB | 6.0 MB |

The border adds only deep salt water, so every other count is the same with and without it.

| Landform           | Hexes  |
| ------------------ | -----: |
| Salt water         | 13,357 (16,045 with the border) |
| Fresh water        |     63 |
| Flats              |    604 |
| Plains             |  1,941 |
| Rolling plains     |  2,495 |
| Hills              |  2,857 |
| Mountains          |  1,967 |
| Plateaus           |     23 |
| Volcanic highlands |     30 |
| Cliffs             |    195 |

| Depth   | Salt-water hexes | Of them fill hexes | Elevation |
| ------- | ---------------: | -----------------: | --------: |
| Shallow |            9,113 |                 29 | −5 m (8,927), 0 m (the 186 inland-sea hexes) |
| Open    |            2,236 |                 11 | −50 m |
| Deep    |            2,008 (4,696 with the border) | 13 | −500 m |

| Flag       | Hexes |
| ---------- | ----: |
| Coast      | 2,318 (1,037 land, and 1,281 water: 1,280 source and 1 fill) |
| River      | 4,119 |
| Volcano    | 3 |
| Inland sea | 186 |
| Impassable | 320 |

| River size    | Edges | Hex sides |
| ------------- | ----: | --------: |
| `stream`      | 2,952 |     5,904 |
| `river`       | 1,437 |     2,874 |
| `great-river` |   215 |       430 |

- The surfaces and biomes of the 9,917 land hexes are `hmz2bio`'s, unchanged.
- Land elevations run from 0 m to 3,194 m (the Barú summit hex), cliffs from 17 m to 3,092 m, and lakes from 28 m to 78 m.
- 73 cliffs had no median; one pass gave all of them an elevation.
- Recomputing distances over the map, fill hexes included, would change no source hex's depth band.
- Every river edge lies between two source hexes, so none touches a fill or border hex and each is listed on exactly two hexes. 4,289 hexes have rivers, including 106 salt-water hexes (river mouths), 59 lake hexes, and 5 cliffs; none of those three has the `river` flag.
- Land and cliffs reach the map's edges: the top row has 1 hills hex and 3 cliffs, and the last column 3 cliffs. With `-border`, deep water is next to them.

Every count was cross-checked against an independent Python calculation, written from this README, with no mismatches.

## Output

```json
{
  "schema_version": 1,
  "hmz2map_version": "0.2.0",
  "layout": "flat-top, 0-based, odd columns down",
  "columns": 106,
  "rows": 222,
  "border": 0,
  "hex_km": 10,
  "sources": [
    { "file_name": "pandemokh-a48-climate.json", "tool": "hmz2bio", "version": "0.3.0" },
    { "file_name": "pandemokh-a48-terrain.json", "tool": "hmz2ter", "version": "0.3.0" },
    { "file_name": "pandemokh-a48-rivers.json", "tool": "hmz2riv", "version": "0.3.0" }
  ],
  "climate": { "top_lat_deg": 27, "bottom_lat_deg": 7 },
  "hexes": [
    { "col": 0, "row": 0, "landform": "salt-water", "surface": "clear", "biome": "clear", "depth": "deep", "elevation": -500 },
    ...
    { "col": 50, "row": 4, "landform": "rolling-plains", "surface": "clear", "biome": "savanna", "flags": [ "river" ], "elevation": 108,
      "rivers": [ { "side": "n", "flow": "ne", "drainage_km2": 59, "size": "stream" },
                  { "side": "ne", "flow": "e", "drainage_km2": 60.7, "size": "stream" },
                  { "side": "se", "flow": "se", "drainage_km2": 63.6, "size": "stream" } ] },
    ...
  ]
}
```

- `schema_version` is the version of the JSON's layout, `SchemaVersion` in Go. It starts at 1 and changes when a change to the layout would break a reader.
  It is still 1 because the format hasn't changed, though every hex's coordinates have: `hmz2map` v0.1.0 emitted source hex (`c`, `r`) one column further right, at (`c + 1 + b`, `r + b`), behind an extra first column of fill hexes.
- `layout` describes the coordinates, for a reader that doesn't import the package.
- `border` is the border's width in hexes: 0, or 4 with `-border`.
- `sources` lists the input files and the tools that wrote them; the terrain file is the one the climate file names.
- `climate` is the climate step's placement: the latitudes of the source raster's top and bottom edges, north positive.
- `hexes` lists every hex, ordered by row, then column, so hex (`c`, `r`) is at index `r × columns + c`.

Each hex has `col`, `row`, `landform`, `surface`, `biome`, and `elevation` (meters); `depth` on salt water only; `flags` when it has any; and `rivers` when it has any.
The enumerations' values are the terrain model's, as Go constants: `Landform`, `Surface`, `Biome`, `Depth`, `Flag`, and, for rivers, `Side`, `Corner`, and `RiverSize`.

## Preview

The preview is `hmz2bio`'s, drawn on the map's own layout: each hex colored as in `hmz2bio`'s preview (land by surface or biome, water by depth, coast water pale, inland seas teal, cliffs red, volcanoes magenta) and each river edge a dark blue line, drawn from the map's `rivers` lists rather than from the rivers file.
With `-border`, the border is the deep-water color all round.
The image covers just the map: it starts at column 0's west corner and ends at the last column's east corner.

## License

MIT. See `LICENSE`.
