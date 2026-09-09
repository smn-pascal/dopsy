import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

const health = {
  status: "ok",
  docker: { connected: true, mode: "live" },
  ai: { configured: true, model: "test-model" },
};

const containers = [
  {
    id: "container-1",
    name: "api",
    image: "dopsy/api:latest",
    state: "running",
    status: "Up 2 hours",
    health: "healthy",
    created: 1,
  },
];

function jsonResponse(body: unknown, ok = true, status = 200) {
  return Promise.resolve({
    ok,
    status,
    json: () => Promise.resolve(body),
  } as Response);
}

describe("Dopsy app", () => {
  beforeEach(() => {
    Object.defineProperty(globalThis, "crypto", {
      configurable: true,
      value: { randomUUID: vi.fn(() => `${Math.random()}`) },
    });
    Element.prototype.scrollIntoView = vi.fn();
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = input.toString();
        if (url.endsWith("/api/health")) return jsonResponse(health);
        if (url.endsWith("/api/containers")) return jsonResponse(containers);
        return jsonResponse({
          conversationId: "conversation-1",
          answer: "The API container looks healthy.",
          evidence: [],
          steps: [],
        });
      }),
    );
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("loads system and container status", async () => {
    render(<App />);
    expect(await screen.findByText("api")).toBeInTheDocument();
    expect(screen.getByText("Connected")).toBeInTheDocument();
    expect(screen.getByText("test-model")).toBeInTheDocument();
    expect(
      screen.getByText(/bounded log excerpts can contain sensitive data/i),
    ).toBeInTheDocument();
  });

  it("sends a focused question and renders the diagnosis", async () => {
    const user = userEvent.setup();
    render(<App />);
    await user.click(await screen.findByText("api"));
    await user.type(screen.getByLabelText("Ask Dopsy"), "Why is it slow?");
    await user.click(screen.getByLabelText("Send question"));

    expect(
      await screen.findByText("The API container looks healthy."),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith(
        "/api/chat",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            message: "Why is it slow?",
            containerId: "container-1",
          }),
        }),
      );
    });
  });

  it("shows a useful API connection error", async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new Error("offline"));
    render(<App />);
    expect(await screen.findByText("Connection issue")).toBeInTheDocument();
    expect(screen.getByText(/could not reach the API/i)).toBeInTheDocument();
  });

  it("keeps chat usable in demo mode without an AI key", async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) {
        return jsonResponse({
          ...health,
          docker: { connected: true, mode: "demo" },
          ai: { configured: false },
        });
      }
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      return jsonResponse({
        conversationId: "demo-conversation",
        answer: "This is a local demo diagnosis.",
        evidence: [],
        steps: [],
      });
    });

    const user = userEvent.setup();
    render(<App />);

    expect(
      await screen.findByText("Local demo diagnostics"),
    ).toBeInTheDocument();
    expect(screen.getByText("Local demo")).toBeInTheDocument();
    expect(
      screen.getByText(/no AI provider is contacted/i),
    ).toBeInTheDocument();
    const input = screen.getByLabelText("Ask Dopsy");
    expect(input).toBeEnabled();
    await user.type(input, "What happened?");
    await user.click(screen.getByLabelText("Send question"));
    expect(
      await screen.findByText("This is a local demo diagnosis."),
    ).toBeInTheDocument();
  });

  it("shows the provider privacy warning when demo mode uses configured AI", async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) {
        return jsonResponse({
          ...health,
          docker: { connected: true, mode: "demo" },
        });
      }
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      return jsonResponse({
        conversationId: "configured-demo-conversation",
        answer: "Configured provider response.",
        evidence: [],
        steps: [],
      });
    });

    render(<App />);

    expect(await screen.findByText("Demo data")).toBeInTheDocument();
    expect(
      screen.getByText(/bounded log excerpts can contain sensitive data/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/no AI provider is contacted/i),
    ).not.toBeInTheDocument();
  });

  it("keeps conversation context within one selected-container scope", async () => {
    let chatCount = 0;
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) return jsonResponse(health);
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      chatCount += 1;
      return jsonResponse({
        conversationId: `conversation-${chatCount}`,
        answer: `Diagnosis ${chatCount}`,
        evidence: [],
        steps: [],
      });
    });

    const user = userEvent.setup();
    render(<App />);
    await user.click(await screen.findByText("api"));

    const input = screen.getByLabelText("Ask Dopsy");
    await user.type(input, "First question");
    await user.click(screen.getByLabelText("Send question"));
    expect(await screen.findByText("Diagnosis 1")).toBeInTheDocument();

    await user.type(input, "Follow-up question");
    await user.click(screen.getByLabelText("Send question"));
    expect(await screen.findByText("Diagnosis 2")).toBeInTheDocument();

    await user.click(screen.getByText("All containers"));
    expect(screen.getByText("Diagnosis 1")).toBeInTheDocument();
    await user.type(input, "Stack-wide question");
    await user.click(screen.getByLabelText("Send question"));
    expect(await screen.findByText("Diagnosis 3")).toBeInTheDocument();

    const bodies = vi
      .mocked(fetch)
      .mock.calls.filter(([request]) =>
        request.toString().endsWith("/api/chat"),
      )
      .map(
        ([, init]) => JSON.parse(String(init?.body)) as Record<string, string>,
      );

    expect(bodies).toEqual([
      { message: "First question", containerId: "container-1" },
      {
        message: "Follow-up question",
        containerId: "container-1",
        conversationId: "conversation-1",
      },
      { message: "Stack-wide question" },
    ]);
  });
});
