package runs

import (
	"context"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

func TestRunnerEmitsSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(sdktrace.NewTracerProvider()) })

	store := newFakeStore()
	store.runs["run-1"] = domain.Run{ID: "run-1", OrgID: "o", ProjectID: "p", TaskID: "t", Status: domain.RunQueued}
	runner := NewRunner(&fakeBackend{}, store, nil, nil, "", slog.New(slog.DiscardHandler))

	if _, err := runner.Execute(context.Background(), store.runs["run-1"], "prompt", nil, "", nil, nil, nil, "", "", "", 0.01, nil, 0, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("no spans exported")
	}
	found := false
	for _, span := range spans {
		if span.Name != "run.execute" {
			continue
		}
		found = true
		for _, attr := range span.Attributes {
			if string(attr.Key) == "run.id" && attr.Value.AsString() == "run-1" {
				return
			}
		}
	}
	if !found {
		t.Fatalf("run.execute span missing, got %d spans", len(spans))
	}
	t.Fatal("run.execute span missing the run.id attribute")
}
