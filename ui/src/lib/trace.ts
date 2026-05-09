import type { RequestEvent } from "./types";

export function summarizeEvent(ev: RequestEvent): string {
  let p: Record<string, unknown> = {};
  try {
    p = JSON.parse(ev.payload) as Record<string, unknown>;
  } catch {
    /* keep empty */
  }
  switch (ev.kind) {
    case "tool_use": {
      const name = String(p.name ?? "?");
      const input = (p.input ?? {}) as Record<string, unknown>;
      const path = typeof input.path === "string" ? input.path : "";
      if (name === "submit_response") return "→ submit_response";
      return path ? `→ ${name}(${path})` : `→ ${name}`;
    }
    case "tool_result": {
      if (typeof p.error === "string") return `← error: ${p.error}`;
      if (typeof p.bytes === "number" && typeof p.path === "string") {
        return `← ${p.path} (${p.bytes}B)`;
      }
      if (Array.isArray(p.entries)) return `← ${p.entries.length} entries`;
      if (p.ok === true) return "← ok";
      return "← (result)";
    }
    case "assistant_text": {
      const text = typeof p.text === "string" ? p.text : "";
      return text.length > 200 ? `${text.slice(0, 200)}…` : text;
    }
    case "final": {
      return p.synthesized
        ? `final (synthesized: ${String(p.reason ?? "")})`
        : "final";
    }
  }
  return ev.kind;
}
