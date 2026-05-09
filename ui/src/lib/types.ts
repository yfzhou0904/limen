export type Material = {
  id: string;
  kind: "note" | "webpage" | "pdf";
  title: string;
  source: string;
  ext: "md" | "pdf";
  page_count: number;
  created_at: number;
};

export type UsedItem = { material_id: string; title: string; page?: number; why_relevant: string };
export type MissingItem = { what: string; suggestion: string };
export type AgentResponse = {
  answer: string;
  used_items: UsedItem[];
  next_actions: string[];
  missing_context: MissingItem[];
};

export type RequestEvent = {
  id: number;
  request_id: string;
  seq: number;
  kind: "tool_use" | "tool_result" | "assistant_text" | "final";
  payload: string;
};

export type RequestStatus = "pending" | "running" | "ready" | "error";

export type RequestRecord = {
  id: string;
  prompt: string;
  status: RequestStatus;
  error?: string;
  created_at: number;
  events?: RequestEvent[];
  response?: AgentResponse;
};
