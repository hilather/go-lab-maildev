import { type ReactNode } from "react";
import { BrowserRouter, NavLink, Navigate, Outlet, Route, Routes, useParams } from "react-router-dom";
import { AuthProvider, useAuth } from "./auth/AuthProvider";
import { SCOPE_ADMIN, SCOPE_AUDIT } from "./auth/scopes";
import { LiveProvider, useLive } from "./hooks/LiveProvider";
import { AuditPage } from "./pages/AuditPage";
import { InboxPage } from "./pages/InboxPage";
import { LoginPage } from "./pages/LoginPage";
import { ResetPage } from "./pages/ResetPage";
import { StatusPage } from "./pages/StatusPage";
import { navItems } from "./ui/forbidden";

function SkipLink() {
  return (
    <a className="skip-link" href="#app-main">
      Skip to main content
    </a>
  );
}

function NavItem({ to, children, badge }: { to: string; children: ReactNode; badge?: number | undefined }) {
  return (
    <NavLink to={to} className={({ isActive }) => (isActive ? "rail__link rail__link--active" : "rail__link")} end={to === "/"}>
      <span>{children}</span>
      {badge !== undefined && badge > 0 ? <span className="rail__badge">{badge}</span> : null}
    </NavLink>
  );
}

function Masthead({ signedIn, onSignOut }: { signedIn: boolean; onSignOut: () => void }) {
  return (
    <header className="masthead">
      <NavLink className="brand" to="/">
        {signedIn ? <span className="brand__dot" aria-hidden="true" /> : null}
        LabMail
      </NavLink>
      {signedIn ? (
        <div className="masthead__meta">
          <LiveChips />
          <span className="chip">receive-only</span>
          <button type="button" className="btn-signout" onClick={onSignOut}>
            Sign out
          </button>
        </div>
      ) : null}
    </header>
  );
}

function LiveChips() {
  const { mode } = useLive();
  if (mode === "sse") {
    return <span className="chip chip--live">live</span>;
  }
  if (mode === "poll") {
    return <span className="chip chip--live">poll</span>;
  }
  return <span className="chip">connecting</span>;
}

function Rail() {
  const { hasScope } = useAuth();
  const { unreadCount } = useLive();
  const items = navItems(hasScope(SCOPE_AUDIT), hasScope(SCOPE_ADMIN));
  return (
    <nav className="rail" aria-label="Primary">
      {items.map((item) => (
        <NavItem key={item.to} to={item.to} badge={item.to === "/" ? unreadCount : undefined}>
          {item.label}
        </NavItem>
      ))}
    </nav>
  );
}

export function AppShell() {
  const { state, logout } = useAuth();
  const signedIn = state.status === "signed_in";
  return (
    <div className="app">
      <SkipLink />
      <LiveProvider enabled={signedIn}>
        {signedIn ? (
          <>
            <Masthead signedIn onSignOut={() => void logout()} />
            <div className="workspace">
              <Rail />
              <div id="app-main" className="stage">
                <Outlet />
              </div>
            </div>
          </>
        ) : (
          <>
            <Masthead signedIn={false} onSignOut={() => undefined} />
            <div id="app-main" className="stage stage--solo">
              <Outlet />
            </div>
          </>
        )}
      </LiveProvider>
    </div>
  );
}

function RequireSession() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status !== "signed_in") {
    return <Navigate to="/login" replace />;
  }
  return <Outlet />;
}

function RedirectIfSignedIn() {
  const { state } = useAuth();
  if (state.status === "loading") {
    return (
      <main className="page">
        <p role="status">Checking session…</p>
      </main>
    );
  }
  if (state.status === "signed_in") {
    return <Navigate to="/" replace />;
  }
  return <Outlet />;
}

function MessageRedirect() {
  const { id = "" } = useParams();
  const qs = new URLSearchParams();
  if (id !== "") {
    qs.set("id", id);
  }
  return <Navigate to={id === "" ? "/" : `/?${qs.toString()}`} replace />;
}

export function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route element={<AppShell />}>
            <Route element={<RedirectIfSignedIn />}>
              <Route path="/login" element={<LoginPage />} />
            </Route>
            <Route element={<RequireSession />}>
              <Route path="/" element={<InboxPage />} />
              <Route path="/messages/:id" element={<MessageRedirect />} />
              <Route path="/status" element={<StatusPage />} />
              <Route path="/audit" element={<AuditPage />} />
              <Route path="/reset" element={<ResetPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
