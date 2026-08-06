package store

import (
	"github.com/linguaquest/server/internal/analytics"
)

func (s *MemoryStore) RecordModelUsage(record analytics.ModelUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := record.Day + "|" + record.Provider + "|" + record.Model + "|" + record.Operation
	prior := s.modelUsageDaily[key]
	prior.Day, prior.Provider, prior.Model, prior.Operation = record.Day, record.Provider, record.Model, record.Operation
	prior.PromptTokens += record.PromptTokens
	prior.CompletionTokens += record.CompletionTokens
	prior.TotalTokens += record.TotalTokens
	prior.RequestCount += record.RequestCount
	prior.ReportedRequestCount += record.ReportedRequestCount
	prior.ErrorCount += record.ErrorCount
	prior.TotalLatencyMilliseconds += record.TotalLatencyMilliseconds
	s.modelUsageDaily[key] = prior
	return nil
}

func (s *MemoryStore) IncrementProductMetric(record analytics.ProductMetric) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := record.Day + "|" + record.Category + "|" + record.Name
	prior := s.productMetricsDaily[key]
	prior.Day, prior.Category, prior.Name = record.Day, record.Category, record.Name
	prior.Count += record.Count
	s.productMetricsDaily[key] = prior
	return nil
}

func (s *MemoryStore) DailyReport(fromDay string, toDay string) (analytics.DailyReport, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	report := analytics.DailyReport{}
	for _, item := range s.modelUsageDaily {
		if item.Day >= fromDay && item.Day <= toDay {
			report.ModelUsage = append(report.ModelUsage, item)
		}
	}
	for _, item := range s.productMetricsDaily {
		if item.Day >= fromDay && item.Day <= toDay {
			report.ProductMetrics = append(report.ProductMetrics, item)
		}
	}
	return report, nil
}
