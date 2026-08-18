export const SCOPE_READ = "mail.read";
export const SCOPE_WRITE = "mail.write";
export const SCOPE_ADMIN = "mail.admin";
export const SCOPE_AUDIT = "mail.audit.read";

export function hasScope(scopes: readonly string[], need: string): boolean {
  return scopes.includes(SCOPE_ADMIN) || scopes.includes(need);
}

export function formatAddress(name: string, address: string): string {
  if (name !== "" && address !== "") {
    return `${name} <${address}>`;
  }
  return address || name || "(none)";
}

export function formatBytes(n: number): string {
  if (n < 1024) {
    return `${n} B`;
  }
  if (n < 1024 * 1024) {
    return `${(n / 1024).toFixed(1)} KiB`;
  }
  return `${(n / (1024 * 1024)).toFixed(1)} MiB`;
}
