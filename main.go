package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Add new struct for error summary response
type ErrorSummary struct {
	Overall struct {
		ErrorCount       int `json:"errorCount"`
		TransactionCount int `json:"transactionCount"`
	} `json:"overall"`
	Transactions []struct {
		TransactionName  string `json:"transactionName"`
		ErrorCount       int    `json:"errorCount"`
		TransactionCount int    `json:"transactionCount"`
	} `json:"transactions"`
}

// Add new struct for points response
type Points struct {
	NormalPoints  [][4]interface{} `json:"normalPoints"`
	ErrorPoints   [][4]interface{} `json:"errorPoints"`
	PartialPoints [][4]interface{} `json:"partialPoints"`
}

// Update TraceHeader struct
type TraceHeader struct {
	TransactionName string `json:"transactionName"`
}

type AgentRollup struct {
	ID       string        `json:"id"`
	Display  string        `json:"display"`
	Children []AgentRollup `json:"children"`
}

type ChildAgent struct {
	ID      string `json:"id"`
	Display string `json:"display"`
}

type Config struct {
	Server struct {
		GlowrootURL                  string `yaml:"glowroot_url"`
		ExporterPort                 int    `yaml:"exporter_port"`
		GlowrootTimeIntervalMinutes  int    `yaml:"glowroot_time_interval_minutes"`
		MetricsUpdateIntervalSeconds int    `yaml:"metrics_update_interval_seconds"`
	} `yaml:"server"`
}

// Add global config variable
var config *Config

var (
	// agentRollup represents the mapping between agent rollup IDs and their display names
	// Labels:
	//   - agent_rollup: The ID of the agent rollup
	//   - agent_rollup_display_name: The human-readable name of the agent rollup
	agentRollup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_agent_rollup",
			Help: "Information about Glowroot agent rollups",
		},
		[]string{"agent_rollup", "agent_rollup_display_name"},
	)

	// agentRollupID tracks the relationship between agent rollups and their child agents
	// Labels:
	//   - agent_rollup: The parent agent rollup ID
	//   - agent_id: The child agent ID
	agentRollupID = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_agent_rollup_id",
			Help: "Information about Glowroot agent IDs",
		},
		[]string{"agent_rollup", "agent_id"},
	)
	// errorTotalCount tracks the total number of errors for each agent
	// Labels:
	//   - agent_rollup: The parent agent rollup ID
	//   - agent_id: The agent ID reporting the errors
	errorTotalCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_summaries_agent_rollup_id_error_total_count",
			Help: "Total error count from overall statistics",
		},
		[]string{"agent_rollup", "agent_id"},
	)

	// transactionTotalCount tracks the total number of transactions for each agent
	// Labels:
	//   - agent_rollup: The parent agent rollup ID
	//   - agent_id: The agent ID reporting the transactions
	transactionTotalCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_summaries_agent_rollup_id_error_transaction_total_count",
			Help: "Total transaction count from overall statistics",
		},
		[]string{"agent_rollup", "agent_id"},
	)

	// transactionError tracks error counts per individual transaction
	// Labels:
	//   - agent_rollup: The parent agent rollup ID
	//   - agent_id: The agent ID reporting the transaction
	//   - transaction_name: The name of the transaction
	//   - transaction_count: Total number of occurrences of this transaction
	transactionError = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_summaries_agent_rollup_id_error",
			Help: "Error count per individual transaction",
		},
		[]string{"agent_rollup", "agent_id", "transaction_name"},
	)

	// traceCountSlowTrace tracks the number of traces for each agent
	traceCountSlowTrace = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_trace_count_agent_rollup_id_slow_trace",
			Help: "Total slow trace count for agent",
		},
		[]string{"agent_rollup", "agent_id"},
	)

	// Update pointsSlowTrace definition
	pointsSlowTrace = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_points_agent_rollup_id_slow_trace",
			Help: "Slow Trace points information",
		},
		[]string{"agent_rollup", "agent_id", "trace_id", "status", "transaction_name"},
	)
)

