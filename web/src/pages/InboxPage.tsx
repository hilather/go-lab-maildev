import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { APIError, clearMessages, listMessages } from "../api/client";
import type { Message } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE, formatAddress } from "../auth/scopes";
import { useInboxLive } from "../hooks/useInboxLive";

export function InboxPage() {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const [items, setItems] = useState<Message[]>([]);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");
  const [generation, setGeneration] = useState<number | null>(null);

  const refresh = useCallback(() => {
    void (async () => {
      try {
        const list = await listMessages(filter === "" ? {} : { subjectContains: filter });
        setItems(list.items);
        setGeneration(list.storeGeneration);
        setError("");
      } catch (err) {
        setError(err instanceof APIError ? err.message : "Could not load inbox.");
      }
    })();
  }, [filter]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const mode = useInboxLive(refresh, true);

  async function onClear() {
    if (!window.confirm("Delete every captured message?")) {
      return;
    }
    try {
      await clearMessages();
      refresh();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Clear failed.");
    }
  }

  return (
    <main className="page">
      <h1>Inbox</h1>
      <p className="muted">
        Live update: {mode === "sse" ? "event stream" : mode === "poll" ? "3s poll fallback" : "connecting…"}.
        {generation !== null ? ` Store generation ${generation}.` : ""}
      </p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <form
        className="row"
        onSubmit={(ev) => {
          ev.preventDefault();
          const fd = new FormData(ev.currentTarget);
          setFilter(String(fd.get("q") ?? "").trim());
        }}
      >
        <div className="field">
          <label htmlFor="inbox-filter">Subject contains</label>
          <input id="inbox-filter" name="q" defaultValue={filter} />
        </div>
        <button type="submit">Filter</button>
        {canWrite ? (
          <button type="button" onClick={() => void onClear()}>
            Clear inbox
          </button>
        ) : null}
      </form>
      {items.length === 0 ? (
        <p>No messages.</p>
      ) : (
        <ul className="mail-list">
          {items.map((m) => (
            <li key={m.id}>
              <Link to={`/messages/${encodeURIComponent(m.id)}`}>
                <span className="unread-dot" data-read={m.read ? "true" : "false"} aria-hidden="true" />
                <span>
                  <span className="subject">{m.subject || "(no subject)"}</span>
                  <span className="muted">
                    {" "}
                    {m.from.map((a) => formatAddress(a.name, a.address)).join(", ") || m.envelope.from}
                  </span>
                </span>
                <time dateTime={m.receivedAt}>{m.receivedAt}</time>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
