import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getRequest, startAsk } from "../lib/api";
import { RequestView } from "./RequestView";

export function AskPanel() {
  const [q, setQ] = useState("");
  const [activeId, setActiveId] = useState<string | null>(null);
  const qc = useQueryClient();

  const start = useMutation({
    mutationFn: () => startAsk(q),
    onSuccess: (data) => {
      setActiveId(data.id);
      setQ("");
      qc.invalidateQueries({ queryKey: ["requests"] });
    },
  });

  const reqQ = useQuery({
    queryKey: ["request", activeId],
    queryFn: () => getRequest(activeId!),
    enabled: !!activeId,
    refetchInterval: (query) => {
      const s = query.state.data?.status;
      return s === "pending" || s === "running" ? 1000 : false;
    },
  });

  const status = reqQ.data?.status;
  useEffect(() => {
    if (status === "ready" || status === "error") {
      qc.invalidateQueries({ queryKey: ["requests"] });
    }
  }, [status, qc]);

  return (
    <>
      <div className="card">
        <div className="row">
          <textarea
            placeholder="e.g. Summarize what I currently know about X, what's missing, and propose next steps."
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
        <button
          onClick={() => start.mutate()}
          disabled={!q.trim() || start.isPending}
        >
          {start.isPending ? "Submitting…" : "Ask workspace"}
        </button>
        {start.isError && (
          <span className="why err"> {(start.error as Error).message}</span>
        )}
      </div>

      {reqQ.data && <RequestView req={reqQ.data} />}
    </>
  );
}
