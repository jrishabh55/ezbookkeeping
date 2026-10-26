//go:build ignore

package main

import (
	"fmt"
	"github.com/go-pdf/fpdf"
	"os"
	"sort"
)

func main() {
	missing := map[int]bool{}
	for _, style := range []string{"", "-Bold"} {
		serif, e := fpdf.TtfParse("fonts/DejaVuSerif" + style + ".ttf")
		if e != nil {
			panic(e)
		}
		sans, e := fpdf.TtfParse("fonts/DejaVuSans" + style + ".ttf")
		if e != nil {
			panic(e)
		}
		for r, g := range sans.Chars {
			if g != 0 && serif.Chars[r] == 0 {
				missing[int(r)] = true
			}
		}
	}
	codes := make([]int, 0, len(missing))
	for r := range missing {
		codes = append(codes, r)
	}
	sort.Ints(codes)
	f, e := os.Create("font_coverage.go")
	if e != nil {
		panic(e)
	}
	defer f.Close()
	fmt.Fprintln(f, "// Code generated from the bundled DejaVu 2.37 cmap tables using fpdf.TtfParse. DO NOT EDIT.")
	fmt.Fprintln(f, "package invoicepdf\n\nimport \"unicode\"\n\n// Characters present in DejaVu Sans but absent from Serif in either regular or bold.\nvar serifFallbackCharacters = &unicode.RangeTable{R16: []unicode.Range16{")
	ranges := 0
	for i := 0; i < len(codes); {
		lo, hi := codes[i], codes[i]
		i++
		for i < len(codes) && codes[i] == hi+1 {
			hi = codes[i]
			i++
		}
		fmt.Fprintf(f, "{Lo:0x%04x,Hi:0x%04x,Stride:1},\n", lo, hi)
		ranges++
	}
	fmt.Fprintln(f, "}}")
	fmt.Printf("Recorded %d fallback characters in %d compact ranges.\n", len(codes), ranges)
}
