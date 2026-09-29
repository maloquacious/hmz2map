// Copyright (c) 2026 Michael D Henderson. All rights reserved.

// Command hmz2map emits the game engine's hex map from an hmz2bio climate
// file and an hmz2riv rivers file, as JSON, with an optional PNG preview.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/maloquacious/hmz2bio"
	"github.com/maloquacious/hmz2ele"
	"github.com/maloquacious/hmz2map"
	"github.com/maloquacious/hmz2riv"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hmz2map: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("hmz2map", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hmz2map [flags] <climate.json>\n\n")
		fs.PrintDefaults()
	}
	riversFile := fs.String("rivers", "", "hmz2riv JSON file with the river edges (required)")
	border := fs.Bool("border", false, fmt.Sprintf("add a band of deep ocean, %d hexes wide, around the map", hmz2map.BorderWidth))
	hexKm := fs.Float64("hex-km", 10, "a hex's flat-to-flat size, in km, recorded in the output")
	output := fs.String("output", "", "JSON file to write (required)")
	preview := fs.String("preview", "", "PNG preview file to write (optional)")
	previewScale := fs.Int("preview-scale", 8, "raster pixels per preview pixel, in each direction")
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, hmz2map.Version())
		return nil
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected one input file, got %d", fs.NArg())
	}
	if *output == "" {
		return fmt.Errorf("-output is required")
	}
	if *riversFile == "" {
		return fmt.Errorf("-rivers is required")
	}
	input := fs.Arg(0)
	start := time.Now()
	phase := func(name string) {
		fmt.Fprintf(stdout, "%-24s %6.1fs\n", name, time.Since(start).Seconds())
	}

	var doc hmz2bio.Document
	if err := readJSON(input, &doc); err != nil {
		return err
	}
	var rivers hmz2riv.Rivers
	if err := readJSON(*riversFile, &rivers); err != nil {
		return err
	}
	phase("read input")
	md := doc.Heightmap.Metadata
	g, err := hmz2ele.NewGrid(doc.Grid.ApothemPx, int(md.Width), int(md.Height))
	if err != nil {
		return err
	}
	if g.Columns != doc.Grid.Columns || g.Rows != doc.Grid.Rows || len(doc.Hexes) != doc.Grid.HexCount {
		return fmt.Errorf("%s: grid is %d × %d with %d hexes, but the heightmap and apothem give %d × %d",
			input, doc.Grid.Columns, doc.Grid.Rows, len(doc.Hexes), g.Columns, g.Rows)
	}
	if rivers.Grid.ApothemPx != g.Apothem || rivers.Grid.Columns != g.Columns || rivers.Grid.Rows != g.Rows {
		return fmt.Errorf("%s: grid doesn't match %s", *riversFile, input)
	}

	opt := hmz2map.Options{HexKm: *hexKm, ClimateFile: filepath.Base(input), RiversFile: filepath.Base(*riversFile)}
	if *border {
		opt.Border = hmz2map.BorderWidth
	}
	m, rep, err := hmz2map.Build(g, &doc, &rivers, opt)
	if err != nil {
		return err
	}
	phase("build")
	if err := hmz2map.Validate(m); err != nil {
		return fmt.Errorf("invalid map:\n%w", err)
	}
	phase("validate")

	if err := writeFile(*output, func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(m)
	}); err != nil {
		return err
	}
	if *preview != "" {
		img, err := hmz2map.RenderPreview(m, g.Apothem, *previewScale)
		if err != nil {
			return err
		}
		if err := writeFile(*preview, func(w io.Writer) error { return png.Encode(w, img) }); err != nil {
			return err
		}
	}
	phase("write output")

	s := hmz2map.Summarize(m)
	fmt.Fprintf(stdout, "map:                %d × %d, border %d\n", m.Columns, m.Rows, m.Border)
	fmt.Fprintf(stdout, "hexes:              %d source, %d fill, %d border\n",
		rep.Kinds[hmz2map.KindSource], rep.Kinds[hmz2map.KindFill], rep.Kinds[hmz2map.KindBorder])
	fmt.Fprintf(stdout, "cliffs filled:      %d in %d passes\n", rep.CliffsFilled, rep.CliffPasses)
	fmt.Fprintf(stdout, "depth changes:      %d source hexes\n", rep.DepthChanges)
	fmt.Fprintf(stdout, "edges on fill:      %d, on border %d, off the map %d\n",
		rep.EdgesTouching[hmz2map.KindFill], rep.EdgesTouching[hmz2map.KindBorder], rep.EdgesOffMap)
	fmt.Fprintf(stdout, "land elevation:     %d to %d m\n", s.LandMin, s.LandMax)
	count := func(title string, keys []string, counts map[string]int) {
		fmt.Fprintf(stdout, "%s:\n", title)
		for _, k := range keys {
			if n := counts[k]; n > 0 {
				fmt.Fprintf(stdout, "  %-22s %6d\n", k+":", n)
			}
		}
	}
	count("landforms", strs(hmz2map.Landforms), s.Landforms)
	count("surfaces (land)", strs(hmz2map.Surfaces), s.Surfaces)
	count("biomes (land)", strs(hmz2map.Biomes), s.Biomes)
	count("depths", strs(hmz2map.Depths), s.Depths)
	count("flags", strs(hmz2map.Flags), s.Flags)
	count("river edges", strs(hmz2map.RiverSizes), s.RiverEdges)
	return nil
}

func strs[T ~string](xs []T) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = string(x)
	}
	return out
}

func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewDecoder(bufio.NewReader(f)).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeFile(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if err := write(w); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	return f.Close()
}
