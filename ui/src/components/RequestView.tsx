import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Material, RequestRecord } from "../lib/types";
import { listMaterials } from "../lib/api";
import { summarizeEvent } from "../lib/trace";
import { MaterialViewer } from "./MaterialViewer";
import { Markdown } from "./Markdown";

export function RequestView({ req }: { req: RequestRecord }) {
  const [showTraceWhenReady, setShowTraceWhenReady] = useState(false);
  const [openMat, setOpenMat] = useState<{ m: Material; page?: number } | null>(null);
  const mats = useQuery({ queryKey: ["materials"], queryFn: listMaterials });
  const events = req.events ?? [];
  const inFlight = req.status === "pending" || req.status === "running";
  const showTrace = inFlight || showTraceWhenReady;

  return (
    <>
      {events.length > 0 && (
        <div className="card">
          <div className="trace-header">
            <span className="why">
              {inFlight ? `Working… (${events.length} steps)` : `${events.length} steps`}
              {inFlight && <span className="dot-pulse"> ●</span>}
            </span>
            {!inFlight && (
              <button
                className="row-action"
                onClick={() => setShowTraceWhenReady((v) => !v)}
              >
                {showTraceWhenReady ? "hide trace" : "show trace"}
              </button>
            )}
          </div>
          {showTrace && (
            <ol className="trace-list">
              {events.map((ev) => (
                <li key={ev.id} className={`trace-item kind-${ev.kind}`}>
                  <span className="trace-seq">{String(ev.seq).padStart(2, "0")}</span>{" "}
                  <span className="trace-summary">{summarizeEvent(ev)}</span>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}

      {req.status === "error" && (
        <div className="card">
          <div className="why err">Error: {req.error}</div>
        </div>
      )}

      {req.status === "ready" && req.response && (
        <>
          <div className="card">
            <h2>Answer</h2>
            <div className="answer"><Markdown content={req.response.answer} /></div>
          </div>
          <div className="card used">
            <h2>Used materials ({(req.response.used_items ?? []).length})</h2>
            {(req.response.used_items ?? []).length === 0 ? (
              <div className="why">none</div>
            ) : (
              (req.response.used_items ?? []).map((u, i) => {
                const m = mats.data?.find((x) => x.id === u.material_id);
                const label = `${u.title || u.material_id}${u.page ? ` (p. ${u.page})` : ""}`;
                return (
                  <div key={i} style={{ marginBottom: 8 }}>
                    {m ? (
                      <button
                        className="row-action"
                        style={{ font: "inherit", fontWeight: 600, padding: 0 }}
                        onClick={() => setOpenMat({ m, page: u.page })}
                      >
                        {label}
                      </button>
                    ) : (
                      <b>{label}</b>
                    )}
                    <div className="why">{u.why_relevant}</div>
                  </div>
                );
              })
            )}
          </div>
          <div className="card missing">
            <h2>Missing context</h2>
            {(req.response.missing_context ?? []).length === 0 ? (
              <div className="why">none reported</div>
            ) : (
              (req.response.missing_context ?? []).map((mc, i) => (
                <div key={i} style={{ marginBottom: 8 }}>
                  <b>{mc.what}</b>
                  <div className="why">→ {mc.suggestion}</div>
                </div>
              ))
            )}
          </div>
          <div className="card next">
            <h2>Next actions</h2>
            <ol style={{ margin: 0, paddingLeft: 20 }}>
              {(req.response.next_actions ?? []).map((a, i) => (
                <li key={i}>{a}</li>
              ))}
            </ol>
          </div>
        </>
      )}
      {openMat && <MaterialViewer material={openMat.m} initialPage={openMat.page} onClose={() => setOpenMat(null)} />}
    </>
  );
}
