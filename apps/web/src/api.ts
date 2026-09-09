import type { ChatResponse, Container, HealthResponse } from "./types";

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status?: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  const headers = new Headers(init?.headers);
  if (init?.body) headers.set("Content-Type", "application/json");

  try {
    response = await fetch(path, {
      ...init,
      headers,
    });
  } catch {
    throw new ApiError(
      "Dopsy could not reach the API. Check that the server is running.",
    );
  }

  if (!response.ok) {
    let message = `The request failed (${response.status}).`;
    try {
      const body = (await response.json()) as {
        error?: string;
        message?: string;
      };
      message = body.message ?? body.error ?? message;
    } catch {
      // The status code is still useful when an upstream proxy returns non-JSON.
    }
    throw new ApiError(message, response.status);
  }

  return (await response.json()) as T;
}

export const api = {
  health: () => request<HealthResponse>("/api/health"),
  containers: () => request<Container[]>("/api/containers"),
  chat: (message: string, containerId?: string, conversationId?: string) =>
    request<ChatResponse>("/api/chat", {
      method: "POST",
      body: JSON.stringify({
        message,
        ...(containerId ? { containerId } : {}),
        ...(conversationId ? { conversationId } : {}),
      }),
    }),
};
