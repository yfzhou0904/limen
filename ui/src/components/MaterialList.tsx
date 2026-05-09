import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { deleteMaterial, listMaterials, renameMaterial } from "../lib/api";
import type { Material } from "../lib/types";
import { MaterialViewer } from "./MaterialViewer";

export function MaterialList() {
  const { data, isLoading } = useQuery({ queryKey: ["materials"], queryFn: listMaterials });
  const [open, setOpen] = useState<Material | null>(null);

  if (isLoading) return <div className="why">Loading…</div>;
  if (!data || data.length === 0) {
    return <div className="why">No materials yet. Upload a PDF/MD or paste a URL above.</div>;
  }

  return (
    <>
      {data.map((m) => (
        <MaterialRow key={m.id} m={m} onOpen={() => setOpen(m)} />
      ))}
      {open && <MaterialViewer material={open} onClose={() => setOpen(null)} />}
    </>
  );
}

function MaterialRow({ m, onOpen }: { m: Material; onOpen: () => void }) {
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(m.title);
  const qc = useQueryClient();

  const renameMut = useMutation({
    mutationFn: (t: string) => renameMaterial(m.id, t),
    onSuccess: () => {
      setEditing(false);
      qc.invalidateQueries({ queryKey: ["materials"] });
    },
  });
  const delMut = useMutation({
    mutationFn: () => deleteMaterial(m.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["materials"] }),
  });

  const commit = () => {
    const t = title.trim();
    if (!t || t === m.title) {
      setEditing(false);
      setTitle(m.title);
      return;
    }
    renameMut.mutate(t);
  };

  return (
    <div
      className="card material-row"
      onClick={() => {
        if (!editing) onOpen();
      }}
    >
      <div className="item-meta">
        <span className={`badge ${m.kind}`}>{m.kind}</span>
        {new Date(m.created_at * 1000).toLocaleString()}
        {m.kind === "pdf" && ` · ${m.page_count} pages`}
        <button
          className="row-action"
          onClick={(e) => {
            e.stopPropagation();
            if (window.confirm(`Delete "${m.title}"?`)) delMut.mutate();
          }}
          disabled={delMut.isPending}
          style={{ marginLeft: "auto" }}
        >
          {delMut.isPending ? "…" : "delete"}
        </button>
      </div>
      {editing ? (
        <input
          autoFocus
          className="rename-input"
          value={title}
          onClick={(e) => e.stopPropagation()}
          onChange={(e) => setTitle(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === "Enter") commit();
            if (e.key === "Escape") {
              setEditing(false);
              setTitle(m.title);
            }
          }}
        />
      ) : (
        <div
          className="item-title"
          onClick={(e) => {
            e.stopPropagation();
            setEditing(true);
          }}
          title="Click to rename"
        >
          {m.title}
        </div>
      )}
      {m.source && <div className="item-source">{m.source}</div>}
    </div>
  );
}
