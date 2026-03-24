package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/run-bigpig/jcp/internal/services"
)

func TestAppCancelScreeningQueryStopsInFlightRequest(t *testing.T) {
	tempDir := t.TempDir()
	configService, err := services.NewConfigService(tempDir)
	if err != nil {
		t.Fatalf("NewConfigService() error = %v", err)
	}
	store, err := services.NewScreeningStore(tempDir)
	if err != nil {
		t.Fatalf("NewScreeningStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	generator := &blockingScreeningSQLGenerator{
		started: make(chan struct{}, 1),
	}
	app := &App{
		screeningQuery: services.NewScreeningQueryService(configService, store, generator),
	}

	resultCh := make(chan *services.ScreeningQueryResponse, 1)
	go func() {
		resultCh <- app.RunScreeningQuery(services.ScreeningQueryRequest{
			Prompt:      "测试取消筛选",
			ResultMode:  services.ScreeningResultModeTopN,
			ResultLimit: 10,
			Page:        1,
			PageSize:    50,
		})
	}()

	select {
	case <-generator.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("query did not start in time")
	}

	if !app.CancelScreeningQuery() {
		t.Fatalf("CancelScreeningQuery() = false, want true")
	}

	select {
	case response := <-resultCh:
		if response == nil {
			t.Fatalf("response = nil")
		}
		if !strings.Contains(strings.ToLower(response.Error), "canceled") {
			t.Fatalf("response.Error = %q, want canceled error", response.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("query did not stop after cancellation")
	}
}

type blockingScreeningSQLGenerator struct {
	started chan struct{}
}

func (g *blockingScreeningSQLGenerator) GenerateSQL(ctx context.Context, prompt string, aiConfigID string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (g *blockingScreeningSQLGenerator) GenerateSQLStream(ctx context.Context, prompt string, aiConfigID string, onDelta func(string)) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (g *blockingScreeningSQLGenerator) GenerateReasoningStream(ctx context.Context, prompt string, aiConfigID string, onDelta func(string)) (string, error) {
	select {
	case g.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return "", ctx.Err()
}
