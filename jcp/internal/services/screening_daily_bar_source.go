package services

import (
	"fmt"
	"strings"

	"github.com/run-bigpig/jcp/internal/models"
)

type screeningDailyBarSource interface {
	Fetch(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error)
}

type ScreeningDailyBarSourceEvent struct {
	Source  string `json:"source"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ScreeningDailyBarSourceObserver func(event ScreeningDailyBarSourceEvent)

type screeningDailyBarSourceFunc func(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error)

func (f screeningDailyBarSourceFunc) Fetch(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
	return f(symbol, lookbackDays, observer)
}

type namedScreeningDailyBarSource struct {
	name   string
	source screeningDailyBarSource
}

type screeningDailyBarSourceChain struct {
	sources []namedScreeningDailyBarSource
}

func newScreeningDailyBarSourceChain(baostockSource, sinaSource screeningDailyBarSource) screeningDailyBarSource {
	return &screeningDailyBarSourceChain{
		sources: []namedScreeningDailyBarSource{
			{name: "baostock", source: baostockSource},
			{name: "sina", source: sinaSource},
		},
	}
}

func (c *screeningDailyBarSourceChain) Fetch(symbol string, lookbackDays int, observer ScreeningDailyBarSourceObserver) ([]models.KLineData, error) {
	var failures []string
	for index, candidate := range c.sources {
		if candidate.source == nil {
			failures = append(failures, fmt.Sprintf("%s: source not configured", candidate.name))
			continue
		}

		emitScreeningDailyBarSourceEvent(observer, candidate.name, "start", fmt.Sprintf("using %s", candidate.name))
		bars, err := candidate.source.Fetch(symbol, lookbackDays, observer)
		if err == nil && len(bars) > 0 {
			emitScreeningDailyBarSourceEvent(observer, candidate.name, "success", fmt.Sprintf("%s succeeded", candidate.name))
			return bars, nil
		}
		if err != nil {
			emitScreeningDailyBarSourceEvent(observer, candidate.name, "error", err.Error())
			failures = append(failures, fmt.Sprintf("%s: %v", candidate.name, err))
			if index < len(c.sources)-1 {
				emitScreeningDailyBarSourceEvent(observer, c.sources[index+1].name, "switch", fmt.Sprintf("switching to %s", c.sources[index+1].name))
			}
			continue
		}
		emitScreeningDailyBarSourceEvent(observer, candidate.name, "empty", "empty daily bar response")
		failures = append(failures, fmt.Sprintf("%s: empty daily bar response", candidate.name))
		if index < len(c.sources)-1 {
			emitScreeningDailyBarSourceEvent(observer, c.sources[index+1].name, "switch", fmt.Sprintf("switching to %s", c.sources[index+1].name))
		}
	}

	return nil, fmt.Errorf("screening daily bars for %s failed: %s", symbol, strings.Join(failures, "; "))
}

func emitScreeningDailyBarSourceEvent(observer ScreeningDailyBarSourceObserver, source, status, message string) {
	if observer == nil {
		return
	}
	observer(ScreeningDailyBarSourceEvent{
		Source:  source,
		Status:  status,
		Message: message,
	})
}
