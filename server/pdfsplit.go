package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

var pageNumRe = regexp.MustCompile(`_(\d+)\.pdf$`)

func splitPDF(input []byte) ([][]byte, error) {
	tmpIn, err := os.CreateTemp("", "split-in-*.pdf")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpIn.Name())
	if _, err := tmpIn.Write(input); err != nil {
		tmpIn.Close()
		return nil, err
	}
	tmpIn.Close()

	outDir, err := os.MkdirTemp("", "split-out-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outDir)

	if err := api.SplitFile(tmpIn.Name(), outDir, 1, nil); err != nil {
		return nil, fmt.Errorf("pdfcpu split: %w", err)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".pdf" {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("pdfcpu produced no pages")
	}
	sort.Slice(names, func(i, j int) bool { return pageOrd(names[i]) < pageOrd(names[j]) })
	pages := make([][]byte, 0, len(names))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(outDir, n))
		if err != nil {
			return nil, err
		}
		pages = append(pages, b)
	}
	return pages, nil
}

func pageOrd(name string) int {
	m := pageNumRe.FindStringSubmatch(name)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
