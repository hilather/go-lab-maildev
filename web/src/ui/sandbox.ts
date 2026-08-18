// Empty sandbox: no allow-scripts, allow-same-origin, or
// allow-popups-to-escape-sandbox. The preview document is a unique origin
// and cannot mint blob: URLs; cid: parts are inlined as data: by the server.
export const PREVIEW_SANDBOX = "";

export function sandboxTokens(value: string): string[] {
  return value.split(/\s+/).filter((t) => t !== "");
}

// Only the frozen empty sandbox is safe. allow-forms / allow-popups / a
// missing attribute must not pass.
export function isSafePreviewSandbox(value: string | null): boolean {
  return value === PREVIEW_SANDBOX;
}