func init() {
	prometheus.MustRegister(agentRollup)
	prometheus.MustRegister(agentRollupID)
	prometheus.MustRegister(errorTotalCount)
	prometheus.MustRegister(transactionTotalCount)
	prometheus.MustRegister(transactionError)
	prometheus.MustRegister(traceCountSlowTrace)
	prometheus.MustRegister(pointsSlowTrace)
}

// fetchAgentRollups retrieves the list of top-level agent rollups from Glowroot
// baseURL: Base URL of the Glowroot server
// Returns: List of agent rollups and error if any
func fetchAgentRollups(baseURL string) ([]AgentRollup, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	from := now - (int64(config.Server.GlowrootTimeIntervalMinutes) * 60 * 1000)

	url := fmt.Sprintf("%s/backend/top-level-agent-rollups?from=%d&to=%d", baseURL, from, now)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response failed: %v", err)
	}

	var rollups []AgentRollup
	if err := json.Unmarshal(body, &rollups); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %v, body: %s", err, string(body))
	}

	return rollups, nil
}

// fetchChildAgentRollups retrieves child agents for a given top-level agent rollup
// baseURL: Base URL of the Glowroot server
// topLevelID: ID of the parent agent rollup
// Returns: List of child agents and error if any
func fetchChildAgentRollups(baseURL, topLevelID string) ([]ChildAgent, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	from := now - (int64(config.Server.GlowrootTimeIntervalMinutes) * 60 * 1000)

	url := fmt.Sprintf("%s/backend/child-agent-rollups?top-level-id=%s&from=%d&to=%d",
		baseURL, url.QueryEscape(topLevelID), from, now)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var children []ChildAgent
	if err := json.NewDecoder(resp.Body).Decode(&children); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %v", err)
	}

	return children, nil
}

// fetchErrorSummary retrieves error statistics for a specific agent
// baseURL: Base URL of the Glowroot server
// agentID: ID of the agent to fetch errors for
// Returns: Error summary containing overall and per-transaction error counts
func fetchErrorSummary(baseURL, agentID string) (*ErrorSummary, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	from := now - (int64(config.Server.GlowrootTimeIntervalMinutes) * 60 * 1000)

	url := fmt.Sprintf("%s/backend/error/summaries?agent-rollup-id=%s&transaction-type=Web&from=%d&to=%d&sort-order=error-count&limit=1000",
		baseURL, url.QueryEscape(agentID), from, now)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var summary ErrorSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %v", err)
	}

	return &summary, nil
}

// fetchTraceCountSlowTrace retrieves trace count for a specific agent
func fetchTraceCountSlowTrace(baseURL, agentID string) (int, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	from := now - (int64(config.Server.GlowrootTimeIntervalMinutes) * 60 * 1000)

	url := fmt.Sprintf("%s/backend/transaction/trace-count?agent-rollup-id=%s&transaction-type=Web&from=%d&to=%d",
		baseURL, url.QueryEscape(agentID), from, now)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("reading response failed: %v", err)
	}

	count := 0
	if err := json.Unmarshal(body, &count); err != nil {
		return 0, fmt.Errorf("JSON unmarshal failed: %v", err)
	}

	return count, nil
}

// Add new function after other fetch functions
func fetchPointsSlowTrace(baseURL, agentID string) (*Points, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	from := now - (int64(config.Server.GlowrootTimeIntervalMinutes) * 60 * 1000)

	url := fmt.Sprintf("%s/backend/transaction/points?transaction-type=Web&from=%d&to=%d&duration-millis-low=0&headline-comparator=begins&headline=&error-message-comparator=begins&error-message=&user-comparator=begins&user=&attribute-name=&attribute-value-comparator=begins&attribute-value=&limit=500&agent-rollup-id=%s",
		baseURL, from, now, url.QueryEscape(agentID))

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var points Points
	if err := json.NewDecoder(resp.Body).Decode(&points); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %v", err)
	}

	return &points, nil
}

