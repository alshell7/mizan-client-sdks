package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFileInboxDeduplicatesBothIdentities(t *testing.T) {
	inbox := &fileInbox{root: t.TempDir()}
	record := inboxRecord{
		OutboxID: "outbox-1", Stream: "ledger", EventID: "ledger-1",
		BusinessID: "business-1", BusinessSequence: 7,
		Payload: json.RawMessage(`{"event_id":"ledger-1"}`), ReceivedAt: time.Now().UTC(),
	}
	if err := inbox.persist(record); err != nil {
		t.Fatalf("persist first delivery: %v", err)
	}
	if err := inbox.persist(record); err != nil {
		t.Fatalf("persist exact retry: %v", err)
	}

	eventRetry := record
	eventRetry.OutboxID = "outbox-2"
	if err := inbox.persist(eventRetry); err != nil {
		t.Fatalf("deduplicate repeated event identity: %v", err)
	}

	outboxCollision := record
	outboxCollision.EventID = "ledger-2"
	if err := inbox.persist(outboxCollision); err == nil {
		t.Fatal("expected reused outbox ID to be rejected")
	}

	eventCollision := eventRetry
	eventCollision.BusinessSequence = 8
	if err := inbox.persist(eventCollision); err == nil {
		t.Fatal("expected reused event ID to be rejected")
	}
}
