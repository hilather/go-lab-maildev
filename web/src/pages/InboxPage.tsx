import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { APIError, clearMessages, listAllMessages } from "../api/client";
import type { Message } from "../api/types";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_WRITE, senderDisplay } from "../auth/scopes";
import { useLive } from "../hooks/LiveProvider";
import { ConfirmBar } from "../ui/ConfirmBar";
import { formatRelativeReceived } from "../ui/relativeTime";
import { MessagePage } from "./MessagePage";

function matchesFilter(m: Message, q: string): boolean {
  if (q === "") {
    return true;
  }
  const needle = q.toLowerCase();
  if (m.subject.toLowerCase().includes(needle)) {
    return true;
  }
  if (m.envelope.from.toLowerCase().includes(needle)) {
    return true;
  }
  return m.from.some((a) => a.name.toLowerCase().includes(needle) || a.address.toLowerCase().includes(needle));
}

export function InboxPage() {
  const { hasScope } = useAuth();
  const canWrite = hasScope(SCOPE_WRITE);
  const { subscribeRefresh, decrementUnread } = useLive();
  const [params, setParams] = useSearchParams();
  const selectedId = params.get("id") ?? "";
  const [items, setItems] = useState<Message[]>([]);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");
  const [confirmClear, setConfirmClear] = useState(false);
  const refreshSeq = useRef(0);

  const refresh = useCallback(() => {
    const seq = ++refreshSeq.current;
    void (async () => {
      try {
        const list = await listAllMessages();
        if (seq !== refreshSeq.current) {
          return;
        }
        setItems(list.items);
        setError("");
      } catch (err) {
        if (seq !== refreshSeq.current) {
          return;
        }
        setError(err instanceof APIError ? err.message : "Could not load inbox.");
      }
    })();
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => subscribeRefresh(refresh), [subscribeRefresh, refresh]);

  const visible = useMemo(() => items.filter((m) => matchesFilter(m, filter)), [items, filter]);

  function select(id: string) {
    const next = new URLSearchParams(params);
    next.set("id", id);
    setParams(next, { replace: true });
  }

  function clearSelection() {
    const next = new URLSearchParams(params);
    next.delete("id");
    setParams(next, { replace: true });
  }

  async function onClear() {
    try {
      await clearMessages();
      setConfirmClear(false);
      clearSelection();
      refresh();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Clear failed.");
    }
  }

  function onBecameRead(id: string) {
    setItems((cur) => cur.map((m) => (m.id === id ? { ...m, read: true } : m)));
    decrementUnread();
  }

  return (
    <div className="inbox">
      <section className="captured" aria-label="Captured messages">
        <div className="captured__head">
          <h1 className="captured__title">Captured</h1>
        </div>
        <label className="sr-only" htmlFor="inbox-filter">
          Filter subject or from
        </label>
        <input
          id="inbox-filter"
          className="captured__search"
          placeholder="Filter subject or from"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        {error !== "" ? (
          <p className="banner-error" role="alert">
            {error}
          </p>
        ) : null}
        {canWrite ? (
          confirmClear ? (
            <ConfirmBar
              title="Delete every captured message?"
              confirmLabel="Clear inbox"
              danger
              onConfirm={() => void onClear()}
              onCancel={() => setConfirmClear(false)}
            />
          ) : (
            <div className="captured__tools">
              <button type="button" className="btn-ghost" onClick={() => setConfirmClear(true)}>
                Clear inbox
              </button>
            </div>
          )
        ) : null}
        {visible.length === 0 ? (
          <p className="captured__empty">No messages.</p>
        ) : (
          <ul className="mail-list">
            {visible.map((m) => {
              const from = m.from[0];
              const sender = from ? senderDisplay(from.name, from.address) : senderDisplay("", m.envelope.from);
              const selected = m.id === selectedId;
              const att = m.attachments.length;
              return (
                <li key={m.id}>
                  <button
                    type="button"
                    className={selected ? "mail-row mail-row--selected" : "mail-row"}
                    data-read={m.read ? "true" : "false"}
                    onClick={() => select(m.id)}
                  >
                    <span className="unread-dot" data-read={m.read ? "true" : "false"} aria-hidden="true" />
                    <span className="mail-row__body">
                      <span className="mail-row__top">
                        <span className="mail-row__from">{sender}</span>
                        <time dateTime={m.receivedAt}>{formatRelativeReceived(m.receivedAt)}</time>
                      </span>
                      <span className="mail-row__subject">{m.subject || "(no subject)"}</span>
                      {att > 0 ? (
                        <span className="mail-row__hint">
                          {att} attachment{att === 1 ? "" : "s"}
                        </span>
                      ) : null}
                    </span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </section>
      <section className="inspector" aria-label="Message">
        {selectedId === "" ? (
          <p className="inspector__empty">Select a captured message.</p>
        ) : (
          <MessagePage
            messageId={selectedId}
            embedded
            onDeleted={clearSelection}
            onBecameRead={() => onBecameRead(selectedId)}
          />
        )}
      </section>
    </div>
  );
}
