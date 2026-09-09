package background

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/pkg/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type integrationOutbox struct {
	event     *entity.TaskOutbox
	published bool
}

func (o *integrationOutbox) ListUnpublished(context.Context, int) ([]*entity.TaskOutbox, error) {
	if o.published {
		return nil, nil
	}
	return []*entity.TaskOutbox{o.event}, nil
}

func (o *integrationOutbox) CountUnpublished(context.Context) (int64, error) {
	if o.published {
		return 0, nil
	}
	return 1, nil
}

func (o *integrationOutbox) MarkPublished(context.Context, string) error {
	o.published = true
	return nil
}

func TestRedisOutboxPreservesW3CParentAcrossConsumer(t *testing.T) {
	address := os.Getenv("MEMORA_TEST_REDIS_ADDR")
	if address == "" {
		t.Skip("set MEMORA_TEST_REDIS_ADDR to run the Redis trace integration test")
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	}()

	stream := "memora:test:document:" + uuid.NewString()
	group := "trace-test"
	t.Cleanup(func() { _ = client.Del(context.Background(), stream).Err() })
	if err := client.XGroupCreateMkStream(ctx, stream, group, "0").Err(); err != nil {
		t.Fatal(err)
	}
	taskID := uuid.NewString()
	outbox := &integrationOutbox{event: &entity.TaskOutbox{
		ID: uuid.NewString(), EventType: "document.parse", AggregateID: taskID,
		Payload: `{"task_id":"` + taskID + `","traceparent":"00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"}`,
	}}
	manager := &Manager{
		redis: client, outbox: outbox,
		consumer:  config.DocumentConsumerConfig{Enabled: true, Stream: stream, Group: group},
		outboxCfg: config.OutboxConfig{BatchSize: 10},
	}
	manager.publishBatch(ctx)
	if !outbox.published {
		t.Fatal("outbox event was not marked published")
	}

	streams, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: group, Consumer: "trace-consumer", Streams: []string{stream, ">"}, Count: 1,
	}).Result()
	if err != nil || len(streams) != 1 || len(streams[0].Messages) != 1 {
		t.Fatalf("read stream: streams=%#v err=%v", streams, err)
	}
	message := streams[0].Messages[0]
	consumerCtx := extractStreamContext(context.Background(), message)
	_, span := provider.Tracer("integration").Start(consumerCtx, "document.process")
	span.End()

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d", len(spans))
	}
	if spans[0].SpanContext().TraceID().String() != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("trace id = %s", spans[0].SpanContext().TraceID())
	}
	if spans[0].Parent().SpanID().String() != "0123456789abcdef" {
		t.Fatalf("parent span id = %s", spans[0].Parent().SpanID())
	}
}
