// internal/sms/service.go — Core SMS business logic.
package sms

import (
	"context"
	"fmt"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Encoding describes the character encoding for an SMS message.
type Encoding string

const (
	EncodingGSM     Encoding = "GSM-7"
	EncodingUnicode Encoding = "Unicode"

	maxGSM     = 160
	maxUnicode = 70

	// Multi-part limits
	maxGSMPart     = 153
	maxUnicodePart = 67
)

// Message represents a single SMS job.
type Message struct {
	ID       string    `json:"id"`
	Phone    string    `json:"phone"`
	Body     string    `json:"body"`
	Encoding Encoding  `json:"encoding"`
	Segments int       `json:"segments"`
	Status   string    `json:"status"`
	SentAt   time.Time `json:"sent_at"`
}

// Service handles SMS creation and queuing.
type Service struct {
	repo *Repository
}

// NewService creates a new SMS service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// isUnicode reports whether the string contains any non-GSM-7 characters.
// We approximate GSM-7 as: printable ASCII (0x20–0x7E) plus common extended chars.
func isUnicode(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return true
		}
		// Characters outside GSM-7 basic set
		if r < 0x20 && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

// calculateSegments returns the encoding type and number of SMS segments.
func calculateSegments(body string) (Encoding, int) {
	length := len([]rune(body))

	if isUnicode(body) {
		if length <= maxUnicode {
			return EncodingUnicode, 1
		}
		return EncodingUnicode, (length + maxUnicodePart - 1) / maxUnicodePart
	}

	if length <= maxGSM {
		return EncodingGSM, 1
	}
	return EncodingGSM, (length + maxGSMPart - 1) / maxGSMPart
}

// Send validates, creates, and enqueues an SMS message.
func (s *Service) Send(ctx context.Context, phone, body string) (*Message, error) {
	if phone == "" || body == "" {
		return nil, fmt.Errorf("phone and message are required")
	}

	enc, segs := calculateSegments(body)
	msg := &Message{
		ID:       uuid.New().String(),
		Phone:    phone,
		Body:     body,
		Encoding: enc,
		Segments: segs,
		Status:   "Queued",
		SentAt:   time.Now(),
	}

	if err := s.repo.SaveStatus(ctx, msg.ID, msg.Status); err != nil {
		return nil, fmt.Errorf("sms: save status: %w", err)
	}
	if err := s.repo.Enqueue(ctx, msg.ID); err != nil {
		return nil, fmt.Errorf("sms: enqueue: %w", err)
	}

	return msg, nil
}

// GetStatus returns the current status of a message by ID.
func (s *Service) GetStatus(ctx context.Context, id string) (string, error) {
	return s.repo.GetStatus(ctx, id)
}
