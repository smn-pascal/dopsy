package session

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestStoreKeepsScopeAndBoundedHistory(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	store := New(Options{
		TTL:         30 * time.Minute,
		MaxSessions: 2,
		MaxTurns:    2,
		MaxBytes:    32,
		Now:         func() time.Time { return now },
	})

	created, err := store.Begin("", "container-a")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !created.Created || created.ContainerID != "container-a" {
		t.Fatalf("unexpected new conversation: %+v", created)
	}
	if err := store.Commit(created.ID, "first", "answer-one"); err != nil {
		t.Fatal(err)
	}

	followup, err := store.Begin(created.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if followup.ContainerID != "container-a" || len(followup.Turns) != 1 {
		t.Fatalf("scope or history was not retained: %+v", followup)
	}
	if err := store.Commit(created.ID, "second", "answer-two"); err != nil {
		t.Fatal(err)
	}

	_, err = store.Begin(created.ID, "container-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(created.ID, "third", strings.Repeat("é", 30)); err != nil {
		t.Fatal(err)
	}
	fourth, err := store.Begin(created.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Abort(created.ID, false)
	if len(fourth.Turns) > 2 {
		t.Fatalf("history contains %d turns, want at most 2", len(fourth.Turns))
	}
	bytes := 0
	for _, turn := range fourth.Turns {
		bytes += len(turn.User) + len(turn.Assistant)
		if !utf8.ValidString(turn.User) || !utf8.ValidString(turn.Assistant) {
			t.Fatalf("history was truncated to invalid UTF-8: %+v", turn)
		}
	}
	if bytes > 32 {
		t.Fatalf("history contains %d bytes, want at most 32", bytes)
	}
}

func TestStoreRejectsScopeMismatchAndConcurrentUse(t *testing.T) {
	store := New(Options{})
	created, err := store.Begin("", "container-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(created.ID, "question", "answer"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(created.ID, "container-b"); !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("scope error = %v, want %v", err, ErrScopeMismatch)
	}
	if _, err := store.Begin(created.ID, "container-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(created.ID, "container-a"); !errors.Is(err, ErrBusy) {
		t.Fatalf("busy error = %v, want %v", err, ErrBusy)
	}
	store.Abort(created.ID, false)
}

func TestStoreExpiresAndEvictsIdleConversations(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	store := New(Options{
		TTL:         5 * time.Minute,
		MaxSessions: 1,
		Now:         func() time.Time { return now },
	})
	first, err := store.Begin("", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(first.ID, "one", "one"); err != nil {
		t.Fatal(err)
	}

	second, err := store.Begin("", "")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("new conversation reused an existing random ID")
	}
	if err := store.Commit(second.ID, "two", "two"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Begin(first.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("evicted conversation error = %v, want %v", err, ErrNotFound)
	}

	now = now.Add(6 * time.Minute)
	if _, err := store.Begin(second.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired conversation error = %v, want %v", err, ErrNotFound)
	}
}
