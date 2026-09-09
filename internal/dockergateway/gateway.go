package dockergateway

import (
	"context"
	"errors"

	"github.com/smn-pascal/dopsy/internal/domain"
)

const (
	MaxLogTail  = 2000
	MaxLogBytes = 256 * 1024
)

var (
	ErrBlockedRequest   = errors.New("docker request blocked by read-only allowlist")
	ErrInvalidContainer = errors.New("invalid container identifier")
	ErrResponseTooLarge = errors.New("docker response exceeds size limit")
)

type Gateway interface {
	Mode() string
	Check(context.Context) error
	ListContainers(context.Context) ([]domain.Container, error)
	InspectContainer(context.Context, string) (domain.Inspection, error)
	ContainerLogs(context.Context, string, domain.LogOptions) (domain.Logs, error)
	ContainerStats(context.Context, string) (domain.Stats, error)
}
