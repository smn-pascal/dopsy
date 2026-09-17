// @vitest-environment node

import { describe, expect, it } from "vitest";
import config from "../../vite.config";

describe("development API proxy", () => {
  it("preserves the browser host instead of changing it to the API target", () => {
    expect(config.server?.proxy?.["/api"]).toMatchObject({
      target: "http://127.0.0.1:3001",
      changeOrigin: false,
    });
  });
});
