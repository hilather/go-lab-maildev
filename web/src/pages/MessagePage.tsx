import { useEffect, useRef, useState } from "react";
import {
  APIError,
  attachmentURL,
  deleteMessage,
  getMessage,
  getMessageRaw,
  markMessageRead,
  previewURL,
} from "../api/client";
import type { Message } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE, formatAddress, formatBytes } from "../auth/scopes";
import { ConfirmBar } from "../ui/ConfirmBar";
import { formatRelativeReceived } from "../ui/relativeTime";
import { PREVIEW_SANDBOX } from "../ui/sandbox";

type Tab = "html" | "text" | "raw" | "headers";

type MessagePageProps = {
  messageId?: string;
  embedded?: boolean;
  onDeleted?: () => void;
  onBecameRead?: () => void;
};

export function MessagePage({ messageId = "", embedded = false, onDeleted, onBecameRead }: MessagePageProps) {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [tab, setTab] = useState<Tab>("html");
  const [msg, setMsg] = useState<Message | null>(null);
  const [raw, setRaw] = useState("");
  const [error, setError] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const id = messageId;
  const becameRead = useRef(onBecameRead);
  becameRead.current = onBecameRead;

  useEffect(() => {
    let cancelled = false;
    setMsg(null);
    setRaw("");
    setError("");
    setConfirmDelete(false);
    void (async () => {
      try {
        const next = await getMessage(id);
        if (cancelled) {
          return;
        }
        setMsg(next);
        setTab(next.hasHTML ? "html" : "text");
        setError("");
        if (!next.read && canWrite) {
          try {
            await markMessageRead(id);
            if (!cancelled) {
              setMsg({ ...next, read: true });
              becameRead.current?.();
            }
          } catch {
            // Inspector still shows the GET body; unread badge stays until retry.
          }
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Message not found.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [id, canWrite]);

  useEffect(() => {
    if (tab !== "raw" || id === "") {
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        const body = await getMessageRaw(id);
        if (!cancelled) {
          setRaw(body);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof APIError ? err.message : "Could not load raw message.");
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [tab, id]);

  async function onDelete() {
    try {
      await deleteMessage(id);
      onDeleted?.();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Delete failed.");
    }
  }

  if (error !== "" && msg === null) {
    return (
      <div className={embedded ? "pane" : "page"}>
        <p className="banner-error" role="alert">
          {error}
        </p>
      </div>
    );
  }
  if (msg === null) {
    return (
      <div className={embedded ? "pane" : "page"}>
        <p role="status">Loading message…</p>
      </div>
    );
  }

  const tabs: { id: Tab; label: string }[] = [
    { id: "html", label: "HTML" },
    { id: "text", label: "Text" },
    { id: "raw", label: "Raw" },
    { id: "headers", label: "Headers" },
  ];
  const from = msg.from.map((a) => formatAddress(a.name, a.address)).join(", ") || msg.envelope.from;
  const to = msg.to.map((a) => formatAddress(a.name, a.address)).join(", ") || msg.envelope.to.join(", ");

  return (
    <article className={embedded ? "pane" : "page"}>
      <header className="pane__head">
        <div>
          <h1 className="pane__subject">{msg.subject || "(no subject)"}</h1>
          <p className="pane__meta">
            {from} → {to} · {formatRelativeReceived(msg.receivedAt)}
          </p>
        </div>
        {canWrite ? (
          confirmDelete ? (
            <ConfirmBar
              title="Delete this message?"
              confirmLabel="Delete"
              danger
              onConfirm={() => void onDelete()}
              onCancel={() => setConfirmDelete(false)}
            />
          ) : (
            <button type="button" className="btn-danger" onClick={() => setConfirmDelete(true)}>
              Delete
            </button>
          )
        ) : null}
      </header>
      {msg.parseWarning ? <p className="banner-error">{msg.parseWarning}</p> : null}
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="tabs" role="tablist" aria-label="Message parts">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>
      {tab === "text" ? <pre className="raw">{msg.text || "(no text body)"}</pre> : null}
      {tab === "html" ? (
        <>
          <div className="preview-card">
            <iframe
              className="preview-frame"
              title="HTML preview"
              src={previewURL(msg.id)}
              sandbox={PREVIEW_SANDBOX}
              referrerPolicy="no-referrer"
            />
          </div>
          <p className="sandbox-note">sandbox empty · img-src data: only · no remote pixels</p>
        </>
      ) : null}
      {tab === "headers" ? (
        <table className="data">
          <thead>
            <tr>
              <th>Name</th>
              <th>Value</th>
            </tr>
          </thead>
          <tbody>
            {(msg.headers ?? []).map((h, i) => (
              <tr key={`${h.name}-${i}`}>
                <td>{h.name}</td>
                <td>{h.value}</td>
              </tr>
            ))}
          </tbody>
        </table>
      ) : null}
      {tab === "raw" ? <pre className="raw">{raw || "(loading raw…)"}</pre> : null}
      {msg.attachments.length > 0 ? (
        <ul className="attach-list">
          {msg.attachments.map((a) => (
            <li key={a.id}>
              <a href={attachmentURL(msg.id, a.id)}>{a.filename || a.id}</a>{" "}
              <span className="muted">
                {a.contentType} · {formatBytes(a.size)}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </article>
  );
}
