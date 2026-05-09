package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxUpload = 32 << 20 // 32 MiB

func materialsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mats, err := listMaterials()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, mats)
	case http.MethodPost:
		uploadMaterial(w, r)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func uploadMaterial(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", 400)
		return
	}
	defer file.Close()

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(hdr.Filename, path.Ext(hdr.Filename))
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(hdr.Filename), "."))

	body, err := io.ReadAll(io.LimitReader(file, maxUpload))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	switch ext {
	case "md", "markdown":
		ext = "md"
		m := Material{
			ID:        newID(),
			Kind:      "note",
			Title:     title,
			Source:    hdr.Filename,
			Ext:       "md",
			CreatedAt: time.Now().Unix(),
		}
		if err := blobPut(fmt.Sprintf("materials/%s/content.md", m.ID), body); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := insertMaterial(m); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, m)
	case "pdf":
		pages, err := splitPDF(body)
		if err != nil {
			http.Error(w, "split: "+err.Error(), 500)
			return
		}
		ocrPages, err := mistralOCR(r.Context(), body)
		if err != nil {
			http.Error(w, "ocr: "+err.Error(), 500)
			return
		}
		if len(ocrPages) != len(pages) {
			// not fatal but log
			fmt.Printf("warn: ocr returned %d pages, split produced %d\n", len(ocrPages), len(pages))
		}
		m := Material{
			ID:        newID(),
			Kind:      "pdf",
			Title:     title,
			Source:    hdr.Filename,
			Ext:       "pdf",
			PageCount: len(pages),
			CreatedAt: time.Now().Unix(),
		}
		for i, b := range pages {
			if err := blobPut(fmt.Sprintf("materials/%s/page_%d.pdf", m.ID, i+1), b); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		for _, p := range ocrPages {
			page := p.Index + 1
			if err := blobPut(fmt.Sprintf("materials/%s/page_%d.md", m.ID, page), []byte(p.Markdown)); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		if err := insertMaterial(m); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, m)
	default:
		http.Error(w, "only .pdf and .md supported", 400)
	}
}

func materialNoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		http.Error(w, "title required", 400)
		return
	}
	m := Material{
		ID:        newID(),
		Kind:      "note",
		Title:     title,
		Source:    "typed",
		Ext:       "md",
		CreatedAt: time.Now().Unix(),
	}
	if err := blobPut(fmt.Sprintf("materials/%s/content.md", m.ID), []byte(in.Content)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := insertMaterial(m); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, m)
}

func materialURLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	in.URL = strings.TrimSpace(in.URL)
	if in.URL == "" {
		http.Error(w, "url required", 400)
		return
	}
	if _, err := url.ParseRequestURI(in.URL); err != nil {
		http.Error(w, "invalid url", 400)
		return
	}
	md, err := fetchPureMD(r.Context(), in.URL)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = deriveTitle(md, in.URL)
	}
	m := Material{
		ID:        newID(),
		Kind:      "webpage",
		Title:     title,
		Source:    in.URL,
		Ext:       "md",
		CreatedAt: time.Now().Unix(),
	}
	if err := blobPut(fmt.Sprintf("materials/%s/content.md", m.ID), []byte(md)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := insertMaterial(m); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, m)
}

// /api/materials/{id}                  -> GET material metadata
// /api/materials/{id}/content          -> GET raw markdown (for note/webpage)
// /api/materials/{id}/pages/{n}.pdf    -> GET single page bytes
// /api/materials/{id}/pages/{n}.md     -> GET single page markdown
func materialItemHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/materials/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "bad path", 400)
		return
	}
	id := parts[0]
	m, err := getMaterial(id)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, m)
		case http.MethodPatch:
			var in struct {
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			title := strings.TrimSpace(in.Title)
			if title == "" {
				http.Error(w, "title required", 400)
				return
			}
			if err := updateMaterialTitle(id, title); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			m.Title = title
			writeJSON(w, m)
		case http.MethodDelete:
			if err := deleteMaterial(id); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			// best-effort: also remove the blob dir
			_ = blobDeleteDir(fmt.Sprintf("materials/%s", id))
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", 405)
		}
		return
	}
	switch parts[1] {
	case "content":
		b, err := blobGet(fmt.Sprintf("materials/%s/content.md", m.ID))
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Write(b)
	case "pages":
		if len(parts) < 3 {
			http.Error(w, "missing page", 400)
			return
		}
		fname := parts[2]
		dot := strings.LastIndex(fname, ".")
		if dot < 1 {
			http.Error(w, "bad page", 400)
			return
		}
		n, err := strconv.Atoi(fname[:dot])
		if err != nil || n < 1 || n > m.PageCount {
			http.Error(w, "bad page number", 400)
			return
		}
		ext := fname[dot+1:]
		switch ext {
		case "pdf":
			b, err := blobGet(fmt.Sprintf("materials/%s/page_%d.pdf", m.ID, n))
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			w.Header().Set("Content-Type", "application/pdf")
			w.Write(b)
		case "md":
			b, err := blobGet(fmt.Sprintf("materials/%s/page_%d.md", m.ID, n))
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			w.Write(b)
		default:
			http.Error(w, "ext must be pdf or md", 400)
		}
	default:
		http.Error(w, "unknown subresource", 404)
	}
}

func askHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	q := strings.TrimSpace(in.Question)
	if q == "" {
		http.Error(w, "question required", 400)
		return
	}
	req := Request{ID: newReqID(), Prompt: q, Status: "pending", CreatedAt: time.Now().Unix()}
	if err := createRequest(req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	go runAgentForRequest(req.ID, q)
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]any{"id": req.ID})
}

// runAgentForRequest is the background worker. Detached from the HTTP
// request context so cancel-on-disconnect doesn't kill the agent loop.
func runAgentForRequest(reqID, question string) {
	ctx := context.Background()
	_, _ = db.Exec(`UPDATE requests SET status='running' WHERE id=?`, reqID)

	var seqMu sync.Mutex
	var seq int
	emit := func(kind string, payload any) {
		seqMu.Lock()
		seq++
		n := seq
		seqMu.Unlock()
		raw, _ := json.Marshal(payload)
		_ = appendRequestEvent(reqID, n, kind, string(raw))
	}

	resp, err := runAgent(ctx, question, emit)
	if err != nil {
		_ = updateRequestStatus(reqID, "error", "", err.Error())
		return
	}
	raw, _ := json.Marshal(resp)
	_ = updateRequestStatus(reqID, "ready", string(raw), "")
}

func requestsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	rs, err := listRequests()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, rs)
}

func requestItemHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "bad path", 400)
		return
	}
	switch r.Method {
	case http.MethodGet:
		req, err := getRequest(id)
		if err == sql.ErrNoRows {
			http.Error(w, "not found", 404)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out := map[string]any{
			"id":         req.ID,
			"prompt":     req.Prompt,
			"status":     req.Status,
			"error":      req.Error,
			"created_at": req.CreatedAt,
			"events":     req.Events,
		}
		if req.Response != "" {
			out["response"] = json.RawMessage(req.Response)
		}
		writeJSON(w, out)
	case http.MethodDelete:
		if err := deleteRequest(id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func deriveTitle(md, fallback string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
