import {
  FormEvent,
  KeyboardEvent,
  ReactNode,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { api } from "./api";
import type {
  ChatMessage,
  Container,
  Evidence,
  HealthResponse,
  InvestigationStep,
} from "./types";

const sampleQuestions = [
  {
    icon: "pulse",
    title: "Find the bottleneck",
    prompt: "Which container is using the most resources, and why?",
  },
  {
    icon: "history",
    title: "Explain a restart",
    prompt: "Why did this container restart, and what should I check next?",
  },
  {
    icon: "logs",
    title: "Investigate errors",
    prompt: "Are there any important errors in the recent logs?",
  },
] as const;

const loadingSteps = [
  "Preparing the diagnostic context",
  "Read-only analysis in progress",
  "Waiting for the diagnostic response",
];

function Icon({ name, size = 18 }: { name: string; size?: number }) {
  const paths: Record<string, ReactNode> = {
    logo: (
      <>
        <path d="M8.2 5.6h7.6a4.2 4.2 0 0 1 0 8.4H8.2V5.6Z" />
        <path d="M8.2 9.8h5.4" />
      </>
    ),
    menu: <path d="M4 7h16M4 12h16M4 17h16" />,
    close: <path d="m6 6 12 12M18 6 6 18" />,
    search: (
      <path d="m21 21-4.35-4.35m2.35-5.15A7.5 7.5 0 1 1 4 11.5a7.5 7.5 0 0 1 15 0Z" />
    ),
    boxes: (
      <>
        <path d="m12 3 7 4-7 4-7-4 7-4Z" />
        <path d="m5 12 7 4 7-4M5 17l7 4 7-4" />
      </>
    ),
    chevron: <path d="m9 18 6-6-6-6" />,
    pulse: <path d="M3 12h4l2.3-6 4.2 12 2.2-6H21" />,
    history: (
      <>
        <path d="M3 12a9 9 0 1 0 3-6.7L3 8" />
        <path d="M3 3v5h5M12 7v5l3 2" />
      </>
    ),
    logs: (
      <>
        <path d="M5 4h14v16H5z" />
        <path d="M8 8h8M8 12h8M8 16h5" />
      </>
    ),
    sparkles: (
      <>
        <path d="m12 3 .8 2.2L15 6l-2.2.8L12 9l-.8-2.2L9 6l2.2-.8L12 3Z" />
        <path d="m18 12 .6 1.4L20 14l-1.4.6L18 16l-.6-1.4L16 14l1.4-.6L18 12ZM6 11l1.2 3.8L11 16l-3.8 1.2L6 21l-1.2-3.8L1 16l3.8-1.2L6 11Z" />
      </>
    ),
    send: (
      <>
        <path d="m4 4 17 8-17 8 3-8-3-8Z" />
        <path d="M7 12h14" />
      </>
    ),
    shield: (
      <>
        <path d="M12 3 5 6v5c0 4.6 2.8 8 7 10 4.2-2 7-5.4 7-10V6l-7-3Z" />
        <path d="m9 12 2 2 4-4" />
      </>
    ),
    refresh: (
      <>
        <path d="M20 7v5h-5" />
        <path d="M18.1 16A7 7 0 1 1 19 8l1 4" />
      </>
    ),
    bot: (
      <>
        <rect x="4" y="7" width="16" height="12" rx="4" />
        <path d="M12 3v4M8.5 12h.01M15.5 12h.01M9 16h6" />
      </>
    ),
    copy: (
      <>
        <rect x="8" y="8" width="11" height="11" rx="2" />
        <path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" />
      </>
    ),
    check: <path d="m5 12 4 4L19 6" />,
    alert: (
      <>
        <path d="M10.3 4.2 2.8 17a2 2 0 0 0 1.7 3h15a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0Z" />
        <path d="M12 9v4M12 17h.01" />
      </>
    ),
    tool: (
      <>
        <path d="M14.7 6.3a4 4 0 0 0-5-5L12 3.6 9.6 6 7.3 3.7a4 4 0 0 0 5 5l-7.6 7.6a2.1 2.1 0 0 0 3 3l7.6-7.6a4 4 0 0 0 5-5L18 6l-2.4-2.4 2.3-2.3" />
      </>
    ),
    external: (
      <path d="M14 4h6v6M20 4l-9 9M18 13v6a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h6" />
    ),
  };

  return (
    <svg
      aria-hidden="true"
      className="icon"
      fill="none"
      height={size}
      viewBox="0 0 24 24"
      width={size}
    >
      <g
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      >
        {paths[name]}
      </g>
    </svg>
  );
}

function Logo() {
  return (
    <div className="brand" aria-label="Dopsy home">
      <span className="brand-mark">
        <Icon name="logo" size={24} />
      </span>
      <span className="brand-word">dopsy</span>
      <span className="version">v0.1</span>
    </div>
  );
}

function stateTone(container: Container) {
  const state = `${container.state} ${container.health ?? ""}`.toLowerCase();
  if (
    state.includes("unhealthy") ||
    state.includes("dead") ||
    state.includes("exited")
  )
    return "danger";
  if (
    state.includes("starting") ||
    state.includes("paused") ||
    state.includes("restart")
  )
    return "warning";
  if (state.includes("running") || state.includes("healthy")) return "healthy";
  return "muted";
}

function statusLabel(container: Container) {
  if (container.health) return container.health;
  return container.state;
}

type SidebarProps = {
  containers: Container[];
  health: HealthResponse | null;
  error: string | null;
  loading: boolean;
  selectedId?: string;
  open: boolean;
  onClose: () => void;
  onRefresh: () => void;
  onSelect: (id?: string) => void;
};

function Sidebar({
  containers,
  health,
  error,
  loading,
  selectedId,
  open,
  onClose,
  onRefresh,
  onSelect,
}: SidebarProps) {
  const [query, setQuery] = useState("");
  const demoProvider = health?.docker.mode === "demo" && !health.ai.configured;
  const visibleContainers = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) return containers;
    return containers.filter((container) =>
      `${container.name} ${container.image}`.toLowerCase().includes(normalized),
    );
  }, [containers, query]);

  const selectContainer = (id?: string) => {
    onSelect(id);
    onClose();
  };

  return (
    <>
      <button
        aria-label="Close navigation"
        className={`sidebar-backdrop ${open ? "visible" : ""}`}
        onClick={onClose}
        tabIndex={open ? 0 : -1}
      />
      <aside
        className={`sidebar ${open ? "open" : ""}`}
        aria-label="Container navigation"
      >
        <div className="sidebar-top">
          <Logo />
          <button
            className="icon-button sidebar-close"
            aria-label="Close sidebar"
            onClick={onClose}
          >
            <Icon name="close" />
          </button>
        </div>

        <div className="sidebar-section-heading">
          <span>Containers</span>
          <span className="count">{containers.length}</span>
        </div>

        <label className="search-field">
          <span className="sr-only">Search containers</span>
          <Icon name="search" size={16} />
          <input
            type="search"
            placeholder="Search containers"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <kbd>/</kbd>
        </label>

        <nav className="container-nav" aria-label="Available containers">
          <button
            className={`container-row all-containers ${selectedId === undefined ? "selected" : ""}`}
            onClick={() => selectContainer(undefined)}
          >
            <span className="container-icon">
              <Icon name="boxes" size={17} />
            </span>
            <span className="container-details">
              <strong>All containers</strong>
              <small>Stack-wide diagnosis</small>
            </span>
            <Icon name="chevron" size={15} />
          </button>

          {loading && containers.length === 0 && (
            <div
              className="container-skeletons"
              aria-label="Loading containers"
            >
              {[0, 1, 2].map((item) => (
                <span className="skeleton-row" key={item} />
              ))}
            </div>
          )}

          {!loading && error && (
            <div className="sidebar-error" role="alert">
              <Icon name="alert" size={16} />
              <span>Containers unavailable</span>
            </div>
          )}

          {!loading && !error && containers.length === 0 && (
            <div className="empty-containers">
              <span>No containers found</span>
              <small>Start a container, then refresh.</small>
            </div>
          )}

          {visibleContainers.map((container) => (
            <button
              aria-pressed={selectedId === container.id}
              className={`container-row ${selectedId === container.id ? "selected" : ""}`}
              key={container.id}
              onClick={() => selectContainer(container.id)}
            >
              <span
                className={`state-dot ${stateTone(container)}`}
                aria-hidden="true"
              />
              <span className="container-details">
                <strong>{container.name}</strong>
                <small title={container.image}>{container.image}</small>
              </span>
              <span className={`state-label ${stateTone(container)}`}>
                {statusLabel(container)}
              </span>
            </button>
          ))}

          {query && visibleContainers.length === 0 && containers.length > 0 && (
            <p className="no-search-results">No matches for “{query}”</p>
          )}
        </nav>

        <div className="sidebar-status" aria-label="System status">
          <div className="status-heading">
            <span>System status</span>
            <button
              aria-label="Refresh status"
              className={`refresh-button ${loading ? "spinning" : ""}`}
              disabled={loading}
              onClick={onRefresh}
            >
              <Icon name="refresh" size={14} />
            </button>
          </div>
          <div className="status-row">
            <span
              className={`status-dot ${health?.docker.connected ? "online" : "offline"}`}
            />
            <span>Docker</span>
            <strong>
              {health
                ? health.docker.connected
                  ? health.docker.mode === "demo"
                    ? "Demo"
                    : "Connected"
                  : "Offline"
                : "Checking"}
            </strong>
          </div>
          <div className="status-row">
            <span
              className={`status-dot ${health?.ai.configured || demoProvider ? "online" : "offline"}`}
            />
            <span>AI provider</span>
            <strong>
              {health
                ? health.ai.configured
                  ? (health.ai.model ?? "Ready")
                  : demoProvider
                    ? "Local demo"
                    : "Not configured"
                : "Checking"}
            </strong>
          </div>
          {health?.docker.message && (
            <p className="status-note">{health.docker.message}</p>
          )}
          <div className="read-only-note">
            <Icon name="shield" size={14} /> Read-only access
          </div>
        </div>
      </aside>
    </>
  );
}

