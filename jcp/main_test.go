package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestBuildAppOptionsStartsMaximised(t *testing.T) {
	app := &App{}
	appOptions := buildAppOptions(app)
	if appOptions == nil {
		t.Fatalf("buildAppOptions() = nil")
	}
	if appOptions.WindowStartState != options.Maximised {
		t.Fatalf("WindowStartState = %v, want %v", appOptions.WindowStartState, options.Maximised)
	}
}
