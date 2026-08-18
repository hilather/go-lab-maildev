import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const srcRoot = join(dirname(fileURLToPath(import.meta.url)), "..");

function walk(dir: string): string[] {
  const out: string[] = [];
  for (const ent of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, ent.name);
    if (ent.isDirectory()) {
      out.push(...walk(p));
      continue;
    }
    if (dir.split(sep).includes("test")) {
      continue;
    }
    if (/\.(ts|tsx)$/.test(ent.name) && !ent.name.endsWith(".test.ts") && !ent.name.endsWith(".test.tsx")) {
      out.push(p);
    }
  }
  return out;
}

describe("XSS and secret handling", () => {
  it("never assigns innerHTML, stores tokens, or relaxes the preview sandbox", () => {
    const files = walk(srcRoot);
    expect(files.length).toBeGreaterThan(0);
    for (const file of files) {
      const text = readFileSync(file, "utf8");
      expect(text, file).not.toMatch(/dangerouslySetInnerHTML/);
      expect(text, file).not.toMatch(/\.innerHTML\s*=/);
      if (!file.endsWith(`${sep}storage.ts`)) {
        expect(text, file).not.toMatch(/localStorage|sessionStorage|indexedDB/i);
      }
      if (!file.endsWith(`${sep}sandbox.ts`)) {
        expect(text, file).not.toMatch(/allow-scripts|allow-same-origin|allow-popups-to-escape-sandbox/);
      }
      expect(text, file).not.toMatch(/srcdoc=/);
    }
  });

  it("preview uses the sandboxed /preview route", () => {
    const page = readFileSync(join(srcRoot, "pages/MessagePage.tsx"), "utf8");
    expect(page).toMatch(/sandbox=\{PREVIEW_SANDBOX\}/);
    expect(page).toMatch(/previewURL\(/);
    expect(page).not.toMatch(/Relay/);
  });
});
