import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createNote, uploadFile, uploadURL } from "../lib/api";

type Mode = "file" | "url" | "note";

export function AddMaterial() {
  const [mode, setMode] = useState<Mode>("file");
  const [file, setFile] = useState<File | null>(null);
  const [url, setUrl] = useState("");
  const [noteTitle, setNoteTitle] = useState("");
  const [noteContent, setNoteContent] = useState("");
  const qc = useQueryClient();

  const noteMut = useMutation({
    mutationFn: () => createNote(noteTitle.trim(), noteContent),
    onSuccess: () => {
      setNoteTitle("");
      setNoteContent("");
      qc.invalidateQueries({ queryKey: ["materials"] });
    },
  });

  const fileMut = useMutation({
    mutationFn: () => {
      if (!file) throw new Error("no file");
      return uploadFile(file);
    },
    onSuccess: () => {
      setFile(null);
      qc.invalidateQueries({ queryKey: ["materials"] });
    },
  });

  const urlMut = useMutation({
    mutationFn: () => uploadURL(url),
    onSuccess: () => {
      setUrl("");
      qc.invalidateQueries({ queryKey: ["materials"] });
    },
  });

  const busy = fileMut.isPending || urlMut.isPending || noteMut.isPending;

  return (
    <div className="card">
      <div className="tabs">
        <button className={`tab ${mode === "file" ? "active" : ""}`} onClick={() => setMode("file")}>
          Upload PDF / Markdown
        </button>
        <button className={`tab ${mode === "url" ? "active" : ""}`} onClick={() => setMode("url")}>
          From URL
        </button>
        <button className={`tab ${mode === "note" ? "active" : ""}`} onClick={() => setMode("note")}>
          Write note
        </button>
      </div>

      {mode === "note" ? (
        <>
          <div className="row">
            <input
              placeholder="Title"
              value={noteTitle}
              onChange={(e) => setNoteTitle(e.target.value)}
            />
          </div>
          <div className="row">
            <textarea
              placeholder="Markdown content"
              value={noteContent}
              onChange={(e) => setNoteContent(e.target.value)}
              style={{ minHeight: 160 }}
            />
          </div>
          <button onClick={() => noteMut.mutate()} disabled={!noteTitle.trim() || busy}>
            {noteMut.isPending ? "Saving…" : "Save note"}
          </button>
          {noteMut.isError && <span className="why err"> {(noteMut.error as Error).message}</span>}
        </>
      ) : mode === "file" ? (
        <>
          <div className="row">
            <input
              type="file"
              accept=".pdf,.md,.markdown"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </div>
          <button onClick={() => fileMut.mutate()} disabled={!file || busy}>
            {fileMut.isPending ? "Uploading…" : "Upload"}
          </button>
          {fileMut.isError && <span className="why err"> {(fileMut.error as Error).message}</span>}
          {fileMut.isPending && file?.name.endsWith(".pdf") && (
            <span className="why"> Splitting + OCR via Mistral, may take ~30s for a long PDF…</span>
          )}
        </>
      ) : (
        <>
          <div className="row">
            <input
              placeholder="https://example.com/article"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
          </div>
          <button onClick={() => urlMut.mutate()} disabled={!url || busy}>
            {urlMut.isPending ? "Fetching…" : "Fetch via pure.md"}
          </button>
          {urlMut.isError && <span className="why err"> {(urlMut.error as Error).message}</span>}
        </>
      )}
    </div>
  );
}
