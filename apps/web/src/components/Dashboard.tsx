import { useMemo } from "react";
import type {
  AssessmentLevel,
  ContainerOverview,
  OverviewResponse,
} from "../types";

type DashboardProps = {
  data: OverviewResponse | null;
  error: string | null;
  loading: boolean;
  onInvestigate: (containerId: string) => void;
  onRefresh: () => void;
};

const priority: Record<AssessmentLevel, number> = {
  critical: 0,
  warning: 1,
  unknown: 2,
  ok: 3,
};

function formatTime(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "Zeit unbekannt";
  return new Intl.DateTimeFormat("de-DE", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(seconds * 1000));
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes < 0) return "–";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${new Intl.NumberFormat("de-DE", { maximumFractionDigits: value >= 100 || unit === 0 ? 0 : 1 }).format(value)} ${units[unit]}`;
}

function formatPercent(value: number) {
  if (!Number.isFinite(value)) return "–";
  const safe = Math.max(0, value);
  const options: Intl.NumberFormatOptions =
    safe > 0 && safe < 0.1
      ? { maximumSignificantDigits: 2 }
      : { maximumFractionDigits: 1, minimumFractionDigits: 1 };
  return `${new Intl.NumberFormat("de-DE", options).format(safe)} %`;
}

function stateLabel(container: ContainerOverview) {
  const health = container.health?.toLowerCase();
  const state = container.state.toLowerCase();
  if (state === "running") {
    if (health === "unhealthy") return "Ungesund";
    if (health === "healthy") return "Gesund";
    if (health === "starting") return "Startet";
  }
  switch (state) {
    case "running":
      return "Läuft";
    case "exited":
      return "Beendet";
    case "restarting":
      return "Neustart";
    case "paused":
      return "Pausiert";
    case "dead":
      return "Fehler";
    case "created":
      return "Erstellt";
    default:
      return container.state || "Unbekannt";
  }
}

function signalLabel(container: ContainerOverview) {
  if (container.details?.oomKilled) return "Speichermangel";
  if (container.assessment.level === "critical") return "Kritisch";
  if (container.assessment.level === "warning") return "Prüfen";
  if (container.assessment.level === "unknown") return "Unklar";
  return null;
}

function chartCeiling(value: number) {
  if (!Number.isFinite(value) || value <= 0) return 1;
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const normalized = value / magnitude;
  const step =
    normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
  return Math.max(1, step * magnitude);
}

function CpuChart({ containers }: { containers: ContainerOverview[] }) {
  const measured = containers
    .filter(
      (container) =>
        container.metrics && Number.isFinite(container.metrics.cpuPercent),
    )
    .sort((a, b) => (b.metrics?.cpuPercent ?? 0) - (a.metrics?.cpuPercent ?? 0))
    .slice(0, 5);
  const scale = chartCeiling(
    Math.max(
      0,
      ...measured.map((container) => container.metrics?.cpuPercent ?? 0),
    ),
  );

  return (
    <section
      className="dashboard-panel cpu-panel"
      aria-labelledby="cpu-chart-title"
    >
      <div className="panel-heading">
        <div>
          <span className="panel-eyebrow">Momentaufnahme</span>
          <h2 id="cpu-chart-title">CPU-Auslastung</h2>
        </div>
        <span className="chart-scale">Skala 0–{formatPercent(scale)}</span>
      </div>
      {measured.length === 0 ? (
        <div className="chart-empty">Keine aktuellen CPU-Werte.</div>
      ) : (
        <figure
          className="cpu-chart"
          aria-label="Aktuelle CPU-Auslastung der beobachteten Container"
        >
          <div className="cpu-chart-grid" aria-hidden="true">
            <span />
            <span />
            <span />
          </div>
          <div className="cpu-chart-columns">
            {measured.map((container) => {
              const value = container.metrics?.cpuPercent ?? 0;
              return (
                <div className="cpu-chart-column" key={container.id}>
                  <div className="cpu-chart-track">
                    <span
                      className="cpu-chart-bar"
                      style={{
                        height: `${Math.max(0, Math.min(100, (value / scale) * 100))}%`,
                      }}
                    />
                  </div>
                  <strong title={container.name}>{container.name}</strong>
                  <small>{formatPercent(value)}</small>
                </div>
              );
            })}
          </div>
        </figure>
      )}
      <p className="panel-footnote">Nur aktuelle Messwerte · kein Verlauf</p>
    </section>
  );
}

function StateChart({
  data,
  containers,
}: {
  data: OverviewResponse;
  containers: ContainerOverview[];
}) {
  const total = Math.max(0, data.summary.total);
  const running = Math.max(0, Math.min(total, data.summary.running));
  const notRunning = total - running;
  const runningShare = total > 0 ? (running / total) * 100 : 0;
  const counts: Record<AssessmentLevel, number> = {
    critical: 0,
    warning: 0,
    unknown: 0,
    ok: 0,
  };
  for (const container of containers) counts[container.assessment.level] += 1;
  const assessed = containers.length;

  return (
    <section
      className="dashboard-panel state-panel"
      aria-labelledby="state-chart-title"
    >
      <div className="panel-heading">
        <div>
          <span className="panel-eyebrow">Alle Container</span>
          <h2 id="state-chart-title">Zustand</h2>
        </div>
      </div>
      <div className="state-ring-wrap">
        <svg
          className="state-ring"
          viewBox="0 0 200 200"
          role="img"
          aria-label={`${running} von ${total} Containern laufen`}
        >
          <circle
            className="state-ring-base"
            cx="100"
            cy="100"
            r="78"
            pathLength="100"
          />
          {running > 0 && (
            <circle
              className="state-ring-value"
              cx="100"
              cy="100"
              r="78"
              pathLength="100"
              strokeDasharray={`${runningShare} 100`}
            />
          )}
        </svg>
        <div className="state-ring-center">
          <strong>{total > 0 ? `${Math.round(runningShare)} %` : "–"}</strong>
          <span>laufen</span>
        </div>
      </div>
      <div className="state-breakdown">
        <span>
          <i className="running" /> Laufen <strong>{running}</strong>
        </span>
        <span>
          <i className="not-running" /> Nicht laufend{" "}
          <strong>{notRunning}</strong>
        </span>
      </div>
      <div className="assessment-chart">
        <div className="assessment-heading">
          <span>Einordnung</span>
          <small>{assessed} beobachtet</small>
        </div>
        <div
          className="assessment-track"
          role="img"
          aria-label={`Beobachtet: ${counts.critical} kritisch, ${counts.warning} prüfen, ${counts.unknown} unklar, ${counts.ok} ohne Warnsignal`}
        >
          {(["critical", "warning", "unknown", "ok"] as const).map((level) =>
            counts[level] > 0 ? (
              <span
                className={level}
                key={level}
                style={{ width: `${(counts[level] / assessed) * 100}%` }}
              />
            ) : null,
          )}
        </div>
        <div className="assessment-legend">
          <span>
            <i className="critical" />
            {counts.critical} kritisch
          </span>
          <span>
            <i className="warning" />
            {counts.warning} prüfen
          </span>
          <span>
            <i className="unknown" />
            {counts.unknown} unklar
          </span>
          <span>
            <i className="ok" />
            {counts.ok} ohne Warnsignal
          </span>
        </div>
      </div>
    </section>
  );
}

function ContainerCard({
  container,
  onInvestigate,
}: {
  container: ContainerOverview;
  onInvestigate: (id: string) => void;
}) {
  const metrics = container.metrics;
  const hasMemoryLimit = !!metrics && metrics.memoryLimitBytes > 0;
  const memoryShare =
    hasMemoryLimit && Number.isFinite(metrics.memoryPercent)
      ? Math.max(0, Math.min(100, metrics.memoryPercent))
      : null;
  const signal = signalLabel(container);

  return (
    <article className={`container-card ${container.assessment.level}`}>
      <div className="container-card-heading">
        <span
          className={`card-state-dot ${container.assessment.level}`}
          aria-hidden="true"
        />
        <div>
          <h3 title={container.name}>{container.name}</h3>
          <small title={container.image}>{container.image}</small>
        </div>
        <span className="card-state-label">{stateLabel(container)}</span>
      </div>
      <div className="container-card-metrics">
        <div>
          <span>CPU</span>
          <strong>{metrics ? formatPercent(metrics.cpuPercent) : "–"}</strong>
        </div>
        <div>
          <span>RAM</span>
          <strong>
            {metrics ? formatBytes(metrics.memoryUsageBytes) : "–"}
          </strong>
          {metrics && (
            <small>
              {hasMemoryLimit
                ? `von ${formatBytes(metrics.memoryLimitBytes)}`
                : "ohne Limit"}
            </small>
          )}
        </div>
      </div>
      {memoryShare !== null && (
        <span
          className="card-memory-meter"
          role="progressbar"
          aria-label={`${container.name}: RAM ${formatPercent(metrics!.memoryPercent)}`}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(memoryShare)}
        >
          <span style={{ width: `${memoryShare}%` }} />
        </span>
      )}
      <div className="container-card-footer">
        <span
          className={
            signal ? `card-signal ${container.assessment.level}` : "card-signal"
          }
        >
          {signal ?? "Ohne Warnsignal"}
        </span>
        <button
          aria-label={`${container.name} untersuchen`}
          onClick={() => onInvestigate(container.id)}
        >
          Untersuchen <span aria-hidden="true">→</span>
        </button>
      </div>
    </article>
  );
}

export default function Dashboard({
  data,
  error,
  loading,
  onInvestigate,
  onRefresh,
}: DashboardProps) {
  const containers = useMemo(
    () =>
      !data
        ? []
        : [...data.containers].sort(
            (a, b) =>
              priority[a.assessment.level] - priority[b.assessment.level] ||
              a.name.localeCompare(b.name, "de"),
          ),
    [data],
  );

  if (loading && !data)
    return (
      <section
        className="dashboard"
        aria-busy="true"
        aria-label="Übersicht lädt"
      >
        <div className="dashboard-skeleton" />
      </section>
    );
  if (!data)
    return (
      <section className="dashboard dashboard-failure" role="alert">
        <h1>Übersicht nicht verfügbar</h1>
        <p>{error ?? "Docker-Daten konnten nicht geladen werden."}</p>
        <button onClick={onRefresh}>Erneut laden</button>
      </section>
    );

  const timestamp =
    Number.isFinite(data.generatedAt) && data.generatedAt > 0
      ? new Date(data.generatedAt * 1000).toISOString()
      : undefined;
  const incomplete = data.collection.partial || data.collection.truncated;

  return (
    <section
      className="dashboard"
      aria-busy={loading}
      aria-labelledby="dashboard-title"
    >
      <header className="dashboard-header">
        <div>
          <h1 id="dashboard-title">Übersicht</h1>
          <p>
            Momentaufnahme ·{" "}
            {timestamp ? (
              <time dateTime={timestamp}>{formatTime(data.generatedAt)}</time>
            ) : (
              "Zeit unbekannt"
            )}
          </p>
        </div>
        <button
          className="dashboard-refresh"
          disabled={loading}
          onClick={onRefresh}
        >
          <svg
            aria-hidden="true"
            fill="none"
            height="16"
            viewBox="0 0 24 24"
            width="16"
          >
            <path
              d="M20 7v5h-5M18.1 16A7 7 0 1 1 19 8l1 4"
              stroke="currentColor"
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="1.8"
            />
          </svg>
          {loading ? "Lädt…" : "Aktualisieren"}
        </button>
      </header>

      {error && (
        <p className="dashboard-notice" role="status">
          Aktualisierung fehlgeschlagen. Letzter Stand wird angezeigt.
        </p>
      )}
      {incomplete && (
        <p className="dashboard-coverage" role="status">
          Daten teilweise · {data.collection.observed}/{data.collection.total}{" "}
          beobachtet
        </p>
      )}

      <dl className="overview-numbers" aria-label="Containerstatus">
        <div>
          <dt>Gesamt</dt>
          <dd>{data.summary.total}</dd>
        </div>
        <div>
          <dt>Laufen</dt>
          <dd>{data.summary.running}</dd>
        </div>
        <div>
          <dt>Gesund</dt>
          <dd>{data.summary.healthy}</dd>
        </div>
        <div className={data.summary.needsReview > 0 ? "attention" : ""}>
          <dt>Prüfen</dt>
          <dd>{data.summary.needsReview}</dd>
        </div>
      </dl>

      <div className="dashboard-charts">
        <CpuChart containers={containers} />
        <StateChart data={data} containers={containers} />
      </div>

      <section
        className="dashboard-containers"
        aria-labelledby="dashboard-containers-title"
      >
        <div className="container-grid-heading">
          <h2 id="dashboard-containers-title">Container</h2>
          <span>{data.collection.observed} beobachtet</span>
        </div>
        {containers.length === 0 ? (
          <div className="overview-empty">Keine Container gefunden.</div>
        ) : (
          <div className="container-card-grid">
            {containers.map((container) => (
              <ContainerCard
                container={container}
                key={container.id}
                onInvestigate={onInvestigate}
              />
            ))}
          </div>
        )}
      </section>
    </section>
  );
}
