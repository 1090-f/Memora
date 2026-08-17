package events

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisEventStreamReplaysHistoryAndContinuesLive(t *testing.T) {
	address := os.Getenv("MEMORA_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("set MEMORA_TEST_REDIS_ADDR to run the Redis Stream integration test")
	}

	client := redis.NewClient(&redis.Options{Addr: address})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	runID := contracts.ID("test-" + uuid.NewString())
	t.Cleanup(func() {
		_ = client.Del(context.Background(), eventStreamKey(runID), eventSequenceKey(runID)).Err()
	})
	publisher := NewRedisEventPublisher(client)
	subscriber := NewRedisEventSubscriber(client)

	publish := func(label string) {
		t.Helper()
		data, _ := json.Marshal(map[string]string{"label": label})
		if err := publisher.Publish(ctx, contracts.AgentEvent{RunID: runID, EventType: contracts.EventAnswerDelta, Data: data}); err != nil {
			t.Fatalf("publish %s: %v", label, err)
		}
	}
	publish("one")
	publish("two")
	publish("three")

	events, err := subscriber.Subscribe(ctx, runID, 1)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	receive := func(wantSequence int64) {
		t.Helper()
		select {
		case event := <-events:
			if event.Sequence != wantSequence {
				t.Fatalf("sequence = %d, want %d", event.Sequence, wantSequence)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for sequence %d", wantSequence)
		}
	}
	receive(2)
	receive(3)
	publish("four")
	receive(4)
}
