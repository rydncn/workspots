// Package mail composes the messages the app would send and hands them to a
// Sender. No provider is chosen yet (ADR 0009): development writes each
// message to its own plain-text stream, production logs only the kind.
package mail

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// Message is one email. Kind is a fixed name for logs ("welcome"); To, Subject
// and Body are personal data and never reach the structured log.
type Message struct {
	Kind, To, Subject, Body string
}

// Sender delivers messages.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Outbox is the development sender: the whole message, plain text, to w,
// separate from the structured log so addresses never land in it.
type Outbox struct {
	mu sync.Mutex
	W  io.Writer
}

func (o *Outbox) Send(_ context.Context, m Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, err := fmt.Fprintf(o.W, "DEV MAIL (%s)\nTo: %s\nSubject: %s\n\n%s\n\n", m.Kind, m.To, m.Subject, m.Body)
	return err
}

// Logging is the production sender until a provider is chosen: it records
// that a message of this kind would have been sent, and nothing personal.
type Logging struct{ Logger *slog.Logger }

func (l Logging) Send(ctx context.Context, m Message) error {
	l.Logger.WarnContext(ctx, "mail not sent: no provider configured", "kind", m.Kind)
	return nil
}

// Recorder is the test fake: it keeps every message.
type Recorder struct {
	mu   sync.Mutex
	sent []Message
}

func (r *Recorder) Send(_ context.Context, m Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return nil
}

// Sent returns a copy of every message so far.
func (r *Recorder) Sent() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.sent...)
}
