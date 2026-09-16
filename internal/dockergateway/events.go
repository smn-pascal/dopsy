package dockergateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/smn-pascal/dopsy/internal/domain"
)

const (
	MaxEvents             = 200
	MaxEventBytes         = 256 * 1024
	eventsTruncatedHeader = "X-Dopsy-Events-Truncated"
)

var eventFilters = []string{"start", "stop", "die", "oom", "restart", "kill", "health_status"}

func validEventWindow(options domain.EventOptions, now int64) bool {
	return options.Since > 0 && options.Until > options.Since &&
		options.Until < now && options.Until-options.Since <= maxLogWindowSeconds
}

func validContainerEventsQuery(query url.Values, now int64) bool {
	if len(query) != 3 || len(query["filters"]) != 1 {
		return false
	}
	since, hasSince, sinceOK := optionalUnixTimestamp(query, "since")
	until, hasUntil, untilOK := optionalUnixTimestamp(query, "until")
	if !hasSince || !hasUntil || !sinceOK || !untilOK || !validEventWindow(domain.EventOptions{Since: since, Until: until}, now) {
		return false
	}
	_, ok := decodeEventFilters(query.Get("filters"))
	return ok
}

func decodeEventFilters(value string) (string, bool) {
	if len(value) > 2048 {
		return "", false
	}
	var filters map[string][]string
	if json.Unmarshal([]byte(value), &filters) != nil || len(filters) != 3 ||
		len(filters["container"]) != 1 || !validContainerID(filters["container"][0]) ||
		len(filters["type"]) != 1 || filters["type"][0] != "container" ||
		len(filters["event"]) != len(eventFilters) {
		return "", false
	}
	seen := make(map[string]bool)
	for _, action := range filters["event"] {
		if !oneOf(action, eventFilters...) || seen[action] {
			return "", false
		}
		seen[action] = true
	}
	return filters["container"][0], true
}

type rawEvent struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		ID string `json:"ID"`
	} `json:"Actor"`
	Time int64 `json:"time"`
}

func allowedEventAction(action string) bool {
	return oneOf(action, "start", "stop", "die", "oom", "restart", "kill",
		"health_status: healthy", "health_status: unhealthy", "health_status: starting")
}

// readEvents bounds the entire response before decoding. Unknown fields are
// never retained; records outside the requested container/window are ignored.
func readEvents(reader io.Reader, id string, options domain.EventOptions) ([]rawEvent, bool, error) {
	body, err := io.ReadAll(io.LimitReader(reader, MaxEventBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(body) > MaxEventBytes {
		return nil, false, ErrResponseTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	events := make([]rawEvent, 0)
	truncated := false
	for {
		var event rawEvent
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return nil, false, fmt.Errorf("invalid Docker event response")
		}
		if event.Type != "container" || !validContainerID(event.Actor.ID) ||
			event.Actor.ID != id ||
			!allowedEventAction(event.Action) || event.Time < options.Since || event.Time > options.Until {
			continue
		}
		if len(events) == MaxEvents {
			truncated = true
			continue
		}
		events = append(events, event)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time < events[j].Time })
	return events, truncated, nil
}

func (e *Engine) ContainerEvents(ctx context.Context, id string, options domain.EventOptions) (domain.Events, error) {
	if !validContainerID(id) {
		return domain.Events{}, ErrInvalidContainer
	}
	if !validEventWindow(options, time.Now().Unix()) {
		return domain.Events{}, fmt.Errorf("invalid event window")
	}
	// Resolve names and short IDs without exposing actor attributes from events.
	inspection, err := e.InspectContainer(ctx, id)
	if err != nil {
		return domain.Events{}, err
	}
	if !validContainerID(inspection.ID) {
		return domain.Events{}, ErrInvalidContainer
	}
	filters, _ := json.Marshal(map[string][]string{
		"container": {inspection.ID}, "type": {"container"}, "event": eventFilters,
	})
	query := url.Values{"since": {strconv.FormatInt(options.Since, 10)}, "until": {strconv.FormatInt(options.Until, 10)}, "filters": {string(filters)}}
	response, err := e.openGET(ctx, "/events?"+query.Encode())
	if err != nil {
		return domain.Events{}, err
	}
	defer response.Body.Close()
	raw, truncated, err := readEvents(response.Body, inspection.ID, options)
	if err != nil {
		return domain.Events{}, err
	}
	result := domain.Events{Items: make([]domain.ContainerEvent, 0, len(raw)), Since: options.Since, Until: options.Until, Truncated: truncated || response.Header.Get(eventsTruncatedHeader) == "true", HistoryLimited: true}
	for _, event := range raw {
		result.Items = append(result.Items, domain.ContainerEvent{Action: event.Action, Time: event.Time})
	}
	return result, nil
}