function EvidenceGrid({ evidence }: { evidence: Evidence[] }) {
  if (evidence.length === 0) return null;
  return (
    <div className="evidence-grid" aria-label="Evidence">
      {evidence.map((item, index) => (
        <div
          className={`evidence-card ${item.severity ?? "info"}`}
          key={`${item.label}-${index}`}
        >
          <span>{item.label}</span>
          <strong>{item.value}</strong>
        </div>
      ))}
    </div>
  );
}

function ToolSteps({ steps }: { steps: InvestigationStep[] }) {
  const [expanded, setExpanded] = useState(false);
  if (steps.length === 0) return null;
  return (
    <div className="tool-steps">
      <button
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
      >
        <span>
          <Icon name="tool" size={15} /> Investigated with {steps.length}{" "}
          read-only {steps.length === 1 ? "tool" : "tools"}
        </span>
        <span className={`disclosure ${expanded ? "expanded" : ""}`}>
          <Icon name="chevron" size={14} />
        </span>
      </button>
      {expanded && (
        <ol>
          {steps.map((step, index) => (
            <li key={`${step.tool}-${index}`}>
              <span className="step-check">
                <Icon name="check" size={12} />
              </span>
              <div>
                <code>{step.tool}</code>
                <p>{step.summary}</p>
              </div>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

function AssistantMessage({
  message,
}: {
  message: Extract<ChatMessage, { role: "assistant" }>;
}) {
  const [copied, setCopied] = useState(false);

  const copyAnswer = async () => {
    try {
      await navigator.clipboard.writeText(message.content);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      // Clipboard access may be unavailable in an insecure self-hosted context.
    }
  };

  return (
    <article className="message assistant-message">
      <div className="assistant-avatar">
        <Icon name="logo" size={19} />
      </div>
      <div className="message-content">
        <div className="message-meta">
          <strong>Dopsy</strong>
          <span>Diagnostic assistant</span>
        </div>
        <ToolSteps steps={message.steps} />
        <p className="answer-text">{message.content}</p>
        <EvidenceGrid evidence={message.evidence} />
        <button className="copy-button" onClick={copyAnswer}>
          <Icon name={copied ? "check" : "copy"} size={14} />{" "}
          {copied ? "Copied" : "Copy answer"}
        </button>
      </div>
    </article>
  );
}

function InvestigationLoader({
  selectedName,
  step,
}: {
  selectedName?: string;
  step: number;
}) {
  return (
    <article
      className="message assistant-message"
      aria-live="polite"
      aria-label="Dopsy is preparing a diagnosis"
    >
      <div className="assistant-avatar active">
        <Icon name="logo" size={19} />
      </div>
      <div className="message-content loading-message">
        <div className="message-meta">
          <strong>
            Preparing a diagnosis{selectedName ? ` for ${selectedName}` : ""}
          </strong>
        </div>
        <div className="loader-steps">
          {loadingSteps.map((label, index) => (
            <div
              className={`loader-step ${index < step ? "done" : index === step ? "current" : ""}`}
              key={label}
            >
              <span>
                {index < step ? <Icon name="check" size={11} /> : index + 1}
              </span>
              <p>{label}</p>
              {index === step && <i aria-hidden="true" />}
            </div>
          ))}
        </div>
      </div>
    </article>
  );
}

type EmptyStateProps = {
  selected?: Container;
  disabled: boolean;
  onPrompt: (prompt: string) => void;
};

function EmptyState({ selected, disabled, onPrompt }: EmptyStateProps) {
  return (
    <section className="empty-state" aria-labelledby="welcome-title">
      <div className="hero-mark">
        <span>
          <Icon name="sparkles" size={28} />
        </span>
      </div>
      <div className="eyebrow">
        <Icon name="shield" size={13} /> Read-only diagnostic workspace
      </div>
      <h1 id="welcome-title">
        Ask your stack
        <br />
        what happened.
      </h1>
      <p>
        Dopsy investigates the right logs, state, and metadata for your
        question—then explains the evidence in plain language.
      </p>
      {selected && (
        <div className="selected-context">
          <span className={`state-dot ${stateTone(selected)}`} />
          Questions will focus on <strong>{selected.name}</strong>
        </div>
      )}
      <div className="prompt-grid">
        {sampleQuestions.map((item) => (
          <button
            disabled={disabled}
            key={item.title}
            onClick={() => onPrompt(item.prompt)}
          >
            <span className="prompt-icon">
              <Icon name={item.icon} size={18} />
            </span>
            <span>
              <strong>{item.title}</strong>
              <small>{item.prompt}</small>
            </span>
            <Icon name="chevron" size={16} />
          </button>
        ))}
      </div>
    </section>
  );
}

export default function App() {
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [containers, setContainers] = useState<Container[]>([]);
  const [selectedId, setSelectedId] = useState<string>();
  const [conversationId, setConversationId] = useState<string>();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loaderStep, setLoaderStep] = useState(0);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const scopeVersionRef = useRef(0);

  const selected = containers.find((container) => container.id === selectedId);

  const loadData = async () => {
    setLoading(true);
    setLoadError(null);
    const [healthResult, containersResult] = await Promise.allSettled([
      api.health(),
      api.containers(),
    ]);

    if (healthResult.status === "fulfilled") setHealth(healthResult.value);
    if (containersResult.status === "fulfilled") {
      setContainers(containersResult.value);
      if (
        selectedId &&
        !containersResult.value.some((container) => container.id === selectedId)
      ) {
        scopeVersionRef.current += 1;
        setConversationId(undefined);
        setSelectedId(undefined);
      }
    }

    const failures = [healthResult, containersResult].filter(
      (result) => result.status === "rejected",
    );
    if (failures.length) {
      const first = failures[0] as PromiseRejectedResult;
      setLoadError(
        first.reason instanceof Error
          ? first.reason.message
          : "Dopsy could not load system data.",
      );
    }
    setLoading(false);
  };

  useEffect(() => {
    void loadData();
  }, []);

  useEffect(() => {
    if (!sending) {
      setLoaderStep(0);
      return;
    }
    const timer = window.setInterval(() => {
      setLoaderStep((current) =>
        Math.min(current + 1, loadingSteps.length - 1),
      );
    }, 1400);
    return () => window.clearInterval(timer);
  }, [sending]);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages, sending, loaderStep]);

  useEffect(() => {
    const focusSearch = (event: globalThis.KeyboardEvent) => {
      if (
        event.key === "/" &&
        !(event.target instanceof HTMLInputElement) &&
        !(event.target instanceof HTMLTextAreaElement)
      ) {
        event.preventDefault();
        setSidebarOpen(true);
        window.setTimeout(
          () =>
            document
              .querySelector<HTMLInputElement>(".search-field input")
              ?.focus(),
          0,
        );
      }
    };
    window.addEventListener("keydown", focusSearch);
    return () => window.removeEventListener("keydown", focusSearch);
  }, []);

  const selectScope = (containerId?: string) => {
    if (containerId === selectedId) return;
    scopeVersionRef.current += 1;
    setConversationId(undefined);
    setSelectedId(containerId);
  };

  const submitMessage = async (
    message: string,
    containerId = selectedId,
    requestConversationId = conversationId,
  ) => {
    const trimmed = message.trim();
    if (!trimmed || sending) return;

    const requestScopeVersion = scopeVersionRef.current;
    const belongsToCurrentScope = containerId === selectedId;
    const container = containers.find((item) => item.id === containerId);
    const userMessage: ChatMessage = {
      id: crypto.randomUUID(),
      role: "user",
      content: trimmed,
      containerName: container?.name,
    };
    setMessages((current) => [...current, userMessage]);
    setInput("");
    setSending(true);

    try {
      const response = await api.chat(
        trimmed,
        containerId,
        requestConversationId,
      );
      if (
        belongsToCurrentScope &&
        scopeVersionRef.current === requestScopeVersion
      ) {
        setConversationId(response.conversationId);
      }
      setMessages((current) => [
        ...current,
        {
          id: crypto.randomUUID(),
          role: "assistant",
          content: response.answer,
          evidence: response.evidence ?? [],
          steps: response.steps ?? [],
        },
      ]);
    } catch (error) {
      setMessages((current) => [
        ...current,
        {
          id: crypto.randomUUID(),
          role: "error",
          content:
            error instanceof Error
              ? error.message
              : "The diagnosis failed unexpectedly.",
          originalMessage: trimmed,
          containerId,
        },
      ]);
    } finally {
      setSending(false);
      window.setTimeout(() => textareaRef.current?.focus(), 0);
    }
  };

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    void submitMessage(input);
  };

  const onTextareaKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      if (input.trim()) void submitMessage(input);
    }
  };

  const demoProvider =
    health !== null && health.docker.mode === "demo" && !health.ai.configured;
  const aiUnavailable =
    health !== null && !health.ai.configured && !demoProvider;
  const dockerUnavailable = health !== null && !health.docker.connected;
  const inputDisabled = sending || aiUnavailable || dockerUnavailable;

  return (
    <div className="app-shell">
      <Sidebar
        containers={containers}
        error={loadError}
        health={health}
        loading={loading}
        onClose={() => setSidebarOpen(false)}
        onRefresh={() => void loadData()}
        onSelect={selectScope}
        open={sidebarOpen}
        selectedId={selectedId}
      />

      <main className="workspace">
        <header className="topbar">
          <div className="topbar-left">
            <button
              className="icon-button mobile-menu"
              aria-label="Open sidebar"
              onClick={() => setSidebarOpen(true)}
            >
              <Icon name="menu" />
            </button>
            <div>
              <span className="mobile-brand">dopsy</span>
              <h2>{selected ? selected.name : "All containers"}</h2>
              <p>
                {selected
                  ? selected.image
                  : "Ask across your Docker environment"}
              </p>
            </div>
          </div>
          <div className="topbar-actions">
            {health?.docker.mode === "demo" && (
              <span className="demo-badge">Demo data</span>
            )}
            <span className="safety-badge">
              <Icon name="shield" size={14} /> Read only
            </span>
            <a
              className="docs-link"
              href="https://github.com/smn-pascal/dopsy"
              target="_blank"
              rel="noreferrer"
            >
              Docs <Icon name="external" size={13} />
            </a>
          </div>
        </header>

        {loadError && (
          <div className="global-alert" role="alert">
            <Icon name="alert" size={17} />
            <span>
              <strong>Connection issue</strong>
              {loadError}
            </span>
            <button onClick={() => void loadData()}>Try again</button>
          </div>
        )}

        {aiUnavailable && (
          <div className="global-alert warning" role="alert">
            <Icon name="alert" size={17} />
            <span>
              <strong>AI provider not configured</strong>Add a supported
              provider in your Dopsy environment to start a diagnosis.
            </span>
          </div>
        )}

        {demoProvider && (
          <div className="global-alert info" role="status">
            <Icon name="sparkles" size={17} />
            <span>
              <strong>Local demo diagnostics</strong>Questions use built-in
              sample evidence, so no AI key is needed.
            </span>
          </div>
        )}

        {dockerUnavailable && (
          <div className="global-alert warning" role="alert">
            <Icon name="alert" size={17} />
            <span>
              <strong>Docker is unavailable</strong>
              {health?.docker.message ??
                "Check the Docker socket connection and refresh."}
            </span>
          </div>
        )}

        <div
          className={`conversation ${messages.length === 0 ? "is-empty" : ""}`}
        >
          {messages.length === 0 ? (
            <EmptyState
              disabled={inputDisabled}
              onPrompt={(prompt) => void submitMessage(prompt)}
              selected={selected}
            />
          ) : (
            <div className="message-list" aria-live="polite">
              {messages.map((message) => {
                if (message.role === "user") {
                  return (
                    <article className="message user-message" key={message.id}>
                      <div className="message-content">
                        {message.containerName && (
                          <span className="message-context">
                            {message.containerName}
                          </span>
                        )}
                        <p>{message.content}</p>
                      </div>
                      <div className="user-avatar">You</div>
                    </article>
                  );
                }
                if (message.role === "error") {
                  return (
                    <article
                      className="message assistant-message error-message"
                      key={message.id}
                      role="alert"
                    >
                      <div className="assistant-avatar error">
                        <Icon name="alert" size={18} />
                      </div>
                      <div className="message-content">
                        <div className="message-meta">
                          <strong>Investigation stopped</strong>
                        </div>
                        <p>{message.content}</p>
                        <button
                          onClick={() =>
                            void submitMessage(
                              message.originalMessage,
                              message.containerId,
                              message.containerId === selectedId
                                ? conversationId
                                : undefined,
                            )
                          }
                          disabled={sending}
                        >
                          <Icon name="refresh" size={14} /> Try again
                        </button>
                      </div>
                    </article>
                  );
                }
                return <AssistantMessage key={message.id} message={message} />;
              })}
              {sending && (
                <InvestigationLoader
                  selectedName={selected?.name}
                  step={loaderStep}
                />
              )}
              <div ref={endRef} />
            </div>
          )}
        </div>

        <div className="composer-wrap">
          <form className="composer" onSubmit={onSubmit}>
            {selected && (
              <div className="composer-context">
                <span className={`state-dot ${stateTone(selected)}`} />
                <span>{selected.name}</span>
                <button
                  type="button"
                  aria-label="Clear selected container"
                  onClick={() => selectScope(undefined)}
                >
                  <Icon name="close" size={12} />
                </button>
              </div>
            )}
            <div className="composer-row">
              <textarea
                aria-label="Ask Dopsy"
                disabled={inputDisabled}
                onChange={(event) => setInput(event.target.value)}
                onKeyDown={onTextareaKeyDown}
                placeholder={
                  aiUnavailable
                    ? "Configure an AI provider to begin…"
                    : dockerUnavailable
                      ? "Connect Docker to begin…"
                      : "Ask what happened in your containers…"
                }
                ref={textareaRef}
                rows={1}
                value={input}
              />
              <button
                className="send-button"
                type="submit"
                disabled={inputDisabled || !input.trim()}
                aria-label="Send question"
              >
                {sending ? (
                  <span className="button-spinner" />
                ) : (
                  <Icon name="send" size={18} />
                )}
              </button>
            </div>
            <div className="composer-footer">
              <span>Enter to send · Shift + Enter for a new line</span>
              <span>
                <Icon name="shield" size={12} /> Dopsy cannot modify your
                containers
              </span>
            </div>
          </form>
          <p className="disclaimer privacy-disclaimer">
            <Icon name="shield" size={12} />
            {demoProvider
              ? "Local demo: no AI provider is contacted."
              : "Privacy: bounded log excerpts can contain sensitive data and are sent to your configured AI provider."}
          </p>
        </div>
      </main>
    </div>
  );
}
