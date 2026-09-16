import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
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

const overview = {
  generatedAt: 1_788_908_400,
  summary: {
    total: 1,
    running: 1,
    healthy: 1,
    needsReview: 0,
  },
  collection: {
    observed: 1,
    total: 1,
    partial: false,
    truncated: false,
  },
  containers: [
    {
      ...containers[0],
      details: {
        running: true,
        oomKilled: false,
        exitCode: 0,
        restartCount: 0,
        startedAt: "2026-09-12T20:00:00Z",
      },
      metrics: {
        cpuPercent: 1.2,
        memoryUsageBytes: 96 * 1024 * 1024,
        memoryLimitBytes: 512 * 1024 * 1024,
        memoryPercent: 18.75,
        readAt: 1_788_908_400,
      },
      assessment: {
        level: "ok",
        summary: "No current warning signals.",
      },
    },
  ],
};

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
        if (url.endsWith("/api/overview")) return jsonResponse(overview);
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
    vi.useRealTimers();
    Reflect.deleteProperty(document, "visibilityState");
    vi.unstubAllGlobals();
  });

  it("opens the overview with container and system status", async () => {
    render(<App />);
    expect(
      await screen.findByRole("heading", {
        name: "Übersicht",
        level: 1,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("navigation", { name: "Bereiche" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Übersicht" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByRole("button", { name: "Diagnose" }),
    ).not.toHaveAttribute("aria-current");
    expect(screen.getByText("Verbunden")).toBeInTheDocument();
    expect(screen.getByText("test-model")).toBeInTheDocument();
    expect(screen.getByText("v0.1.3")).toBeInTheDocument();
    expect(screen.getByText("Gesamt")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "CPU-Auslastung", level: 2 }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "1 von 1 Containern laufen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "api", level: 3 }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: /api: RAM/i }),
    ).toHaveAttribute("aria-valuenow", "19");
    expect(screen.getByRole("button", { name: "Aktualisieren" })).toBeEnabled();

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Diagnose" }));
    expect(
      screen.getByText(/Begrenzte Logauszüge können sensible Daten enthalten/i),
    ).toBeInTheDocument();
  });

  it("sends a focused question and renders the diagnosis", async () => {
    const user = userEvent.setup();
    render(<App />);
    await user.click(await screen.findByRole("button", { name: "Diagnose" }));
    await user.click(await screen.findByText("api"));
    await user.type(screen.getByLabelText("Dopsy fragen"), "Why is it slow?");
    await user.click(screen.getByLabelText("Frage senden"));

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

  it("shows retained event facts with the incomplete-history warning", async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) return jsonResponse(health);
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      if (url.endsWith("/api/overview")) return jsonResponse(overview);
      return jsonResponse({
        conversationId: "event-conversation",
        answer: "Docker retained an OOM event before the exit.",
        evidence: [
          {
            label: "Event history",
            value: "Limited Docker buffer; not a complete archive",
            severity: "warning",
          },
          {
            label: "Docker event",
            value: "oom · 2026-09-16T12:00:00Z",
            severity: "critical",
          },
        ],
        steps: [
          {
            tool: "get_container_events",
            summary:
              "Read retained container events; history may be incomplete",
          },
        ],
      });
    });
    const user = userEvent.setup();
    render(<App />);
    await user.click(
      await screen.findByRole("button", { name: "api untersuchen" }),
    );
    await user.type(screen.getByLabelText("Dopsy fragen"), "What happened?");
    await user.click(screen.getByLabelText("Frage senden"));
    expect(
      await screen.findByText("oom · 2026-09-16T12:00:00Z"),
    ).toBeInTheDocument();
    expect(screen.getByText(/not a complete archive/i)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Diagnoseschritte/i }));
    expect(screen.getByText("get_container_events")).toBeInTheDocument();
  });

  it("shows a useful API connection error", async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new Error("offline"));
    render(<App />);
    const user = userEvent.setup();
    await user.click(await screen.findByRole("button", { name: "Diagnose" }));
    expect(await screen.findByText("Verbindung gestört")).toBeInTheDocument();
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
      if (url.endsWith("/api/overview")) return jsonResponse(overview);
      return jsonResponse({
        conversationId: "demo-conversation",
        answer: "This is a local demo diagnosis.",
        evidence: [],
        steps: [],
      });
    });

    const user = userEvent.setup();
    render(<App />);

    await user.click(await screen.findByRole("button", { name: "Diagnose" }));

    expect(await screen.findByText("Demodaten")).toBeInTheDocument();
    expect(
      screen.getByText(/Keine Daten an einen KI-Anbieter/i),
    ).toBeInTheDocument();
    const input = screen.getByLabelText("Dopsy fragen");
    expect(input).toBeEnabled();
    await user.type(input, "What happened?");
    await user.click(screen.getByLabelText("Frage senden"));
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
      if (url.endsWith("/api/overview")) return jsonResponse(overview);
      return jsonResponse({
        conversationId: "configured-demo-conversation",
        answer: "Configured provider response.",
        evidence: [],
        steps: [],
      });
    });

    const user = userEvent.setup();
    render(<App />);

    await user.click(await screen.findByRole("button", { name: "Diagnose" }));

    expect(await screen.findByText("Demodaten")).toBeInTheDocument();
    expect(
      screen.getByText(/Begrenzte Logauszüge können sensible Daten enthalten/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Keine Daten an einen KI-Anbieter/i),
    ).not.toBeInTheDocument();
  });

  it("keeps conversation context within one selected-container scope", async () => {
    let chatCount = 0;
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) return jsonResponse(health);
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      if (url.endsWith("/api/overview")) return jsonResponse(overview);
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
    await user.click(await screen.findByRole("button", { name: "Diagnose" }));
    await user.click(await screen.findByText("api"));

    const input = screen.getByLabelText("Dopsy fragen");
    await user.type(input, "First question");
    await user.click(screen.getByLabelText("Frage senden"));
    expect(await screen.findByText("Diagnosis 1")).toBeInTheDocument();

    await user.type(input, "Follow-up question");
    await user.click(screen.getByLabelText("Frage senden"));
    expect(await screen.findByText("Diagnosis 2")).toBeInTheDocument();

    await user.click(screen.getByText("Alle Container"));
    expect(screen.getByText("Diagnosis 1")).toBeInTheDocument();
    await user.type(input, "Stack-wide question");
    await user.click(screen.getByLabelText("Frage senden"));
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

  it("opens a correctly scoped investigation from the overview", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      await screen.findByRole("button", { name: "api untersuchen" }),
    );
    expect(screen.getByRole("button", { name: "Diagnose" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByLabelText("Container-Auswahl aufheben"),
    ).toBeInTheDocument();

    await user.type(
      screen.getByLabelText("Dopsy fragen"),
      "Explain this snapshot",
    );
    await user.click(screen.getByLabelText("Frage senden"));

    await waitFor(() => {
      const chatCall = vi
        .mocked(fetch)
        .mock.calls.find(([request]) =>
          request.toString().endsWith("/api/chat"),
        );
      expect(JSON.parse(String(chatCall?.[1]?.body))).toEqual({
        message: "Explain this snapshot",
        containerId: "container-1",
      });
    });
  });

  it("keeps the overview usable when AI is not configured", async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) {
        return jsonResponse({ ...health, ai: { configured: false } });
      }
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      if (url.endsWith("/api/overview")) return jsonResponse(overview);
      return jsonResponse({}, false, 503);
    });

    render(<App />);

    expect(
      await screen.findByRole("heading", {
        name: "Übersicht",
        level: 1,
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aktualisieren" })).toBeEnabled();
    expect(screen.queryByText("KI nicht eingerichtet")).not.toBeInTheDocument();

    await userEvent
      .setup()
      .click(screen.getByRole("button", { name: "Diagnose" }));
    expect(screen.getByRole("alert")).toHaveTextContent(
      "KI nicht eingerichtet",
    );
    expect(
      screen.getByRole("textbox", { name: "Dopsy fragen" }),
    ).toBeDisabled();
  });

  it("shows partial collection coverage without inventing missing metrics", async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) return jsonResponse(health);
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      if (url.endsWith("/api/overview")) {
        return jsonResponse({
          ...overview,
          collection: {
            observed: 1,
            total: 3,
            partial: true,
            truncated: true,
          },
          containers: [
            {
              ...overview.containers[0],
              metrics: undefined,
              assessment: {
                level: "unknown",
                summary: "Resource snapshot could not be collected.",
              },
            },
          ],
        });
      }
      return jsonResponse({});
    });

    render(<App />);

    expect(await screen.findByText(/Daten teilweise/i)).toHaveTextContent(
      "1/3",
    );
    const card = screen
      .getByRole("heading", { name: "api", level: 3 })
      .closest(".container-card");
    expect(card).toHaveTextContent("–");
    expect(card).not.toHaveTextContent("0,0 %");
    expect(
      screen.getByRole("img", {
        name: /Beobachtet: 0 kritisch, 0 prüfen, 1 unklar, 0 ohne Warnsignal/i,
      }),
    ).toBeInTheDocument();
  });

  it("puts critical containers before healthy containers", async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) return jsonResponse(health);
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      if (url.endsWith("/api/overview")) {
        return jsonResponse({
          ...overview,
          summary: { ...overview.summary, total: 2, needsReview: 1 },
          collection: { ...overview.collection, observed: 2, total: 2 },
          containers: [
            overview.containers[0],
            {
              id: "container-2",
              name: "worker",
              image: "dopsy/worker:latest",
              state: "exited",
              health: "healthy",
              status: "Exited (137) 2 minutes ago",
              created: 1,
              details: {
                running: false,
                oomKilled: true,
                exitCode: 137,
                restartCount: 4,
              },
              assessment: {
                level: "critical",
                summary: "Docker reports an out-of-memory termination.",
              },
            },
          ],
        });
      }
      return jsonResponse({});
    });

    render(<App />);

    await screen.findByRole("heading", { name: "worker", level: 3 });
    const cards = screen.getAllByRole("article");
    expect(cards[0]).toHaveTextContent("worker");
    expect(cards[0]).toHaveTextContent("Beendet");
    expect(cards[0]).not.toHaveTextContent("Gesund");
    expect(cards[0]).toHaveTextContent("Speichermangel");
    expect(cards[1]).toHaveTextContent("api");
  });

  it("uses a labelled CPU scale above 100 percent without inventing history", async () => {
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) return jsonResponse(health);
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      if (url.endsWith("/api/overview")) {
        return jsonResponse({
          ...overview,
          containers: [
            {
              ...overview.containers[0],
              metrics: { ...overview.containers[0].metrics, cpuPercent: 240 },
            },
          ],
        });
      }
      return jsonResponse({});
    });

    render(<App />);
    expect(await screen.findByText("Skala 0–500,0 %")).toBeInTheDocument();
    expect(screen.getAllByText("240,0 %").length).toBeGreaterThan(0);
    expect(
      screen.getByText("Nur aktuelle Messwerte · kein Verlauf"),
    ).toBeInTheDocument();
  });

  it("refreshes the snapshot only when requested", async () => {
    const user = userEvent.setup();
    render(<App />);
    await screen.findByRole("heading", { name: "Übersicht", level: 1 });

    const overviewCalls = () =>
      vi
        .mocked(fetch)
        .mock.calls.filter(([request]) =>
          request.toString().endsWith("/api/overview"),
        ).length;

    expect(overviewCalls()).toBe(1);
    await user.click(screen.getByRole("button", { name: "Aktualisieren" }));
    await waitFor(() => expect(overviewCalls()).toBe(2));
  });

  it("polls only when Live is enabled on a visible overview", async () => {
    render(<App />);
    await screen.findByRole("heading", { name: "Übersicht", level: 1 });
    const overviewCalls = () =>
      vi
        .mocked(fetch)
        .mock.calls.filter(([request]) =>
          request.toString().endsWith("/api/overview"),
        ).length;

    vi.useFakeTimers();
    fireEvent.click(
      screen.getByRole("button", { name: "Live-Aktualisierung einschalten" }),
    );
    expect(
      screen.getByRole("button", { name: "Live-Aktualisierung ausschalten" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(overviewCalls()).toBe(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(overviewCalls()).toBe(2);

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      value: "hidden",
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(overviewCalls()).toBe(2);

    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      value: "visible",
    });
    fireEvent.click(screen.getByRole("button", { name: "Diagnose" }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(overviewCalls()).toBe(2);

    fireEvent.click(screen.getByRole("button", { name: "Übersicht" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Live-Aktualisierung ausschalten" }),
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(overviewCalls()).toBe(2);
  });

  it("does not overlap dashboard refreshes when one Docker read is slow", async () => {
    render(<App />);
    await screen.findByRole("heading", { name: "Übersicht", level: 1 });
    let releaseRead: (() => void) | undefined;
    const slowRead = new Promise<void>((resolve) => {
      releaseRead = resolve;
    });
    let overviewCalls = 0;
    vi.mocked(fetch).mockImplementation((input: RequestInfo | URL) => {
      const url = input.toString();
      if (url.endsWith("/api/health")) return jsonResponse(health);
      if (url.endsWith("/api/containers")) return jsonResponse(containers);
      if (url.endsWith("/api/overview")) {
        overviewCalls += 1;
        return overviewCalls === 1
          ? slowRead.then(() => jsonResponse(overview))
          : jsonResponse(overview);
      }
      return jsonResponse({});
    });

    vi.useFakeTimers();
    fireEvent.click(
      screen.getByRole("button", { name: "Live-Aktualisierung einschalten" }),
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
    });
    expect(overviewCalls).toBe(1);
    await act(async () => {
      releaseRead?.();
      await Promise.resolve();
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(overviewCalls).toBe(2);
  });
});