// Add new function to fetch trace header
func fetchTraceHeader(baseURL, agentID, traceID string) (*TraceHeader, error) {
	url := fmt.Sprintf("%s/backend/trace/header?agent-id=%s&trace-id=%s",
		baseURL, url.QueryEscape(agentID), url.QueryEscape(traceID))

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var traceHeader TraceHeader
	if err := json.NewDecoder(resp.Body).Decode(&traceHeader); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %v", err)
	}

	return &traceHeader, nil
}

// Add this function before updateMetrics
func logMetrics(metricNames ...string) {
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		log.Printf("Error gathering metrics: %v", err)
		return
	}

	for _, mf := range metricFamilies {
		// Check if this metric name is in the requested list
		shouldLog := false
		for _, name := range metricNames {
			if mf.GetName() == name {
				shouldLog = true
				break
			}
		}

		if shouldLog {
			log.Printf("Metric: %s", mf.GetName())
			log.Printf("Help: %s", mf.GetHelp())

			for _, metric := range mf.GetMetric() {
				labels := make([]string, len(metric.GetLabel()))
				for i, label := range metric.GetLabel() {
					labels[i] = fmt.Sprintf("%s=%s", label.GetName(), label.GetValue())
				}
				value := metric.GetGauge().GetValue()
				log.Printf("  Labels: {%s}, Value: %f", labels, value)
			}
			log.Println("---")
		}
	}
}

