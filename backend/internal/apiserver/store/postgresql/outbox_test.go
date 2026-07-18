package postgresql

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewOutboxMessageSeparatesUUIDAndIdempotencyKey(t *testing.T) {
	idempotencyKey := uuid.NewString() + ":uploaded"
	payload := []byte(`{"asset_id":"asset-1"}`)

	msg := newOutboxMessage(idempotencyKey, payload)

	if _, err := uuid.Parse(msg.UUID); err != nil {
		t.Fatalf("message UUID %q is invalid: %v", msg.UUID, err)
	}
	if len(msg.UUID) != 36 {
		t.Fatalf("message UUID length = %d, want 36", len(msg.UUID))
	}
	if got := msg.Metadata.Get(outboxIdempotencyKeyMetadata); got != idempotencyKey {
		t.Fatalf("idempotency key = %q, want %q", got, idempotencyKey)
	}
	if got := string(msg.Payload); got != string(payload) {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
}
