package main

import (
	"crypto/subtle"
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed all:dist
var distFS embed.FS

var (
	db        *sql.DB
	blobsRoot string
)

func main() {
	dataDir := getenv("DATA_DIR", "./data")
	must(os.MkdirAll(dataDir, 0o755))
	blobsRoot = filepath.Join(dataDir, "blobs")
	must(os.MkdirAll(blobsRoot, 0o755))

	dbPath := getenv("DB_PATH", filepath.Join(dataDir, "data.db"))
	var err error
	db, err = sql.Open("sqlite", dbPath)
	must(err)
	must(initSchema())
	if err := reconcileStaleRequests("server restarted while request was in flight"); err != nil {
		log.Printf("reconcile stale requests: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/materials", materialsHandler)
	mux.HandleFunc("/api/materials/url", materialURLHandler)
	mux.HandleFunc("/api/materials/note", materialNoteHandler)
	mux.HandleFunc("/api/materials/", materialItemHandler)
	mux.HandleFunc("/api/ask", askHandler)
	mux.HandleFunc("/api/requests", requestsHandler)
	mux.HandleFunc("/api/requests/", requestItemHandler)

	sub, _ := fs.Sub(distFS, "dist")
	mux.Handle("/", spaHandler(sub))

	var handler http.Handler = mux
	if code := os.Getenv("ACCESS_CODE"); code != "" {
		handler = basicAuth(code, mux)
	}

	addr := getenv("ADDR", ":8080")
	log.Printf("listening on %s", addr)
	must(http.ListenAndServe(addr, handler))
}

// spaHandler serves embedded UI; falls back to index.html for unknown paths.
func spaHandler(sub fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := sub.Open(r.URL.Path[1:])
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// fallback to index.html
		idx, err := sub.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		idx.Close()
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func basicAuth(code string, next http.Handler) http.Handler {
	expected := []byte(code)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, got, ok := r.BasicAuth()
		if !ok || subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="limen"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
