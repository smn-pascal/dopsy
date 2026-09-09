package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound      = errors.New("conversation not found or expired")
	ErrScopeMismatch = errors.New("conversation container scope does not match")
	ErrBusy          = errors.New("conversation already has a diagnosis in progress")
	ErrCapacity      = errors.New("conversation store is at capacity")
)

const (
	defaultTTL         = 30 * time.Minute
	defaultMaxSessions = 128
	defaultMaxTurns    = 12
	defaultMaxBytes    = 128 * 1024
)

// Turn is the only conversation state retained between diagnoses. Tool calls
// and their potentially sensitive raw results are deliberately never stored.
type Turn struct {
	User      string
	Assistant string
}

type Snapshot struct {
	ID          string
	ContainerID string
	Turns       []Turn
	Created     bool
}

type Options struct {
	TTL         time.Duration
	MaxSessions int
	MaxTurns    int
	MaxBytes    int
	Now         func() time.Time
}

type Store struct {
	mu          sync.Mutex
	items       map[string]*conversation
	ttl         time.Duration
	maxSessions int
	maxTurns    int
	maxBytes    int
	now         func() time.Time
}

type conversation struct {
	id          string
	containerID string
	turns       []Turn
	bytes       int
	lastAccess  time.Time
	busy        bool
}

func New(options Options) *Store {
	ttl := options.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	maxSessions := options.MaxSessions
	if maxSessions <= 0 {
		maxSessions = defaultMaxSessions
	}
	maxTurns := options.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store{
		items:       make(map[string]*conversation),
		ttl:         ttl,
		maxSessions: maxSessions,
		maxTurns:    maxTurns,
		maxBytes:    maxBytes,
		now:         now,
	}
}

// Begin creates or locks a conversation for one diagnosis. An omitted scope on
// a follow-up inherits the original scope; a different explicit scope is
// rejected. The caller must eventually call Commit or Abort.
func (s *Store) Begin(id, requestedContainerID string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.removeExpired(now)
	if id != "" {
		item, ok := s.items[id]
		if !ok {
			return Snapshot{}, ErrNotFound
		}
		if requestedContainerID != "" && requestedContainerID != item.containerID {
			return Snapshot{}, ErrScopeMismatch
		}
		if item.busy {
			return Snapshot{}, ErrBusy
		}
		item.busy = true
		item.lastAccess = now
		return snapshot(item, false), nil
	}

	if len(s.items) >= s.maxSessions && !s.evictOldestIdle() {
		return Snapshot{}, ErrCapacity
	}
	generated, err := s.newID()
	if err != nil {
		return Snapshot{}, err
	}
	item := &conversation{
		id:          generated,
		containerID: requestedContainerID,
		lastAccess:  now,
		busy:        true,
	}
	s.items[generated] = item
	return snapshot(item, true), nil
}

// Commit appends the successful exchange and releases the per-conversation
// lock. Oldest turns are discarded until both configured bounds are met.
func (s *Store) Commit(id, user, assistant string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return ErrNotFound
	}

	turn := fitTurn(Turn{User: user, Assistant: assistant}, s.maxBytes)
	item.turns = append(item.turns, turn)
	item.bytes += turnBytes(turn)
	for len(item.turns) > s.maxTurns || item.bytes > s.maxBytes {
		item.bytes -= turnBytes(item.turns[0])
		item.turns = item.turns[1:]
	}
	item.busy = false
	item.lastAccess = s.now()
	return nil
}

// Abort releases a conversation after a failed diagnosis. Newly created empty
// conversations may be discarded so failed requests do not consume capacity.
func (s *Store) Abort(id string, discardIfEmpty bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return
	}
	if discardIfEmpty && len(item.turns) == 0 {
		delete(s.items, id)
		return
	}
	item.busy = false
	item.lastAccess = s.now()
}

func (s *Store) removeExpired(now time.Time) {
	for id, item := range s.items {
		if !item.busy && now.Sub(item.lastAccess) >= s.ttl {
			delete(s.items, id)
		}
	}
}

func (s *Store) evictOldestIdle() bool {
	var oldest *conversation
	for _, item := range s.items {
		if item.busy || (oldest != nil && !item.lastAccess.Before(oldest.lastAccess)) {
			continue
		}
		oldest = item
	}
	if oldest == nil {
		return false
	}
	delete(s.items, oldest.id)
	return true
}

func (s *Store) newID() (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		bytes := make([]byte, 24)
		if _, err := rand.Read(bytes); err != nil {
			return "", fmt.Errorf("generate conversation ID: %w", err)
		}
		id := base64.RawURLEncoding.EncodeToString(bytes)
		if _, exists := s.items[id]; !exists {
			return id, nil
		}
	}
	return "", fmt.Errorf("generate unique conversation ID")
}

func snapshot(item *conversation, created bool) Snapshot {
	turns := make([]Turn, len(item.turns))
	copy(turns, item.turns)
	return Snapshot{
		ID:          item.id,
		ContainerID: item.containerID,
		Turns:       turns,
		Created:     created,
	}
}

func fitTurn(turn Turn, limit int) Turn {
	if turnBytes(turn) <= limit {
		return turn
	}
	if len(turn.User) >= limit {
		return Turn{User: truncateUTF8(turn.User, limit)}
	}
	turn.Assistant = truncateUTF8(turn.Assistant, limit-len(turn.User))
	return turn
}

func turnBytes(turn Turn) int {
	return len(turn.User) + len(turn.Assistant)
}

func truncateUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
