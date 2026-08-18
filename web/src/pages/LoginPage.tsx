import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { APIError, basicAuthorization, bearerAuthorization } from "../api/client";
import { useAuth } from "../auth/AuthProvider";

type Mode = "token" | "basic";

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [mode, setMode] = useState<Mode>("token");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onSubmit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault();
    const fd = new FormData(ev.currentTarget);
    let authorization = "";
    if (mode === "token") {
      const token = String(fd.get("token") ?? "").trim();
      if (token === "") {
        setError("Enter an API bearer token.");
        return;
      }
      authorization = bearerAuthorization(token);
    } else {
      const username = String(fd.get("username") ?? "").trim();
      const password = String(fd.get("password") ?? "");
      if (username === "" || password === "") {
        setError("Enter a username and password.");
        return;
      }
      authorization = basicAuthorization(username, password);
    }
    setError("");
    setBusy(true);
    const form = ev.currentTarget;
    try {
      await login(authorization);
      form.reset();
      void navigate("/", { replace: true });
    } catch (err) {
      const detail =
        err instanceof APIError
          ? err.problem.detail || "Sign-in failed."
          : err instanceof Error
            ? err.message
            : "Sign-in failed.";
      setError(detail);
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="page page--narrow">
      <h1>Sign in to LabMail</h1>
      <p>
        Exchange a scoped API token or Basic password for an HttpOnly session cookie. Credentials
        are not written to web storage.
      </p>
      {error !== "" ? (
        <p className="banner-error" role="alert">
          {error}
        </p>
      ) : null}
      <form className="stack" onSubmit={(e) => void onSubmit(e)} noValidate>
        <fieldset className="stack">
          <legend>Sign in with</legend>
          <label>
            <input
              type="radio"
              name="mode"
              value="token"
              checked={mode === "token"}
              onChange={() => setMode("token")}
            />{" "}
            Bearer token
          </label>
          <label>
            <input
              type="radio"
              name="mode"
              value="basic"
              checked={mode === "basic"}
              onChange={() => setMode("basic")}
            />{" "}
            Username and password
          </label>
        </fieldset>
        {mode === "token" ? (
          <div className="field">
            <label htmlFor="login-token">API bearer token</label>
            <input
              id="login-token"
              name="token"
              type="password"
              autoComplete="off"
              autoCapitalize="off"
              spellCheck={false}
              required
            />
          </div>
        ) : (
          <>
            <div className="field">
              <label htmlFor="login-user">Username</label>
              <input id="login-user" name="username" autoComplete="username" required />
            </div>
            <div className="field">
              <label htmlFor="login-pass">Password</label>
              <input id="login-pass" name="password" type="password" autoComplete="current-password" required />
            </div>
          </>
        )}
        <button type="submit" disabled={busy}>
          {busy ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </main>
  );
}
