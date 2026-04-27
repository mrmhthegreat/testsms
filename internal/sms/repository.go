// internal/sms/repository.go — Redis-backed persistence for SMS messages.
package sms

import (
	"context"
	"fmt"

	"testsms/pkg/queue"
)

const keyPrefix = "sms:"

// Repository handles all Redis I/O for SMS state.
type Repository struct {
	q *queue.Client
}

// NewRepository creates a new SMS repository.
func NewRepository(q *queue.Client) *Repository {
	return &Repository{q: q}
}

// SaveStatus persists the status for a given message ID.
func (r *Repository) SaveStatus(ctx context.Context, id, status string) error {
	return r.q.Set(ctx, keyPrefix+id, status)
}

// GetStatus retrieves the current status for a message ID.
func (r *Repository) GetStatus(ctx context.Context, id string) (string, error) {
	val, err := r.q.Get(ctx, keyPrefix+id)
	if err != nil {
		return "", fmt.Errorf("sms: get status %s: %w", id, err)
	}
	return val, nil
}

// Enqueue pushes the message ID onto the SMS processing queue.
func (r *Repository) Enqueue(ctx context.Context, id string) error {
	return r.q.Enqueue(ctx, queue.SMSQueue, id)
}

// Dequeue blocks until a message ID is available and returns it.
func (r *Repository) Dequeue(ctx context.Context) (string, error) {
	return r.q.Dequeue(ctx, queue.SMSQueue, 0) // 0 = block forever
}
