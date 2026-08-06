package store

import (
	"github.com/linguaquest/server/internal/analytics"
)

func (s *SQLiteStore) RecordModelUsage(record analytics.ModelUsage) error {
	_, err := s.db.Exec(`INSERT INTO model_usage_daily (day, provider, model, operation, prompt_tokens, completion_tokens, total_tokens, request_count, reported_request_count, error_count, total_latency_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(day, provider, model, operation) DO UPDATE SET
		prompt_tokens = model_usage_daily.prompt_tokens + excluded.prompt_tokens,
		completion_tokens = model_usage_daily.completion_tokens + excluded.completion_tokens,
		total_tokens = model_usage_daily.total_tokens + excluded.total_tokens,
		request_count = model_usage_daily.request_count + excluded.request_count,
		reported_request_count = model_usage_daily.reported_request_count + excluded.reported_request_count,
		error_count = model_usage_daily.error_count + excluded.error_count,
		total_latency_ms = model_usage_daily.total_latency_ms + excluded.total_latency_ms`,
		record.Day, record.Provider, record.Model, record.Operation, record.PromptTokens, record.CompletionTokens, record.TotalTokens, record.RequestCount, record.ReportedRequestCount, record.ErrorCount, record.TotalLatencyMilliseconds)
	return err
}

func (s *SQLiteStore) IncrementProductMetric(record analytics.ProductMetric) error {
	_, err := s.db.Exec(`INSERT INTO product_metrics_daily (day, category, name, count) VALUES (?, ?, ?, ?)
		ON CONFLICT(day, category, name) DO UPDATE SET count = product_metrics_daily.count + excluded.count`, record.Day, record.Category, record.Name, record.Count)
	return err
}

func (s *SQLiteStore) DailyReport(fromDay string, toDay string) (analytics.DailyReport, error) {
	report := analytics.DailyReport{}
	modelRows, err := s.db.Query(`SELECT day, provider, model, operation, prompt_tokens, completion_tokens, total_tokens, request_count, reported_request_count, error_count, total_latency_ms
		FROM model_usage_daily WHERE day >= ? AND day <= ?`, fromDay, toDay)
	if err != nil {
		return report, err
	}
	defer modelRows.Close()
	for modelRows.Next() {
		var item analytics.ModelUsage
		if err = modelRows.Scan(&item.Day, &item.Provider, &item.Model, &item.Operation, &item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.RequestCount, &item.ReportedRequestCount, &item.ErrorCount, &item.TotalLatencyMilliseconds); err != nil {
			return report, err
		}
		report.ModelUsage = append(report.ModelUsage, item)
	}
	if err = modelRows.Err(); err != nil {
		return report, err
	}
	productRows, err := s.db.Query(`SELECT day, category, name, count FROM product_metrics_daily WHERE day >= ? AND day <= ?`, fromDay, toDay)
	if err != nil {
		return report, err
	}
	defer productRows.Close()
	for productRows.Next() {
		var item analytics.ProductMetric
		if err = productRows.Scan(&item.Day, &item.Category, &item.Name, &item.Count); err != nil {
			return report, err
		}
		report.ProductMetrics = append(report.ProductMetrics, item)
	}
	return report, productRows.Err()
}
