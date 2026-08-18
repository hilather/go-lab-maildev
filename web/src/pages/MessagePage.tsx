import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  APIError,
  attachmentURL,
  deleteMessage,
  getMessage,
  getMessageRaw,
  previewURL,
} from "../api/client";
import type { Message } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE, formatAddress, formatBytes } from "../auth/scopes";
import { PREVIEW_SANDBOX } from "../ui/sandbox";

type Tab = "text" | "html" | "headers" | "raw" | "attachments";

export function MessagePage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [tab, setTab] = useState<Tab>("text");
  const [msg, setMsg] = useState<Message | null>(null);
  const [raw, setRaw] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const next = await getMessage(id, canWrite);
        if (!cancelled) {
          setMsg(next);
          setError("");
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
    if (!window.confirm("Delete this message?")) {
      return;
    }
    try {
      await deleteMessage(id);
      void navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Delete failed.");
    }
  }

  if (error !== "" && msg === null) {
    return (
      <main className="page">
        <p className="banner-error" role="alert">
          {error}
        </p>
        <p>
          <Link to="/">Back to inbox</Link>
        </p>
      </main>
    );
  }
  if (msg === null) {
    return (
      <main className="page">
        <p role="status">Loading message…</p>
      </main>
    );
  }

  const tabs: { id: Tab; label: string }[] = [
    { id: "text", label: "Text" },
    { id: "html", label: "HTML preview" },
    { id: "headers", label: "Headers" },
    { id: "raw", label: "Raw" },
    { id: "attachments", label: "Attachments" },
  ];

  return (
    <main className="page">
      <p>
        <Link to="/">Inbox</Link>
      </p>
      <h1>{msg.subject || "(no subject)"}</h1>
      <p>
        From {msg.from.map((a) => formatAddress(a.name, a.address)).join(", ") || msg.envelope.from} · To{" "}
        {msg.to.map((a) => formatAddress(a.name, a.address)).join(", ") || msg.envelope.to.join(", ")} ·{" "}
        {formatBytes(msg.size)}
      </p>
      {msg.parseWarning ? <p className="banner-error">{msg.parseWarning}</p> : null}
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {canWrite ? (
        <p>
          <button type="button" onClick={() => void onDelete()}>
            Delete message
          </button>
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
        <iframe
          className="preview-frame"
          title="HTML preview"
          src={previewURL(msg.id)}
          sandbox={PREVIEW_SANDBOX}
          referrerPolicy="no-referrer"
        />
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
      {tab === "attachments" ? (
        msg.attachments.length === 0 ? (
          <p>No attachments.</p>
        ) : (
          <ul>
            {msg.attachments.map((a) => (
              <li key={a.id}>
                <a href={attachmentURL(msg.id, a.id)}>{a.filename || a.id}</a>{" "}
                <span className="muted">
                  {a.contentType} · {formatBytes(a.size)}
                </span>
              </li>
            ))}
          </ul>
        )
      ) : null}
    </main>
  );
}
