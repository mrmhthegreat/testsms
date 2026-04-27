// internal/sms/worker.go — Background goroutine for processing the SMS queue.
package sms

import (
	"context"
	"log"
	"time"
)

// Worker processes jobs from the SMS queue and simulates delivery.
type Worker struct {
	repo *Repository
}

// NewWorker creates a new SMS worker.
func NewWorker(repo *Repository) *Worker {
	return &Worker{repo: repo}
}

// Run starts the worker loop. It should be called in a goroutine.
// Use a cancellable context to stop the worker gracefully.
func (w *Worker) Run(ctx context.Context) {
	log.Println("[worker] SMS worker started")

	for {
		// Dequeue blocks until a job is available (or ctx is cancelled).
		id, err := w.repo.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("[worker] shutting down")
				return
			}
			log.Printf("[worker] dequeue error: %v — retrying in 2s\n", err)
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("[worker] processing job: %s\n", id)

		// Step 1: Queued → Sending
		time.Sleep(1 * time.Second)
		if err := w.repo.SaveStatus(ctx, id, "Sending"); err != nil {
			log.Printf("[worker] failed to set Sending for %s: %v\n", id, err)
			continue
		}
		log.Printf("[worker] %s → Sending\n", id)

		// Step 2: Sending → Delivered
		time.Sleep(2 * time.Second)
		if err := w.repo.SaveStatus(ctx, id, "Delivered"); err != nil {
			log.Printf("[worker] failed to set Delivered for %s: %v\n", id, err)
			continue
		}
		log.Printf("[worker] %s → Delivered\n", id)
	}
}
