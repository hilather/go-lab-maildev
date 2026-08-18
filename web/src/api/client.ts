import { assertNoTokenStorage } from "./storage";
import type { AuditList, Message, MessageList, Problem, SessionCreated, SessionView, Status } from "./types";

export const CSRF_HEADER = "X-LabMail-CSRF";

export class APIError extends Error {
  readonly problem: Problem;

  constructor(problem: Problem) {
    super(problem.detail || problem.title || "request failed");
    this.name = "APIError";
    this.problem = problem;
  }
}

let memoryCSRF = "";

export function setMemoryCSRF(value: string): void {
  memoryCSRF = value;
}

export function getMemoryCSRF(): string {
  return memoryCSRF;
}

export function clearMemoryCSRF(): void {
  memoryCSRF = "";
}

function problemFrom(status: number, statusText: string, body: unknown): Problem {
  const fallback: Problem = {
    type: "urn:labmail:error:internal-error",
    title: statusText || "error",
    status,
    detail: statusText || "request failed",
    code: status === 401 ? "unauthenticated" : status === 403 ? "forbidden" : "internal_error",
  };
  if (!body || typeof body !== "object") {
    return fallback;
  }
  const rec = body as Record<string, unknown>;
  return {
    type: typeof rec.type === "string" ? rec.type : fallback.type,
    title: typeof rec.title === "string" ? rec.title : fallback.title,
    status: typeof rec.status === "number" ? rec.status : fallback.status,
    detail: typeof rec.detail === "string" ? rec.detail : fallback.detail,
    code: typeof rec.code === "string" ? rec.code : fallback.code,
  };
}

export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  assertNoTokenStorage();
  const headers = new Headers(init.headers);
  const method = (init.method ?? "GET").toUpperCase();
  if (method !== "GET" && method !== "HEAD" && !headers.has(CSRF_HEADER)) {
    const csrf = getMemoryCSRF();
    if (csrf !== "") {
      headers.set(CSRF_HEADER, csrf);
    }
  }
  if (!headers.has("Accept")) {
    headers.set("Accept", "application/json");
  }
  return fetch(path, {
    ...init,
    credentials: "same-origin",
    headers,
  });
}

async function readJSON<T>(resp: Response): Promise<T> {
  const text = await resp.text();
  let parsed: unknown;
  if (text !== "") {
    try {
      parsed = JSON.parse(text) as unknown;
    } catch {
      parsed = undefined;
    }
  }
  if (!resp.ok) {
    throw new APIError(problemFrom(resp.status, resp.statusText, parsed));
  }
  return parsed as T;
}

export async function createSession(authorization: string): Promise<SessionCreated> {
  const resp = await apiFetch("/v1/session", {
    method: "POST",
    headers: { Authorization: authorization },
  });
  const created = await readJSON<SessionCreated>(resp);
  setMemoryCSRF(created.csrf);
  assertNoTokenStorage();
  return created;
}

export function bearerAuthorization(token: string): string {
  return `Bearer ${token}`;
}

export function basicAuthorization(username: string, password: string): string {
  return `Basic ${btoa(`${username}:${password}`)}`;
}

export async function getSession(): Promise<SessionView> {
  const view = await readJSON<SessionView>(await apiFetch("/v1/session"));
  if (typeof view.csrf === "string" && view.csrf !== "") {
    setMemoryCSRF(view.csrf);
  }
  return view;
}

export async function deleteSession(): Promise<void> {
  const resp = await apiFetch("/v1/session", { method: "DELETE" });
  if (resp.status === 401 || resp.status === 204) {
    clearMemoryCSRF();
    return;
  }
  await readJSON<unknown>(resp);
  clearMemoryCSRF();
}

export async function listMessages(query: { subjectContains?: string } = {}): Promise<MessageList> {
  const params = new URLSearchParams();
  if (query.subjectContains) {
    params.set("subjectContains", query.subjectContains);
  }
  const qs = params.toString();
  return readJSON<MessageList>(await apiFetch(qs === "" ? "/v1/messages" : `/v1/messages?${qs}`));
}

export async function getMessage(id: string, markRead = false): Promise<Message> {
  const qs = markRead ? "?markRead=true" : "";
  return readJSON<Message>(await apiFetch(`/v1/messages/${encodeURIComponent(id)}${qs}`));
}

export async function getMessageRaw(id: string): Promise<string> {
  const resp = await apiFetch(`/v1/messages/${encodeURIComponent(id)}/raw`, {
    headers: { Accept: "message/rfc822" },
  });
  if (!resp.ok) {
    const text = await resp.text();
    let parsed: unknown;
    try {
      parsed = JSON.parse(text) as unknown;
    } catch {
      parsed = undefined;
    }
    throw new APIError(problemFrom(resp.status, resp.statusText, parsed));
  }
  return resp.text();
}

export function previewURL(id: string): string {
  return `/v1/messages/${encodeURIComponent(id)}/preview`;
}

export function attachmentURL(id: string, attId: string): string {
  return `/v1/messages/${encodeURIComponent(id)}/attachments/${encodeURIComponent(attId)}`;
}

export async function deleteMessage(id: string): Promise<void> {
  const resp = await apiFetch(`/v1/messages/${encodeURIComponent(id)}`, { method: "DELETE" });
  if (resp.status !== 204) {
    await readJSON<unknown>(resp);
  }
}

export async function clearMessages(): Promise<{ deleted: number }> {
  return readJSON<{ deleted: number }>(await apiFetch("/v1/messages", { method: "DELETE" }));
}

export async function getStatus(): Promise<Status> {
  return readJSON<Status>(await apiFetch("/v1/status"));
}

export async function listAudit(): Promise<AuditList> {
  return readJSON<AuditList>(await apiFetch("/v1/audit"));
}

export async function resetState(reason: string): Promise<unknown> {
  return readJSON<unknown>(
    await apiFetch("/v1/state:reset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ reason }),
    }),
  );
}
