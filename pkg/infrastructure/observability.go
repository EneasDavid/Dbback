package infrastructure

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// StructuredLogger provides structured logging in JSON format
type StructuredLogger struct {
	component string
	mu        sync.Mutex
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Component string                 `json:"component"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  float64                `json:"duration_ms,omitempty"`
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(component string) *StructuredLogger {
	return &StructuredLogger{
		component: component,
	}
}

// logf writes a structured log entry
func (sl *StructuredLogger) logf(level, message string, fields map[string]interface{}, err error) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Component: sl.component,
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()

	data, _ := json.Marshal(entry)
	fmt.Println(string(data))
}

// Info logs an info message
func (sl *StructuredLogger) Info(message string, fields map[string]interface{}) {
	sl.logf("INFO", message, fields, nil)
}

// Error logs an error message
func (sl *StructuredLogger) Error(message string, err error, fields map[string]interface{}) {
	sl.logf("ERROR", message, fields, err)
}

// Debug logs a debug message
func (sl *StructuredLogger) Debug(message string, fields map[string]interface{}) {
	sl.logf("DEBUG", message, fields, nil)
}

// Profiler captures performance metrics
type Profiler struct {
	name      string
	startTime time.Time
	metrics   *RuntimeMetrics
	mu        sync.Mutex
}

// RuntimeMetrics captures Go runtime metrics
type RuntimeMetrics struct {
	Timestamp     time.Time
	Goroutines    int
	HeapAlloc     uint64
	HeapSys       uint64
	HeapIdle      uint64
	HeapInuse     uint64
	HeapReleased  uint64
	HeapObjects   uint64
	StackInuse    uint64
	StackSys      uint64
	MSpanInuse    uint64
	MCacheInuse   uint64
	MallocCount   uint64
	FreeCount     uint64
	LiveObjects   uint64
	PauseTotalNs  uint64
	PauseCount    uint32
	GCCPUFraction float64
}

// NewProfiler creates a new profiler
func NewProfiler(name string) *Profiler {
	return &Profiler{
		name:      name,
		startTime: time.Now(),
		metrics:   captureMetrics(),
	}
}

// captureMetrics captures current runtime metrics
func captureMetrics() *RuntimeMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &RuntimeMetrics{
		Timestamp:     time.Now(),
		Goroutines:    runtime.NumGoroutine(),
		HeapAlloc:     m.HeapAlloc,
		HeapSys:       m.HeapSys,
		HeapIdle:      m.HeapIdle,
		HeapInuse:     m.HeapInuse,
		HeapReleased:  m.HeapReleased,
		HeapObjects:   m.HeapObjects,
		StackInuse:    m.StackInuse,
		StackSys:      m.StackSys,
		MSpanInuse:    m.MSpanInuse,
		MCacheInuse:   m.MCacheInuse,
		MallocCount:   m.Mallocs,
		FreeCount:     m.Frees,
		LiveObjects:   m.Mallocs - m.Frees,
		PauseTotalNs:  m.PauseNs[(m.NumGC+255)%256],
		PauseCount:    m.NumGC,
		GCCPUFraction: m.GCCPUFraction,
	}
}

// Stop stops profiling and returns metrics
func (p *Profiler) Stop() *ProfileResult {
	p.mu.Lock()
	defer p.mu.Unlock()

	endMetrics := captureMetrics()
	duration := time.Since(p.startTime)

	return &ProfileResult{
		Name:             p.name,
		Duration:         duration,
		StartMetrics:     p.metrics,
		EndMetrics:       endMetrics,
		MemoryDelta:      int64(endMetrics.HeapAlloc) - int64(p.metrics.HeapAlloc),
		GoroutineDelta:   endMetrics.Goroutines - p.metrics.Goroutines,
		AllocationsDelta: int64(endMetrics.MallocCount) - int64(p.metrics.MallocCount),
		DeallocsDelta:    int64(endMetrics.FreeCount) - int64(p.metrics.FreeCount),
	}
}

// ProfileResult contains profiling results
type ProfileResult struct {
	Name             string
	Duration         time.Duration
	StartMetrics     *RuntimeMetrics
	EndMetrics       *RuntimeMetrics
	MemoryDelta      int64
	GoroutineDelta   int
	AllocationsDelta int64
	DeallocsDelta    int64
}

// String returns human-readable profile result
func (pr *ProfileResult) String() string {
	return fmt.Sprintf(
		"Profile[%s]: %v | Memory: %v bytes | Goroutines: %+d | Allocs: %d | Deallocs: %d",
		pr.Name,
		pr.Duration,
		pr.MemoryDelta,
		pr.GoroutineDelta,
		pr.AllocationsDelta,
		pr.DeallocsDelta,
	)
}

// MetricsCollector collects system metrics
type MetricsCollector struct {
	counters   map[string]int64
	gauges     map[string]float64
	histograms map[string][]float64
	mu         sync.RWMutex
}

// NewMetricsCollector creates a new collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		counters:   make(map[string]int64),
		gauges:     make(map[string]float64),
		histograms: make(map[string][]float64),
	}
}

// IncrementCounter increments a counter
func (mc *MetricsCollector) IncrementCounter(name string, value int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.counters[name] += value
}

// SetGauge sets a gauge value
func (mc *MetricsCollector) SetGauge(name string, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.gauges[name] = value
}

// RecordHistogram records a value in histogram
func (mc *MetricsCollector) RecordHistogram(name string, value float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.histograms[name] = append(mc.histograms[name], value)
}

// GetMetrics returns all collected metrics
func (mc *MetricsCollector) GetMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics := make(map[string]interface{})
	metrics["counters"] = mc.counters
	metrics["gauges"] = mc.gauges

	// Calculate histogram statistics
	histStats := make(map[string]map[string]float64)
	for name, values := range mc.histograms {
		if len(values) == 0 {
			continue
		}

		sum := 0.0
		min := values[0]
		max := values[0]

		for _, v := range values {
			sum += v
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}

		histStats[name] = map[string]float64{
			"count": float64(len(values)),
			"sum":   sum,
			"mean":  sum / float64(len(values)),
			"min":   min,
			"max":   max,
		}
	}
	metrics["histograms"] = histStats

	return metrics
}

// TraceEvent represents a trace event
type TraceEvent struct {
	TraceID   string
	SpanID    string
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Status    string
	Metadata  map[string]string
}

// Tracer distributed tracing
type Tracer struct {
	events []*TraceEvent
	mu     sync.RWMutex
	maxLen int
}

// NewTracer creates a new tracer
func NewTracer(maxEvents int) *Tracer {
	return &Tracer{
		events: make([]*TraceEvent, 0, maxEvents),
		maxLen: maxEvents,
	}
}

// RecordEvent records a trace event
func (t *Tracer) RecordEvent(event *TraceEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	event.Duration = event.EndTime.Sub(event.StartTime)
	t.events = append(t.events, event)

	// Keep only recent events
	if len(t.events) > t.maxLen {
		t.events = t.events[len(t.events)-t.maxLen:]
	}
}

// GetTraceEvents returns events for a trace
func (t *Tracer) GetTraceEvents(traceID string) []*TraceEvent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*TraceEvent
	for _, event := range t.events {
		if event.TraceID == traceID {
			result = append(result, event)
		}
	}
	return result
}

// HealthCheck performs system health check
type HealthCheck struct {
	services map[string]func() error
	mu       sync.RWMutex
}

// NewHealthCheck creates a health checker
func NewHealthCheck() *HealthCheck {
	return &HealthCheck{
		services: make(map[string]func() error),
	}
}

// Register registers a service check
func (hc *HealthCheck) Register(name string, check func() error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.services[name] = check
}

// Check runs all health checks
func (hc *HealthCheck) Check() map[string]string {
	hc.mu.RLock()
	services := make(map[string]func() error)
	for k, v := range hc.services {
		services[k] = v
	}
	hc.mu.RUnlock()

	results := make(map[string]string)
	for name, check := range services {
		if err := check(); err != nil {
			results[name] = "UNHEALTHY: " + err.Error()
		} else {
			results[name] = "HEALTHY"
		}
	}

	return results
}
