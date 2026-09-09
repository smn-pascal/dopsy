package domain

// Container is the deliberately small, public view of a Docker container.
type Container struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Image   string  `json:"image"`
	State   string  `json:"state"`
	Status  string  `json:"status"`
	Health  *string `json:"health,omitempty"`
	Created int64   `json:"created"`
}

// Inspection contains only fields that are safe and useful for diagnostics.
// In particular, it can never contain environment variables, command arguments,
// arbitrary labels, or host-side mount paths.
type Inspection struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Image        string  `json:"image"`
	Status       string  `json:"status"`
	Running      bool    `json:"running"`
	OOMKilled    bool    `json:"oomKilled"`
	ExitCode     int     `json:"exitCode"`
	Error        string  `json:"error,omitempty"`
	StartedAt    string  `json:"startedAt,omitempty"`
	FinishedAt   string  `json:"finishedAt,omitempty"`
	Health       *string `json:"health,omitempty"`
	RestartCount int     `json:"restartCount"`
	MemoryLimit  int64   `json:"memoryLimitBytes,omitempty"`
}

type LogOptions struct {
	Tail  int
	Since int64
	Until int64
}

type Logs struct {
	Text      string `json:"text"`
	Tail      int    `json:"tail"`
	Truncated bool   `json:"truncated"`
}

type Stats struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryUsage   uint64  `json:"memoryUsageBytes"`
	MemoryLimit   uint64  `json:"memoryLimitBytes"`
	MemoryPercent float64 `json:"memoryPercent"`
	ReadAt        int64   `json:"readAt"`
}

type Evidence struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Severity string `json:"severity,omitempty"`
}

type Step struct {
	Tool    string `json:"tool"`
	Summary string `json:"summary"`
}

type Diagnosis struct {
	Answer   string     `json:"answer"`
	Evidence []Evidence `json:"evidence"`
	Steps    []Step     `json:"steps"`
}
