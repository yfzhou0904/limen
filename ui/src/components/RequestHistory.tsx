import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { deleteRequest, getRequest, listRequests } from "../lib/api";
import { RequestView } from "./RequestView";

export function RequestHistory() {
  const { data, isLoading } = useQuery({ queryKey: ["requests"], queryFn: listRequests });
  const [openId, setOpenId] = useState<string | null>(null);
  const qc = useQueryClient();

  const detail = useQuery({
    queryKey: ["request", openId],
    queryFn: () => getRequest(openId!),
    enabled: !!openId,
  });

  const del = useMutation({
    mutationFn: (id: string) => deleteRequest(id),
    onSuccess: (_, id) => {
      if (openId === id) setOpenId(null);
      qc.invalidateQueries({ queryKey: ["requests"] });
    },
  });

  if (isLoading) return null;
  if (!data || data.length === 0) return null;

  return (
    <>
      <h2>History</h2>
      {data.map((r) => {
        const isOpen = openId === r.id;
        return (
          <div key={r.id} className="card history-row">
            <div
              className="history-head"
              onClick={() => setOpenId(isOpen ? null : r.id)}
            >
              <div className="item-meta">
                <span className={`badge status-${r.status}`}>{r.status}</span>
                {new Date(r.created_at * 1000).toLocaleString()}
                <button
                  className="row-action"
                  onClick={(e) => {
                    e.stopPropagation();
                    if (window.confirm("Delete this request?")) del.mutate(r.id);
                  }}
                  disabled={del.isPending}
                  style={{ marginLeft: "auto" }}
                >
                  delete
                </button>
              </div>
              <div className="item-title">{r.prompt}</div>
            </div>
            {isOpen && detail.data && <RequestView req={detail.data} />}
          </div>
        );
      })}
    </>
  );
}
