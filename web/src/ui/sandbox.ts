// Empty sandbox: no allow-scripts, allow-same-origin, or
// allow-popups-to-escape-sandbox. The preview document is a unique origin
// and cannot mint blob: URLs; cid: parts are inlined as data: by the server.
export const PREVIEW_SANDBOX = "";

const FORBIDDEN_TOKENS = ["allow-scripts", "allow-same-origin", "allow-popups-to-escape-sandbox"];

export function sandboxTokens(value: string): string[] {
  return value.split(/\s+/).filter((t) => t !== "");
}

export function isSafePreviewSandbox(value: string): boolean {
  return sandboxTokens(value).every((t) => !FORBIDDEN_TOKENS.includes(t));
}
