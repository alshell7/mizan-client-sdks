// Command webhook_receiver receives both Mizan webhook streams through net/http.
//
// It durably enqueues validated events before the SDK emits a 204 response (and,
// for ledger events, X-Mizan-Ack-Sequence). The file inbox is suitable for a
// single process. Replicated deployments should replace it with one shared,
// transactional database while preserving the same callback boundary.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	mizan "github.com/alshell7/mizan-client-sdks/mizan-go"
)

type inboxRecord struct {
	OutboxID         string          `json:"outbox_id"`
	Stream           string          `json:"stream"`
	EventID          string          `json:"event_id"`
	BusinessID       string          `json:"business_id"`
	BusinessSequence int64           `json:"business_sequence,omitempty"`
	Payload          json.RawMessage `json:"payload"`
	ReceivedAt       time.Time       `json:"received_at"`
}

type outboxAlias struct {
	Stream           string `json:"stream"`
	EventID          string `json:"event_id"`
	BusinessID       string `json:"business_id"`
	BusinessSequence int64  `json:"business_sequence,omitempty"`
	PayloadSHA256    string `json:"payload_sha256"`
}

type fileInbox struct {
	root string
	mu   sync.Mutex
}

func main() {
	token := os.Getenv("MIZAN_WEBHOOK_TOKEN")
	if token == "" {
		log.Fatal("MIZAN_WEBHOOK_TOKEN is required")
	}
	inbox := &fileInbox{root: envOr("MIZAN_WEBHOOK_INBOX", "./mizan-webhook-inbox")}
	receiver := mizan.WebhookReceiver{
		BearerToken: token,
		Handler: mizan.WebhookHandlerFuncs{
			Ledger: func(_ context.Context, event mizan.LedgerWebhook, delivery mizan.WebhookContext) error {
				// Persisting to this inbox is the application effect. A downstream
				// projector can process the durable record independently.
				return inbox.persist(inboxRecord{
					OutboxID: delivery.OutboxID, Stream: "ledger", EventID: event.EventID,
					BusinessID: event.BusinessID, BusinessSequence: event.BusinessSequence,
					Payload: append(json.RawMessage(nil), delivery.RawBody...), ReceivedAt: time.Now().UTC(),
				})
			},
			Notification: func(_ context.Context, event mizan.NotificationWebhook, delivery mizan.WebhookContext) error {
				return inbox.persist(inboxRecord{
					OutboxID: delivery.OutboxID, Stream: "notification", EventID: delivery.OutboxID,
					BusinessID: event.BusinessID,
					Payload:    append(json.RawMessage(nil), delivery.RawBody...), ReceivedAt: time.Now().UTC(),
				})
			},
		},
	}

	mux := http.NewServeMux()
	mux.Handle("POST /mizan/webhooks", receiver)
	server := &http.Server{
		Addr:              envOr("MIZAN_WEBHOOK_ADDR", ":8080"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("Mizan webhook receiver listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve webhooks: %v", err)
		}
	}()

	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-shutdown.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func (i *fileInbox) persist(record inboxRecord) error {
	if record.OutboxID == "" || record.Stream == "" || record.EventID == "" || record.BusinessID == "" {
		return errors.New("inbox record identity is incomplete")
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode inbox record: %w", err)
	}
	alias := outboxAlias{
		Stream: record.Stream, EventID: record.EventID, BusinessID: record.BusinessID,
		BusinessSequence: record.BusinessSequence, PayloadSHA256: digestBytes(record.Payload),
	}
	encodedAlias, err := json.Marshal(alias)
	if err != nil {
		return fmt.Errorf("encode outbox alias: %w", err)
	}

	// The mutex makes the two-index operation atomic for this single-process
	// example. Each individual file publication is atomic and fsynced as well.
	i.mu.Lock()
	defer i.mu.Unlock()

	aliasPath := filepath.Join(i.root, "outbox", digest(record.OutboxID)+".json")
	if existing, readErr := os.ReadFile(aliasPath); readErr == nil {
		if bytes.Equal(existing, encodedAlias) {
			return nil // Stable outbox retry: already durably accepted.
		}
		return errors.New("outbox ID was reused for a different event")
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read outbox index: %w", readErr)
	}

	eventPath := filepath.Join(i.root, "events", digest(record.Stream+":"+record.EventID)+".json")
	if existing, readErr := os.ReadFile(eventPath); readErr == nil {
		var original inboxRecord
		if err := json.Unmarshal(existing, &original); err != nil {
			return fmt.Errorf("decode existing inbox record: %w", err)
		}
		if original.Stream != record.Stream || original.EventID != record.EventID ||
			original.BusinessID != record.BusinessID || original.BusinessSequence != record.BusinessSequence ||
			!bytes.Equal(original.Payload, record.Payload) {
			return errors.New("event ID was reused for a different event")
		}
		return writeOnce(aliasPath, encodedAlias)
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return fmt.Errorf("read event index: %w", readErr)
	}

	if err := writeOnce(eventPath, encoded); err != nil {
		return fmt.Errorf("persist event: %w", err)
	}
	if err := writeOnce(aliasPath, encodedAlias); err != nil {
		return fmt.Errorf("persist outbox index: %w", err)
	}
	return nil
}

func writeOnce(path string, contents []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".mizan-inbox-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o640); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			existing, readErr := os.ReadFile(path)
			if readErr == nil && bytes.Equal(existing, contents) {
				return nil
			}
		}
		return err
	}
	// Windows does not support syncing a directory handle. The event file itself
	// was fsynced above; Linux/Unix deployments additionally sync the directory
	// entry to make publication durable across a sudden power loss.
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func digest(value string) string {
	return digestBytes([]byte(value))
}

func digestBytes(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
