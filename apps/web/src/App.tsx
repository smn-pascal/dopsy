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
    title: "Resource pressure",
    prompt: "Which container is using the most resources, and why?",
  },
  {
    icon: "history",
    title: "Unexpected restart",
    prompt: "Why did this container restart, and what should I check next?",
  },
  {
    icon: "logs",
    title: "Recent errors",
    prompt: "Are there any important errors in the recent logs?",
  },
] as const;

const loadingSteps = [
  "Reading container state",
  "Checking bounded evidence",
  "Preparing the diagnosis",
];

function Icon({ name, size = 18 }: { name: string; size?: number }) {
  const paths: Record<string, ReactNode> = {
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
    <div className="brand" aria-label="Dopsy">
      <img className="brand-logo" src="/brand/dopsy-logo.svg" alt="Dopsy" />
      <span className="version">v0.1.1</span>
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

function EvidenceGrid({
  evidence,
  className = "",
}: {
  evidence: Evidence[];
  className?: string;
}) {
  if (evidence.length === 0) return null;
  return (
    <section className={`evidence ${className}`} aria-label="Evidence">
      <h3>Evidence</h3>
      <dl>
        {evidence.map((item, index) => (
          <div
            className={`evidence-row ${item.severity ?? "info"}`}
            key={`${item.label}-${index}`}
          >
            <dt>{item.label}</dt>
            <dd>{item.value}</dd>
            <span className="sr-only">Severity: {item.severity ?? "info"}</span>
          </div>
        ))}
      </dl>
    </section>
  );
}

function EvidenceInspector({
  evidence,
  hasConversation,
}: {
  evidence: Evidence[];
  hasConversation: boolean;
}) {
  return (
    <aside className="evidence-inspector" aria-label="Evidence inspector">
      <div className="inspector-header">
        <span className="inspector-kicker">Evidence inspector</span>
        <h2>What supports the diagnosis</h2>
        <p>Bounded facts attached to the most recent diagnosis.</p>
      </div>

      {evidence.length > 0 ? (
        <EvidenceGrid evidence={evidence} className="inspector-evidence" />
      ) : (
        <div className="inspector-empty">
          <span className="inspector-rule" aria-hidden="true" />
          <strong>
            {hasConversation ? "No evidence attached" : "Ready for evidence"}
          </strong>
          <p>
            {hasConversation
              ? "The latest response did not include structured evidence."
              : "Supporting facts will appear here after an investigation."}
          </p>
        </div>
      )}

      <div className="inspector-footer">
        <Icon name="shield" size={14} />
        Read-only, bounded collection
      </div>
    </aside>
  );
}

function ToolSteps({ steps }: { steps: InvestigationStep[] }) {
  const [expanded, setExpanded] = useState(false);
  if (steps.length === 0) return null;
  return (
    <section className="tool-steps" aria-label="Investigation trace">
      <button
        aria-expanded={expanded}
        onClick={() => setExpanded((value) => !value)}
      >
        <span>
          Investigation trace <b>{steps.length}</b>
        </span>
        <span className={`disclosure ${expanded ? "expanded" : ""}`}>
          <Icon name="chevron" size={14} />
        </span>
      </button>
      {expanded && (
        <ol>
          {steps.map((step, index) => (
            <li key={`${step.tool}-${index}`}>
              <span className="step-index">{index + 1}</span>
              <div>
                <code>{step.tool}</code>
                <p>{step.summary}</p>
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
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
        <img src="/brand/dopsy-mark.svg" alt="" />
      </div>
      <div className="message-content">
        <div className="message-meta">
          <strong>Dopsy</strong>
          <span>Diagnosis</span>
        </div>
        <p className="answer-text">{message.content}</p>
        <EvidenceGrid evidence={message.evidence} className="inline-evidence" />
        <ToolSteps steps={message.steps} />
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
        <img src="/brand/dopsy-mark.svg" alt="" />
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
      <div className="empty-heading">
        <img src="/brand/dopsy-mark.svg" alt="" />
        <span>New investigation</span>
      </div>
      <h1 id="welcome-title">What should we inspect?</h1>
      <p>
        Ask about a failure, slowdown, or unusual container state. Dopsy reads
        only the bounded evidence needed to answer.
      </p>
      {selected && (
        <div className="selected-context">
          <span className={`state-dot ${stateTone(selected)}`} />
          Scope: <strong>{selected.name}</strong>
        </div>
      )}
      <h2>Common investigations</h2>
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
  const latestEvidence = useMemo(() => {
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      if (message.role === "assistant") {
        return message.evidence;
      }
    }
    return [];
  }, [messages]);

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
              <span className="mode-label">Demo data</span>
            )}
            <span className="access-label">
              <span className="status-dot online" /> Read-only
            </span>
            <a
              className="docs-link"
              href="https://smn-pascal.github.io/dopsy/"
              target="_blank"
              rel="noreferrer"
            >
              Documentation <Icon name="external" size={13} />
            </a>
          </div>
        </header>

        <div className="workspace-body">
          <section className="primary-pane" aria-label="Diagnostic workspace">
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
                <Icon name="shield" size={17} />
                <span>
                  <strong>Local demo diagnostics</strong>Built-in sample
                  evidence is used. No provider is contacted.
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
                        <article
                          className="message user-message"
                          key={message.id}
                        >
                          <div className="message-content">
                            <div className="message-meta">
                              <strong>You</strong>
                              <span>Question</span>
                            </div>
                            {message.containerName && (
                              <span className="message-context">
                                Scope: {message.containerName}
                              </span>
                            )}
                            <p>{message.content}</p>
                          </div>
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
                    return (
                      <AssistantMessage key={message.id} message={message} />
                    );
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
                    <Icon name="shield" size={12} /> Read-only by design
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
          </section>

          <EvidenceInspector
            evidence={latestEvidence}
            hasConversation={messages.length > 0}
          />
        </div>
      </main>
    </div>
  );
}
