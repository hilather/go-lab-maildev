import { FormEvent, useState } from "react";
import { APIError, resetState } from "../api/client";
import { useAuth } from "../auth/AuthProvider";
import { SCOPE_ADMIN } from "../auth/scopes";
import { RESET_PHRASE, canSubmitReset } from "../ui/forbidden";

export function ResetPage() {
  const { hasScope } = useAuth();
  const allowed = hasScope(SCOPE_ADMIN);
  const [phrase, setPhrase] = useState("");
  const [reason, setReason] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const ok = canSubmitReset(phrase, confirmed, allowed);

  async function onSubmit(ev: FormEvent) {
    ev.preventDefault();
    if (!ok) {
      return;
    }
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await resetState(reason.trim());
      setNotice("Reset completed. Bootstrap was reread and the inbox was wiped.");
      setPhrase("");
      setConfirmed(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Reset failed.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page page--narrow">
      <h1>Reset</h1>
      <p>
        Reset rereads the mounted bootstrap YAML and wipes the inbox. It does not send mail. Type{" "}
        <code>{RESET_PHRASE}</code> to enable the control.
      </p>
      {!allowed ? <p>Requires scope mail.admin.</p> : null}
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      {notice !== "" ? (
        <p role="status">{notice}</p>
      ) : null}
      <form className="stack" onSubmit={(e) => void onSubmit(e)}>
        <div className="field">
          <label htmlFor="reset-phrase">Confirmation phrase</label>
          <input
            id="reset-phrase"
            value={phrase}
            autoComplete="off"
            spellCheck={false}
            onChange={(e) => setPhrase(e.target.value)}
          />
        </div>
        <div className="field">
          <label htmlFor="reset-reason">Reason (optional)</label>
          <input id="reset-reason" value={reason} onChange={(e) => setReason(e.target.value)} />
        </div>
        <label>
          <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} /> Wipe
          the inbox and reload bootstrap
        </label>
        <button type="submit" disabled={!ok || busy}>
          {busy ? "Resetting…" : "Reset LabMail"}
        </button>
      </form>
    </main>
  );
}
