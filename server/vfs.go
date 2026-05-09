package main

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// VFS layout (read-only, all materials):
//   /notes/<slug>.md                    (markdown note: typed, uploaded .md)
//   /notes/<slug>/                      (paginated PDF — directory, also a "note")
//   /notes/<slug>/page_{N}.md           (mistral OCR for one page)
//   /notes/<slug>/page_{N}.pdf          (original page bytes)
//   /webpages/<slug>.md                 (pure.md fetched url)

type vfsEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

type fileKind int

const (
	fileText fileKind = iota
	filePDF
)

type fileContent struct {
	Kind  fileKind
	Text  string
	Bytes []byte
}

type vfsNode struct {
	ID    string
	Kind  string // "note" | "webpage" | "pdf"
	Slug  string
	Ext   string // "md" | "pdf"
	Pages int
}

func kindDir(kind string) string {
	switch kind {
	case "note", "pdf":
		return "notes"
	case "webpage":
		return "webpages"
	}
	return kind
}

func loadVFS(_ context.Context) ([]vfsNode, error) {
	mats, err := listMaterials()
	if err != nil {
		return nil, err
	}
	used := map[string]int{}
	out := make([]vfsNode, 0, len(mats))
	// reverse so oldest gets the unsuffixed slug (consistent with classroom-aid)
	sort.Slice(mats, func(i, j int) bool { return mats[i].CreatedAt < mats[j].CreatedAt })
	for _, m := range mats {
		base := slugify(m.Title)
		key := kindDir(m.Kind) + "/" + base
		slug := base
		if used[key] > 0 {
			slug = fmt.Sprintf("%s-%d", base, used[key]+1)
		}
		used[key]++
		out = append(out, vfsNode{ID: m.ID, Kind: m.Kind, Slug: slug, Ext: m.Ext, Pages: m.PageCount})
	}
	return out, nil
}

func vfsLs(ctx context.Context, p string) ([]vfsEntry, error) {
	nodes, err := loadVFS(ctx)
	if err != nil {
		return nil, err
	}
	p = normalize(p)

	if p == "/" {
		dirs := map[string]int{}
		for _, n := range nodes {
			dirs[kindDir(n.Kind)]++
		}
		out := []vfsEntry{}
		for _, k := range []string{"notes", "webpages"} {
			if dirs[k] > 0 {
				out = append(out, vfsEntry{Name: k, IsDir: true, Size: int64(dirs[k])})
			}
		}
		return out, nil
	}

	if parts := matchN(p, `^/([^/]+)$`); parts != nil {
		dir := parts[0]
		out := []vfsEntry{}
		for _, n := range nodes {
			if kindDir(n.Kind) != dir {
				continue
			}
			if n.Ext == "pdf" {
				out = append(out, vfsEntry{Name: n.Slug, IsDir: true, Size: int64(n.Pages)})
			} else {
				out = append(out, vfsEntry{Name: n.Slug + "." + n.Ext, IsDir: false})
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("not found: %s", p)
		}
		return out, nil
	}

	if parts := matchN(p, `^/notes/([^/]+)$`); parts != nil {
		n := findNode(nodes, "pdf", parts[0])
		if n == nil {
			return nil, fmt.Errorf("not found: %s", p)
		}
		out := []vfsEntry{}
		for i := 1; i <= n.Pages; i++ {
			out = append(out,
				vfsEntry{Name: fmt.Sprintf("page_%d.md", i)},
				vfsEntry{Name: fmt.Sprintf("page_%d.pdf", i)},
			)
		}
		return out, nil
	}
	return nil, fmt.Errorf("not a directory: %s", p)
}

func vfsRead(ctx context.Context, p string) (*fileContent, error) {
	nodes, err := loadVFS(ctx)
	if err != nil {
		return nil, err
	}
	p = normalize(p)

	// /notes/<slug>.md or /webpages/<slug>.md
	if parts := matchN(p, `^/(notes|webpages)/([^/]+)\.md$`); parts != nil {
		kind := "note"
		if parts[0] == "webpages" {
			kind = "webpage"
		}
		n := findNode(nodes, kind, parts[1])
		if n == nil {
			return nil, fmt.Errorf("not found: %s", p)
		}
		b, err := blobGet(fmt.Sprintf("materials/%s/content.md", n.ID))
		if err != nil {
			return nil, err
		}
		return &fileContent{Kind: fileText, Text: string(b)}, nil
	}
	// /notes/<slug>/page_{N}.md
	if parts := matchN(p, `^/notes/([^/]+)/page_(\d+)\.md$`); parts != nil {
		n := findNode(nodes, "pdf", parts[0])
		page, _ := strconv.Atoi(parts[1])
		if n == nil || page < 1 || page > n.Pages {
			return nil, fmt.Errorf("not found: %s", p)
		}
		b, err := blobGet(fmt.Sprintf("materials/%s/page_%d.md", n.ID, page))
		if err != nil {
			return nil, err
		}
		return &fileContent{Kind: fileText, Text: string(b)}, nil
	}
	// /notes/<slug>/page_{N}.pdf
	if parts := matchN(p, `^/notes/([^/]+)/page_(\d+)\.pdf$`); parts != nil {
		n := findNode(nodes, "pdf", parts[0])
		page, _ := strconv.Atoi(parts[1])
		if n == nil || page < 1 || page > n.Pages {
			return nil, fmt.Errorf("not found: %s", p)
		}
		b, err := blobGet(fmt.Sprintf("materials/%s/page_%d.pdf", n.ID, page))
		if err != nil {
			return nil, err
		}
		return &fileContent{Kind: filePDF, Bytes: b}, nil
	}
	return nil, fmt.Errorf("not a file: %s", p)
}

func vfsOverview(ctx context.Context) (string, error) {
	nodes, err := loadVFS(ctx)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Workspace map (every material; titles in quotes):\n")
	if len(nodes) == 0 {
		b.WriteString("(empty — user has not added any materials yet)\n")
		return b.String(), nil
	}
	mats, _ := listMaterials()
	idTitle := map[string]string{}
	idSource := map[string]string{}
	for _, m := range mats {
		idTitle[m.ID] = m.Title
		idSource[m.ID] = m.Source
	}
	for _, n := range nodes {
		title := idTitle[n.ID]
		extra := ""
		if n.Kind == "webpage" && idSource[n.ID] != "" {
			extra = "  source=" + idSource[n.ID]
		}
		if n.Ext == "pdf" {
			fmt.Fprintf(&b, "/notes/%s/  \"%s\" (pdf, %d pages)%s\n", n.Slug, title, n.Pages, extra)
		} else {
			fmt.Fprintf(&b, "/%s/%s.md  \"%s\"%s\n", kindDir(n.Kind), n.Slug, title, extra)
		}
	}
	return b.String(), nil
}

func findNode(nodes []vfsNode, kind, slug string) *vfsNode {
	for i := range nodes {
		if nodes[i].Kind == kind && nodes[i].Slug == slug {
			return &nodes[i]
		}
	}
	return nil
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	return s
}

func normalize(p string) string {
	if p == "" {
		return "/"
	}
	return path.Clean("/" + strings.TrimSpace(p))
}

func matchN(s, pattern string) []string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	return m[1:]
}
