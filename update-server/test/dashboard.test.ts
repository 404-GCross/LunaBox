import { describe, expect, it } from "vitest";
import { parseReleaseManifest, summarizePatches } from "../src/dashboard";

describe("release dashboard manifest parsing", () => {
  it("summarizes patch sources and their savings by channel", () => {
    const manifest = parseReleaseManifest({
      schema_version: 1,
      version: "2.0.0",
      channels: {
        "windows-amd64-portable": {
          files: [
            {
              path: "LunaBox.exe",
              full: { size: 1_000, url: "https://example.com/full", sha256: "full" },
              patch: {
                size: 250,
                source_version: "1.11.2",
                source_sha256: "source",
                url: "https://example.com/patch",
                sha256: "patch",
              },
            },
          ],
        },
        "windows-arm64-portable": {
          files: [
            {
              path: "LunaBox.exe",
              full: { size: 800 },
              patch: { size: 400, source_version: "1.11.2" },
            },
          ],
        },
      },
    });

    expect(manifest).not.toBeNull();
    expect(summarizePatches(manifest!)).toEqual([
      {
        source_version: "1.11.2",
        channels: [
          {
            name: "windows-amd64-portable",
            asset: "LunaBox.exe",
            patch_size: 250,
            full_size: 1_000,
            saving_percent: 75,
          },
          {
            name: "windows-arm64-portable",
            asset: "LunaBox.exe",
            patch_size: 400,
            full_size: 800,
            saving_percent: 50,
          },
        ],
      },
    ]);
  });

  it("rejects malformed channel files", () => {
    expect(parseReleaseManifest({
      version: "2.0.0",
      channels: { stable: { files: [{ path: "LunaBox.exe" }] } },
    })).toBeNull();
  });
});
