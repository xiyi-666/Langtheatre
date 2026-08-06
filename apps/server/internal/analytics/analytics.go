// Package analytics records privacy-preserving daily product aggregates.
// It deliberately never receives user IDs, prompts, page contents, or raw clicks.
package analytics

import (
	"log"
	"sort"
	"sync"
	"time"
)

const (
	MetricCategoryFeature = "FEATURE"
	MetricCategoryClick   = "CLICK"
)

type ModelUsage struct {
	Day                      string `json:"day"`
	Provider                 string `json:"provider"`
	Model                    string `json:"model"`
	Operation                string `json:"operation"`
	PromptTokens             int64  `json:"promptTokens"`
	CompletionTokens         int64  `json:"completionTokens"`
	TotalTokens              int64  `json:"totalTokens"`
	RequestCount             int64  `json:"requestCount"`
	ReportedRequestCount     int64  `json:"reportedRequestCount"`
	ErrorCount               int64  `json:"errorCount"`
	TotalLatencyMilliseconds int64  `json:"totalLatencyMilliseconds"`
}

type ProductMetric struct {
	Day      string `json:"day"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Count    int64  `json:"count"`
}

type DailyReport struct {
	ModelUsage     []ModelUsage    `json:"modelUsage"`
	ProductMetrics []ProductMetric `json:"productMetrics"`
}

// Store is intentionally small so persistent and development stores can share
// the same reporter without exposing analytics through user-facing services.
type Store interface {
	RecordModelUsage(ModelUsage) error
	IncrementProductMetric(ProductMetric) error
	DailyReport(fromDay string, toDay string) (DailyReport, error)
}

type queuedRecord struct {
	model   *ModelUsage
	product *ProductMetric
}

type Reporter struct {
	store    Store
	location *time.Location
	queue    chan queuedRecord
	done     chan struct{}
	close    sync.Once
}

func NewReporter(store Store, timezone string) *Reporter {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}
	reporter := &Reporter{
		store:    store,
		location: location,
		queue:    make(chan queuedRecord, 2048),
		done:     make(chan struct{}),
	}
	go reporter.run()
	return reporter
}

func (r *Reporter) RecordModelUsage(provider string, model string, operation string, promptTokens int64, completionTokens int64, totalTokens int64, usageReported bool, failed bool, latency time.Duration) {
	if r == nil {
		return
	}
	record := ModelUsage{
		Day:                      r.today(),
		Provider:                 provider,
		Model:                    model,
		Operation:                operation,
		PromptTokens:             max(0, promptTokens),
		CompletionTokens:         max(0, completionTokens),
		TotalTokens:              max(0, totalTokens),
		RequestCount:             1,
		TotalLatencyMilliseconds: max(0, latency.Milliseconds()),
	}
	if usageReported {
		record.ReportedRequestCount = 1
	}
	if failed {
		record.ErrorCount = 1
	}
	r.enqueue(queuedRecord{model: &record})
}

func (r *Reporter) RecordProductMetric(category string, name string) {
	if r == nil {
		return
	}
	r.enqueue(queuedRecord{product: &ProductMetric{Day: r.today(), Category: category, Name: name, Count: 1}})
}

func (r *Reporter) DailyReport(fromDay string, toDay string) (DailyReport, error) {
	if r == nil {
		return DailyReport{}, nil
	}
	report, err := r.store.DailyReport(fromDay, toDay)
	if err != nil {
		return DailyReport{}, err
	}
	sort.Slice(report.ModelUsage, func(i, j int) bool {
		left, right := report.ModelUsage[i], report.ModelUsage[j]
		if left.Day != right.Day {
			return left.Day < right.Day
		}
		if left.Provider != right.Provider {
			return left.Provider < right.Provider
		}
		if left.Model != right.Model {
			return left.Model < right.Model
		}
		return left.Operation < right.Operation
	})
	sort.Slice(report.ProductMetrics, func(i, j int) bool {
		left, right := report.ProductMetrics[i], report.ProductMetrics[j]
		if left.Day != right.Day {
			return left.Day < right.Day
		}
		if left.Category != right.Category {
			return left.Category < right.Category
		}
		return left.Name < right.Name
	})
	return report, nil
}

func (r *Reporter) CurrentDay() string {
	if r == nil {
		return time.Now().Format("2006-01-02")
	}
	return r.today()
}

func (r *Reporter) Close() {
	if r == nil {
		return
	}
	r.close.Do(func() {
		close(r.queue)
		<-r.done
	})
}

func (r *Reporter) today() string {
	return time.Now().In(r.location).Format("2006-01-02")
}

func (r *Reporter) enqueue(record queuedRecord) {
	select {
	case r.queue <- record:
	default:
		log.Printf("analytics queue is full; drop daily aggregate record")
	}
}

func (r *Reporter) run() {
	defer close(r.done)
	for record := range r.queue {
		var err error
		if record.model != nil {
			err = r.store.RecordModelUsage(*record.model)
		} else if record.product != nil {
			err = r.store.IncrementProductMetric(*record.product)
		}
		if err != nil {
			log.Printf("persist analytics aggregate failed: %v", err)
		}
	}
}

func max(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
