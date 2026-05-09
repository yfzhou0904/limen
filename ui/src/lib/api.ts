import type { Material, RequestRecord } from "./types";

async function ok<T>(res: Response): Promise<T> {
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  return res.json();
}

export async function listMaterials(): Promise<Material[]> {
  return ok(await fetch("/api/materials"));
}

export async function uploadFile(file: File): Promise<Material> {
  const fd = new FormData();
  fd.append("file", file);
  return ok(await fetch("/api/materials", { method: "POST", body: fd }));
}

export async function uploadURL(url: string): Promise<Material> {
  return ok(
    await fetch("/api/materials/url", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ url }),
    })
  );
}

export async function createNote(title: string, content: string): Promise<Material> {
  return ok(
    await fetch("/api/materials/note", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ title, content }),
    })
  );
}

export async function renameMaterial(id: string, title: string): Promise<Material> {
  return ok(
    await fetch(`/api/materials/${id}`, {
      method: "PATCH",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ title }),
    })
  );
}

export async function deleteMaterial(id: string): Promise<void> {
  const res = await fetch(`/api/materials/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
}

export async function startAsk(question: string): Promise<{ id: string }> {
  return ok(
    await fetch("/api/ask", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ question }),
    })
  );
}

export async function getRequest(id: string): Promise<RequestRecord> {
  return ok(await fetch(`/api/requests/${id}`));
}

export async function listRequests(): Promise<RequestRecord[]> {
  return ok(await fetch("/api/requests"));
}

export async function deleteRequest(id: string): Promise<void> {
  const res = await fetch(`/api/requests/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
}
