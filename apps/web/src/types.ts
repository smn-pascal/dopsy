export type HealthResponse = {
  status: "ok";
  docker: {
    connected: boolean;
    mode: "live" | "demo";
    message?: string;
  };
  ai: {
    configured: boolean;
    model?: string;
  };
};

export type Container = {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  health?: string | null;
  created: number;
};

export type AssessmentLevel = "ok" | "warning" | "critical" | "unknown";

export type ContainerOverview = Container & {
  details?: {
    running: boolean;
    oomKilled: boolean;
    exitCode: number;
    restartCount: number;
    startedAt?: string;
    finishedAt?: string;
  };
  metrics?: {
    cpuPercent: number;
    memoryUsageBytes: number;
    memoryLimitBytes: number;
    memoryPercent: number;
    readAt: number;
  };
  assessment: {
    level: AssessmentLevel;
    summary: string;
  };
};

export type OverviewResponse = {
  generatedAt: number;
  summary: {
    total: number;
    running: number;
    healthy: number;
    needsReview: number;
  };
  collection: {
    observed: number;
    total: number;
    partial: boolean;
    truncated: boolean;
  };
  containers: ContainerOverview[];
};

export type Evidence = {
  label: string;
  value: string;
  severity?: "info" | "warning" | "critical";
};

export type InvestigationStep = {
  tool: string;
  summary: string;
};

export type ChatResponse = {
  conversationId: string;
  answer: string;
  evidence: Evidence[];
  steps: InvestigationStep[];
};

export type ChatMessage =
  | {
      id: string;
      role: "user";
      content: string;
      containerName?: string;
    }
  | {
      id: string;
      role: "assistant";
      content: string;
      evidence: Evidence[];
      steps: InvestigationStep[];
    }
  | {
      id: string;
      role: "error";
      content: string;
      originalMessage: string;
      containerId?: string;
      retryWithoutContext?: boolean;
    };
