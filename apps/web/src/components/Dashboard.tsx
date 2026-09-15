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
  return `${new Intl.NumberFormat("de-DE", { maximumFractionDigits: 1, minimumFractionDigits: 1 }).format(Math.max(0, value))} %`;
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

function Memory({ container }: { container: ContainerOverview }) {
  const metrics = container.metrics;
  if (!metrics) return <span className="overview-unavailable">–</span>;
  const hasLimit = metrics.memoryLimitBytes > 0;
  const percent =
    hasLimit && Number.isFinite(metrics.memoryPercent)
      ? Math.max(0, Math.min(100, metrics.memoryPercent))
      : null;
  return (
    <div className="overview-memory">
      <strong>{formatBytes(metrics.memoryUsageBytes)}</strong>
      <small>
        {hasLimit
          ? `von ${formatBytes(metrics.memoryLimitBytes)}`
          : "ohne Limit"}
      </small>
      {percent !== null && (
        <span
          aria-label={`${container.name}: RAM ${formatPercent(metrics.memoryPercent)}`}
          aria-valuemax={100}
          aria-valuemin={0}
          aria-valuenow={Math.round(percent)}
          className="overview-meter"
          role="progressbar"
        >
          <span style={{ width: `${percent}%` }} />
        </span>
      )}
    </div>
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

      <section className="overview-list" aria-labelledby="overview-list-title">
        <div className="overview-list-heading">
          <h2 id="overview-list-title">Container</h2>
          {incomplete && (
            <span role="status">
              Daten teilweise · {data.collection.observed}/
              {data.collection.total}
            </span>
          )}
        </div>
        {containers.length === 0 ? (
          <div className="overview-empty">Keine Container gefunden.</div>
        ) : (
          <table className="overview-table">
            <thead>
              <tr>
                <th scope="col">Container</th>
                <th scope="col">CPU</th>
                <th scope="col">RAM</th>
                <th scope="col">
                  <span className="sr-only">Diagnose öffnen</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {containers.map((container) => (
                <tr key={container.id}>
                  <td>
                    <div className="overview-container">
                      <span
                        aria-hidden="true"
                        className={`assessment-mark ${container.assessment.level}`}
                      />
                      <div>
                        <strong>{container.name}</strong>
                        <small title={container.image}>{container.image}</small>
                      </div>
                      <span
                        className={`overview-state ${container.assessment.level}`}
                      >
                        {stateLabel(container)}
                      </span>
                      {signalLabel(container) && (
                        <span className="overview-signal">
                          {signalLabel(container)}
                        </span>
                      )}
                    </div>
                  </td>
                  <td data-label="CPU">
                    {container.metrics ? (
                      formatPercent(container.metrics.cpuPercent)
                    ) : (
                      <span className="overview-unavailable">–</span>
                    )}
                  </td>
                  <td data-label="RAM">
                    <Memory container={container} />
                  </td>
                  <td className="overview-action">
                    <button
                      aria-label={`${container.name} untersuchen`}
                      onClick={() => onInvestigate(container.id)}
                    >
                      Untersuchen <span aria-hidden="true">→</span>
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </section>
  );
}
