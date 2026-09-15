package domain

// Overview is a bounded, point-in-time view of the containers visible to Dopsy.
// It contains only deterministic Docker facts and never includes logs or model
// output.
type Overview struct {
	GeneratedAt int64               `json:"generatedAt"`
	Summary     OverviewSummary     `json:"summary"`
	Collection  OverviewCollection  `json:"collection"`
	Containers  []ContainerOverview `json:"containers"`
}

type OverviewSummary struct {
	Total       int `json:"total"`
	Running     int `json:"running"`
	Healthy     int `json:"healthy"`
	NeedsReview int `json:"needsReview"`
}

type OverviewCollection struct {
	Observed  int  `json:"observed"`
	Total     int  `json:"total"`
	Partial   bool `json:"partial"`
	Truncated bool `json:"truncated"`
}

type ContainerOverview struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Image      string              `json:"image"`
	State      string              `json:"state"`
	Status     string              `json:"status"`
	Health     *string             `json:"health,omitempty"`
	Created    int64               `json:"created"`
	Details    *ContainerDetails   `json:"details,omitempty"`
	Metrics    *Stats              `json:"metrics,omitempty"`
	Assessment ContainerAssessment `json:"assessment"`
}

type ContainerDetails struct {
	Running      bool   `json:"running"`
	OOMKilled    bool   `json:"oomKilled"`
	ExitCode     int    `json:"exitCode"`
	RestartCount int    `json:"restartCount"`
	StartedAt    string `json:"startedAt,omitempty"`
	FinishedAt   string `json:"finishedAt,omitempty"`
}

type ContainerAssessment struct {
	Level   string `json:"level"`
	Summary string `json:"summary"`
}
