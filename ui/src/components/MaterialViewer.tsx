import { useEffect, useState } from "react";
import type { Material } from "../lib/types";
import { Markdown } from "./Markdown";

export function MaterialViewer({
  material,
  onClose,
  initialPage,
}: {
  material: Material;
  onClose: () => void;
  initialPage?: number;
}) {
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div style={{ fontWeight: 600 }}>
              {material.title}
              <span style={{ color: "var(--mute)", fontWeight: 400 }}>
                {" "}.{material.ext}
              </span>
            </div>
            <div style={{ fontSize: 12, color: "var(--mute)" }}>
              <span className={`badge ${material.kind}`}>{material.kind}</span>
              {material.source && (
                <a href={material.kind === "webpage" ? material.source : undefined} target="_blank" rel="noopener" style={{ color: "var(--mute)" }}>
                  {material.source}
                </a>
              )}
            </div>
          </div>
          <button className="ghost" onClick={onClose}>Close</button>
        </div>
        {material.ext === "pdf" ? (
          <PDFPaginator material={material} initialPage={initialPage} />
        ) : (
          <MarkdownView material={material} />
        )}
      </div>
    </div>
  );
}

function PDFPaginator({ material, initialPage }: { material: Material; initialPage?: number }) {
  const [page, setPage] = useState(Math.min(Math.max(1, initialPage ?? 1), material.page_count || 1));
  const [url, setUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let blobUrl: string | null = null;
    setUrl(null);
    setError(null);
    fetch(`/api/materials/${material.id}/pages/${page}.pdf`)
      .then(async (res) => {
        if (!res.ok) throw new Error(`${res.status}`);
        const blob = await res.blob();
        if (cancelled) return;
        blobUrl = URL.createObjectURL(blob);
        setUrl(blobUrl);
      })
      .catch((e) => !cancelled && setError(String(e)));
    return () => {
      cancelled = true;
      if (blobUrl) URL.revokeObjectURL(blobUrl);
    };
  }, [material.id, page]);

  return (
    <>
      <div className="pager">
        <button className="ghost" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page <= 1}>←</button>
        <span>page {page} / {material.page_count}</span>
        <button className="ghost" onClick={() => setPage((p) => Math.min(material.page_count, p + 1))} disabled={page >= material.page_count}>→</button>
      </div>
      <div className="modal-body pdf">
        {error && <p className="err" style={{ padding: 16 }}>{error}</p>}
        {!error && url && <iframe title={`page-${page}`} src={url} style={{ width: "100%", height: "100%", border: 0 }} />}
      </div>
    </>
  );
}

function MarkdownView({ material }: { material: Material }) {
  const [text, setText] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let cancelled = false;
    fetch(`/api/materials/${material.id}/content`)
      .then(async (res) => {
        if (!res.ok) throw new Error(`${res.status}`);
        const t = await res.text();
        if (!cancelled) setText(t);
      })
      .catch((e) => !cancelled && setError(String(e)));
    return () => { cancelled = true; };
  }, [material.id]);
  return (
    <div className="modal-body">
      {error && <p className="err">{error}</p>}
      {!error && <Markdown content={text ?? ""} />}
    </div>
  );
}
