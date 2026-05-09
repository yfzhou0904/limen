package main

import (
	"database/sql"
	"fmt"
	"time"
)

// Material represents a unit of content the user added: a markdown note,
// a fetched webpage, or a parsed PDF. Blobs live on disk under blobsRoot.
type Material struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`  // "note" | "webpage" | "pdf"
	Title     string `json:"title"`
	Source    string `json:"source"` // original URL or filename
	Ext       string `json:"ext"`    // "md" | "pdf"
	PageCount int    `json:"page_count"`
	CreatedAt int64  `json:"created_at"`
}

// Request is one Ask turn: persisted so the UI can poll for progress and
// users can revisit past asks. Status: pending → running → ready|error.
type Request struct {
	ID        string         `json:"id"`
	Prompt    string         `json:"prompt"`
	Status    string         `json:"status"`
	Response  string         `json:"-"`
	Error     string         `json:"error,omitempty"`
	CreatedAt int64          `json:"created_at"`
	Events    []RequestEvent `json:"events,omitempty"`
}

// RequestEvent is one step in the agent loop: tool_use | tool_result |
// assistant_text | final. Payload is opaque JSON.
type RequestEvent struct {
	ID        int64  `json:"id"`
	RequestID string `json:"request_id"`
	Seq       int    `json:"seq"`
	Kind      string `json:"kind"`
	Payload   string `json:"payload"`
}

func initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS materials (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			title TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			ext TEXT NOT NULL,
			page_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS requests (
			id TEXT PRIMARY KEY,
			prompt TEXT NOT NULL,
			status TEXT NOT NULL,
			response TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS request_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			kind TEXT NOT NULL,
			payload TEXT NOT NULL,
			FOREIGN KEY(request_id) REFERENCES requests(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_request_events_req ON request_events(request_id, seq)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

func insertMaterial(m Material) error {
	_, err := db.Exec(`INSERT INTO materials(id,kind,title,source,ext,page_count,created_at) VALUES(?,?,?,?,?,?,?)`,
		m.ID, m.Kind, m.Title, m.Source, m.Ext, m.PageCount, m.CreatedAt)
	return err
}

func getMaterial(id string) (Material, error) {
	var m Material
	err := db.QueryRow(`SELECT id,kind,title,source,ext,page_count,created_at FROM materials WHERE id=?`, id).
		Scan(&m.ID, &m.Kind, &m.Title, &m.Source, &m.Ext, &m.PageCount, &m.CreatedAt)
	return m, err
}

func listMaterials() ([]Material, error) {
	rows, err := db.Query(`SELECT id,kind,title,source,ext,page_count,created_at FROM materials ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Material{}
	for rows.Next() {
		var m Material
		if err := rows.Scan(&m.ID, &m.Kind, &m.Title, &m.Source, &m.Ext, &m.PageCount, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func updateMaterialTitle(id, title string) error {
	res, err := db.Exec(`UPDATE materials SET title=? WHERE id=?`, title, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func deleteMaterial(id string) error {
	_, err := db.Exec(`DELETE FROM materials WHERE id=?`, id)
	return err
}

func newID() string { return fmt.Sprintf("mat_%d", time.Now().UnixNano()) }
func newReqID() string { return fmt.Sprintf("req_%d", time.Now().UnixNano()) }

func createRequest(r Request) error {
	_, err := db.Exec(`INSERT INTO requests(id,prompt,status,response,error,created_at) VALUES(?,?,?,?,?,?)`,
		r.ID, r.Prompt, r.Status, r.Response, r.Error, r.CreatedAt)
	return err
}

func updateRequestStatus(id, status, response, errMsg string) error {
	_, err := db.Exec(`UPDATE requests SET status=?, response=?, error=? WHERE id=?`, status, response, errMsg, id)
	return err
}

func appendRequestEvent(requestID string, seq int, kind, payload string) error {
	_, err := db.Exec(`INSERT INTO request_events(request_id,seq,kind,payload) VALUES(?,?,?,?)`,
		requestID, seq, kind, payload)
	return err
}

func getRequest(id string) (Request, error) {
	var r Request
	err := db.QueryRow(`SELECT id,prompt,status,response,error,created_at FROM requests WHERE id=?`, id).
		Scan(&r.ID, &r.Prompt, &r.Status, &r.Response, &r.Error, &r.CreatedAt)
	if err != nil {
		return r, err
	}
	rows, err := db.Query(`SELECT id,request_id,seq,kind,payload FROM request_events WHERE request_id=? ORDER BY seq ASC`, id)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var ev RequestEvent
		if err := rows.Scan(&ev.ID, &ev.RequestID, &ev.Seq, &ev.Kind, &ev.Payload); err != nil {
			return r, err
		}
		r.Events = append(r.Events, ev)
	}
	return r, nil
}

func listRequests() ([]Request, error) {
	rows, err := db.Query(`SELECT id,prompt,status,response,error,created_at FROM requests ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		var r Request
		if err := rows.Scan(&r.ID, &r.Prompt, &r.Status, &r.Response, &r.Error, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func deleteRequest(id string) error {
	_, err := db.Exec(`DELETE FROM requests WHERE id=?`, id)
	return err
}

func reconcileStaleRequests(msg string) error {
	_, err := db.Exec(`UPDATE requests SET status='error', error=? WHERE status IN ('pending','running')`, msg)
	return err
}
