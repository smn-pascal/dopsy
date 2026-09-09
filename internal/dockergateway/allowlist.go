package dockergateway

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	containerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	apiVersionPattern  = regexp.MustCompile(`^/v[0-9]+\.[0-9]+(/.*)$`)
	decimalPattern     = regexp.MustCompile(`^[0-9]+$`)
)

const maxLogWindowSeconds int64 = 24 * 60 * 60

type dockerRoute uint8

const (
	routeBlocked dockerRoute = iota
	routePing
	routeVersion
	routeContainerList
	routeContainerInspect
	routeContainerLogs
	routeContainerStats
)

// AllowlistTransport is a defense-in-depth boundary around the Docker API. It
// intentionally has no generic proxy escape hatch.
type AllowlistTransport struct {
	Next http.RoundTripper
}

func (t AllowlistTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if !AllowedDockerRequest(request) {
		return nil, ErrBlockedRequest
	}
	next := t.Next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(request)
}

// AllowedDockerRequest permits only the small set of read-only endpoints Dopsy
// understands. Similar-looking paths such as archive, export, top, exec, and
// images are deliberately rejected.
func AllowedDockerRequest(request *http.Request) bool {
	if request == nil || request.URL == nil {
		return false
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		return false
	}

	escaped := strings.ToLower(request.URL.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") {
		return false
	}
	path := normalizedDockerPath(request.URL.Path)
	if path == "" {
		return false
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return false
	}
	switch classifyDockerPath(path) {
	case routePing, routeVersion, routeContainerInspect:
		return len(query) == 0
	case routeContainerList:
		return validContainerListQuery(query)
	case routeContainerLogs:
		return validContainerLogsQuery(query, time.Now().Unix())
	case routeContainerStats:
		return validContainerStatsQuery(query)
	default:
		return false
	}
}

func normalizedDockerPath(path string) string {
	if match := apiVersionPattern.FindStringSubmatch(path); match != nil {
		path = match[1]
	}
	if path == "" || strings.Contains(path, "//") || strings.Contains(path, "/./") || strings.Contains(path, "/../") {
		return ""
	}
	return path
}

func classifyDockerPath(path string) dockerRoute {
	switch path {
	case "/_ping":
		return routePing
	case "/version":
		return routeVersion
	case "/containers/json":
		return routeContainerList
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 3 || parts[0] != "containers" || !validContainerID(parts[1]) {
		return routeBlocked
	}
	switch parts[2] {
	case "json":
		return routeContainerInspect
	case "logs":
		return routeContainerLogs
	case "stats":
		return routeContainerStats
	default:
		return routeBlocked
	}
}

func validContainerListQuery(query url.Values) bool {
	return len(query) == 1 && hasSingleValue(query, "all", "1", "true")
}

func validContainerStatsQuery(query url.Values) bool {
	return len(query) == 1 && hasSingleValue(query, "stream", "0", "false")
}

func validContainerLogsQuery(query url.Values, now int64) bool {
	allowedKeys := map[string]bool{
		"stdout": true, "stderr": true, "timestamps": true, "tail": true,
		"since": true, "until": true,
	}
	for key, values := range query {
		if !allowedKeys[key] || len(values) != 1 {
			return false
		}
	}
	if !hasSingleIntegerInRange(query, "tail", 1, MaxLogTail) {
		return false
	}
	for _, key := range []string{"stdout", "stderr"} {
		if values, present := query[key]; present && !oneOf(values[0], "0", "1", "false", "true") {
			return false
		}
	}
	if values, present := query["timestamps"]; present && values[0] != "1" {
		return false
	}

	since, hasSince, valid := optionalUnixTimestamp(query, "since")
	if !valid {
		return false
	}
	until, hasUntil, valid := optionalUnixTimestamp(query, "until")
	if !valid {
		return false
	}
	switch {
	case !hasSince && !hasUntil:
		return true
	case hasSince && hasUntil:
		return until >= since && until-since <= maxLogWindowSeconds
	case hasSince:
		return since <= now && now-since <= maxLogWindowSeconds
	default:
		// An until-only request ranges from the beginning of the log and is
		// therefore not a bounded time window.
		return false
	}
}

func hasSingleValue(query url.Values, key string, allowed ...string) bool {
	values, present := query[key]
	return present && len(values) == 1 && oneOf(values[0], allowed...)
}

func hasSingleIntegerInRange(query url.Values, key string, minimum, maximum int) bool {
	values, present := query[key]
	if !present || len(values) != 1 || !decimalPattern.MatchString(values[0]) {
		return false
	}
	value, err := strconv.Atoi(values[0])
	return err == nil && value >= minimum && value <= maximum
}

func optionalUnixTimestamp(query url.Values, key string) (int64, bool, bool) {
	values, present := query[key]
	if !present {
		return 0, false, true
	}
	if len(values) != 1 || !decimalPattern.MatchString(values[0]) {
		return 0, true, false
	}
	value, err := strconv.ParseInt(values[0], 10, 64)
	return value, true, err == nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validContainerID(id string) bool {
	return containerIDPattern.MatchString(id)
}

func unixTransport(socketPath string) *http.Transport {
	return &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 8 * time.Second,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
}
