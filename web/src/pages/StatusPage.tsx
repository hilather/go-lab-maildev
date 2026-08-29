import { useEffect, useState } from "react";
import { APIError, getStatus } from "../api/client";
import type { Status } from "../api/types";
import { formatBytes } from "../auth/scopes";

export function StatusPage() {
  const [status, setStatus] = useState<Status | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        setStatus(await getStatus());
      } catch (err) {
        setError(err instanceof APIError ? err.message : "Could not load status.");
      }
    })();
  }, []);

  if (error !== "") {
    return (
      <main className="page">
        <h1 className="page-title">Status</h1>
        <p className="banner-error" role="alert">
          {error}
        </p>
      </main>
    );
  }
  if (status === null) {
    return (
      <main className="page">
        <h1 className="page-title">Status</h1>
        <p className="empty-state" role="status">
          Loading status…
        </p>
      </main>
    );
  }

  return (
    <main className="page">
      <h1 className="page-title">Status</h1>
      <p className="muted">
        Ready: <strong>{status.ready ? "yes" : "no"}</strong>
      </p>
      <section className="panel">
        <h2 className="section-label">Store</h2>
        <dl className="kv">
          <div>
            <dt>Messages</dt>
            <dd>{status.store.messageCount}</dd>
          </div>
          <div>
            <dt>Unread</dt>
            <dd>{status.store.unreadCount}</dd>
          </div>
          <div>
            <dt>Bytes</dt>
            <dd>{formatBytes(status.store.storeBytes)}</dd>
          </div>
          <div>
            <dt>Generation</dt>
            <dd>{status.store.storeGeneration}</dd>
          </div>
          <div>
            <dt>Epoch</dt>
            <dd>{status.store.epoch}</dd>
          </div>
        </dl>
      </section>
      <section className="panel">
        <h2 className="section-label">Listeners</h2>
        {status.listeners.length === 0 ? (
          <p className="empty-state">No listeners reported.</p>
        ) : (
          <ul>
            {status.listeners.map((l) => (
              <li key={l.name}>
                {l.name}: <code>{l.address}</code>
              </li>
            ))}
          </ul>
        )}
      </section>
      <section className="panel">
        <h2 className="section-label">Revisions</h2>
        <pre className="raw">{JSON.stringify(status.revisions, null, 2)}</pre>
      </section>
    </main>
  );
}