// loadConfig loads and parses the YAML configuration file
// configPath: Path to the configuration file
// Returns: Parsed configuration and error if any
func loadConfig(configPath string) (*Config, error) {
	config := &Config{}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// updateMetrics periodically fetches metrics from Glowroot and updates Prometheus metrics
// baseURL: Base URL of the Glowroot server
// This function runs in an infinite loop with configured sleep intervals
func updateMetrics(baseURL string) {

	for {
		// Define new variables for metrics
		agentRollupNew := agentRollup
		agentRollupIDNew := agentRollupID
		errorTotalCountNew := errorTotalCount
		transactionTotalCountNew := transactionTotalCount
		transactionErrorNew := transactionError
		traceCountSlowTraceNew := traceCountSlowTrace
		pointsSlowTraceNew := pointsSlowTrace

		rollups, err := fetchAgentRollups(baseURL)
		if err != nil {
			log.Printf("Error fetching agent rollups: %v", err)
			time.Sleep(time.Duration(config.Server.MetricsUpdateIntervalSeconds) * time.Second)
			continue
		}

		for _, rollup := range rollups {
			// Set metrics for the rollup info
			agentRollupNew.With(prometheus.Labels{
				"agent_rollup":              rollup.ID,
				"agent_rollup_display_name": rollup.Display,
			}).Set(1)

			// Remove the empty agent_id metric set
			// Fetch and set metrics for child agents
			children, err := fetchChildAgentRollups(baseURL, rollup.ID)
			if err != nil {
				log.Printf("Error fetching child agents for %s: %v", rollup.ID, err)
				continue
			}

			for _, child := range children {
				// Set existing agent ID metric
				agentRollupIDNew.With(prometheus.Labels{
					"agent_rollup": rollup.ID,
					"agent_id":     child.ID,
				}).Set(1)

				// Fetch and set error metrics
				summary, err := fetchErrorSummary(baseURL, child.ID)
				if err != nil {
					log.Printf("Error fetching error summary for agent %s: %v", child.ID, err)
					continue
				}

				// Set total counts
				errorTotalCountNew.With(prometheus.Labels{
					"agent_rollup": rollup.ID,
					"agent_id":     child.ID,
				}).Set(float64(summary.Overall.ErrorCount))

				transactionTotalCountNew.With(prometheus.Labels{
					"agent_rollup": rollup.ID,
					"agent_id":     child.ID,
				}).Set(float64(summary.Overall.TransactionCount))

				// Set per-transaction error counts
				for _, t := range summary.Transactions {
					transactionErrorNew.With(prometheus.Labels{
						"agent_rollup":     rollup.ID,
						"agent_id":         child.ID,
						"transaction_name": t.TransactionName,
					}).Set(float64(t.ErrorCount))
				}

				// Fetch and set trace count
				count, err := fetchTraceCountSlowTrace(baseURL, child.ID)
				if err != nil {
					log.Printf("Error fetching trace count for agent %s: %v", child.ID, err)
					continue
				}

				traceCountSlowTraceNew.With(prometheus.Labels{
					"agent_rollup": rollup.ID,
					"agent_id":     child.ID,
				}).Set(float64(count))

				// Fetch and set points data
				points, err := fetchPointsSlowTrace(baseURL, child.ID)
				if err != nil {
					log.Printf("Error fetching points for agent %s: %v", child.ID, err)
					continue
				}

				// Update processPoints function
				processPoints := func(points [][4]interface{}, pointType string) {
					for _, point := range points {
						traceTime := point[1].(float64)
						agentId := point[2].(string)
						traceId := point[3].(string)

						// Fetch trace details
						traceHeader, err := fetchTraceHeader(baseURL, agentId, traceId)
						transactionName := "null"

						if err == nil && traceHeader != nil && traceHeader.TransactionName != "" {
							transactionName = traceHeader.TransactionName
						}
						log.Println(traceHeader)

						pointsSlowTraceNew.With(prometheus.Labels{
							"agent_rollup":     rollup.ID,
							"agent_id":         agentId,
							"trace_id":         traceId,
							"status":           pointType,
							"transaction_name": transactionName,
						}).Set(traceTime)

					}
				}

				processPoints(points.NormalPoints, "normal")
				processPoints(points.ErrorPoints, "error")
				processPoints(points.PartialPoints, "partial")
			}
		}

		// Define new variables for metrics
		agentRollup = agentRollupNew
		agentRollupID = agentRollupIDNew
		errorTotalCount = errorTotalCountNew
		transactionTotalCount = transactionTotalCountNew
		transactionError = transactionErrorNew
		traceCountSlowTrace = traceCountSlowTraceNew
		pointsSlowTrace = pointsSlowTraceNew
		// // After processing all metrics, log them with specific metric names
		// logMetrics(
		// 	"glowroot_agent_rollup",
		// )
		// // After processing all metrics, log them with specific metric names
		// logMetrics(
		// 	"glowroot_agent_rollup",
		// 	"glowroot_agent_rollup_id",
		// 	"glowroot_summaries_agent_rollup_id_error_total_count",
		// 	"glowroot_summaries_agent_rollup_id_error_transaction_total_count",
		// 	"glowroot_summaries_agent_rollup_id_error",
		// 	"glowroot_trace_count_agent_rollup_id_slow_trace",
		// 	"glowroot_points_agent_rollup_id_slow_trace",
		// )

		// Sleep for configured interval before fetching metrics again
		time.Sleep(time.Duration(config.Server.MetricsUpdateIntervalSeconds) * time.Second)
	}
}

// main initializes the exporter, loads configuration, and starts the HTTP server
// Starts metrics collection in a background goroutine and exposes Prometheus metrics endpoint
func main() {
	var err error
	config, err = loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Start metrics collection in background
	go updateMetrics(config.Server.GlowrootURL)

	// Expose Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", config.Server.ExporterPort)
	log.Printf("Glowroot: %s", config.Server.GlowrootURL)
	log.Printf("Metrics Update Interval Seconds: %d", config.Server.MetricsUpdateIntervalSeconds)
	log.Printf("Glowroot Time Interval Minutes: %d", config.Server.GlowrootTimeIntervalMinutes)
	log.Printf("Starting Glowroot exporter on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
