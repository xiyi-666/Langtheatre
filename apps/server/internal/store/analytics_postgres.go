package store

import (
	"context"
	"time"

	"github.com/linguaquest/server/internal/analytics"
)

func (s *PostgresStore) RecordModelUsage(record analytics.ModelUsage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `INSERT INTO model_usage_daily (day, provider, model, operation, prompt_tokens, completion_tokens, total_tokens, request_count, reported_request_count, error_count, total_latency_ms)
		VALUES ($1::date, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (day, provider, model, operation) DO UPDATE SET
		prompt_tokens = model_usage_daily.prompt_tokens + EXCLUDED.prompt_tokens,
		completion_tokens = model_usage_daily.completion_tokens + EXCLUDED.completion_tokens,
		total_tokens = model_usage_daily.total_tokens + EXCLUDED.total_tokens,
		request_count = model_usage_daily.request_count + EXCLUDED.request_count,
		reported_request_count = model_usage_daily.reported_request_count + EXCLUDED.reported_request_count,
		error_count = model_usage_daily.error_count + EXCLUDED.error_count,
		total_latency_ms = model_usage_daily.total_latency_ms + EXCLUDED.total_latency_ms`,
		record.Day, record.Provider, record.Model, record.Operation, record.PromptTokens, record.CompletionTokens, record.TotalTokens, record.RequestCount, record.ReportedRequestCount, record.ErrorCount, record.TotalLatencyMilliseconds)
	return err
}

func (s *PostgresStore) IncrementProductMetric(record analytics.ProductMetric) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `INSERT INTO product_metrics_daily (day, category, name, count) VALUES ($1::date, $2, $3, $4)
		ON CONFLICT (day, category, name) DO UPDATE SET count = product_metrics_daily.count + EXCLUDED.count`, record.Day, record.Category, record.Name, record.Count)
	return err
}

func (s *PostgresStore) DailyReport(fromDay string, toDay string) (analytics.DailyReport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	report := analytics.DailyReport{}
	modelRows, err := s.pool.Query(ctx, `SELECT day::text, provider, model, operation, prompt_tokens, completion_tokens, total_tokens, request_count, reported_request_count, error_count, total_latency_ms
		FROM model_usage_daily WHERE day >= $1::date AND day <= $2::date`, fromDay, toDay)
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
	productRows, err := s.pool.Query(ctx, `SELECT day::text, category, name, count FROM product_metrics_daily WHERE day >= $1::date AND day <= $2::date`, fromDay, toDay)
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
