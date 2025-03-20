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

// Add new struct for transaction summary response
type TransactionSummary struct {
	Overall struct {
		TotalDurationNanos float64 `json:"totalDurationNanos"`
		TransactionCount   int     `json:"transactionCount"`
	} `json:"overall"`
	Transactions []struct {
		TransactionName    string  `json:"transactionName"`
		TotalDurationNanos float64 `json:"totalDurationNanos"`
		TransactionCount   int     `json:"transactionCount"`
	} `json:"transactions"`
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
			Name: "glowroot_agent_rollup_id_error_total_count",
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
			Name: "glowroot_agent_rollup_id_error_transaction_total_count",
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
			Name: "glowroot_agent_rollup_id_error",
			Help: "Error count per individual transaction",
		},
		[]string{"agent_rollup", "agent_id", "transaction_name"},
	)

	// slowTraceTransactionTotalCount tracks the total number of slow transactions
	// Labels:
	//   - agent_rollup: The parent agent rollup ID
	//   - agent_id: The agent ID reporting the slow traces
	slowTraceTransactionTotalCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_agent_rollup_id_slow_trace_transaction_total_count",
			Help: "Total transaction count from slow trace overall statistics",
		},
		[]string{"agent_rollup", "agent_id"},
	)

	// slowTraceTransaction tracks individual slow transactions and their counts
	// Labels:
	//   - agent_rollup: The parent agent rollup ID
	//   - agent_id: The agent ID reporting the slow trace
	//   - transaction_name: The name of the slow transaction
	slowTraceTransaction = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_agent_rollup_id_slow_trace_transaction",
			Help: "Transaction count per slow trace",
		},
		[]string{"agent_rollup", "agent_id", "transaction_name"},
	)
)

func init() {
	prometheus.MustRegister(agentRollup)
	prometheus.MustRegister(agentRollupID)
	prometheus.MustRegister(errorTotalCount)
	prometheus.MustRegister(transactionTotalCount)
	prometheus.MustRegister(transactionError)
	prometheus.MustRegister(slowTraceTransactionTotalCount)
	prometheus.MustRegister(slowTraceTransaction)
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

// fetchChildAgents retrieves child agents for a given top-level agent rollup
// baseURL: Base URL of the Glowroot server
// topLevelID: ID of the parent agent rollup
// Returns: List of child agents and error if any
func fetchChildAgents(baseURL, topLevelID string) ([]ChildAgent, error) {
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

// fetchTransactionSummary retrieves transaction performance data for a specific agent
// baseURL: Base URL of the Glowroot server
// agentID: ID of the agent to fetch transactions for
// Returns: Transaction summary containing overall and per-transaction statistics
func fetchTransactionSummary(baseURL, agentID string) (*TransactionSummary, error) {
	now := time.Now().UnixNano() / int64(time.Millisecond)
	from := now - (int64(config.Server.GlowrootTimeIntervalMinutes) * 60 * 1000)

	url := fmt.Sprintf("%s/backend/transaction/summaries?agent-rollup-id=%s&transaction-type=Web&from=%d&to=%d&sort-order=total-time&limit=10",
		baseURL, url.QueryEscape(agentID), from, now)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var summary TransactionSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %v", err)
	}

	return &summary, nil
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
		rollups, err := fetchAgentRollups(baseURL)
		if err != nil {
			log.Printf("Error fetching agent rollups: %v", err)
			time.Sleep(time.Duration(config.Server.MetricsUpdateIntervalSeconds) * time.Second)
			continue
		}

		for _, rollup := range rollups {
			// Set metrics for the rollup info
			agentRollup.With(prometheus.Labels{
				"agent_rollup":              rollup.ID,
				"agent_rollup_display_name": rollup.Display,
			}).Set(1)

			// Remove the empty agent_id metric set
			// Fetch and set metrics for child agents
			children, err := fetchChildAgents(baseURL, rollup.ID)
			if err != nil {
				log.Printf("Error fetching child agents for %s: %v", rollup.ID, err)
				continue
			}

			for _, child := range children {
				// Set existing agent ID metric
				agentRollupID.With(prometheus.Labels{
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
				errorTotalCount.With(prometheus.Labels{
					"agent_rollup": rollup.ID,
					"agent_id":     child.ID,
				}).Set(float64(summary.Overall.ErrorCount))

				transactionTotalCount.With(prometheus.Labels{
					"agent_rollup": rollup.ID,
					"agent_id":     child.ID,
				}).Set(float64(summary.Overall.TransactionCount))

				// Set per-transaction error counts
				for _, t := range summary.Transactions {
					transactionError.With(prometheus.Labels{
						"agent_rollup":     rollup.ID,
						"agent_id":         child.ID,
						"transaction_name": t.TransactionName,
					}).Set(float64(t.ErrorCount))
				}

				// Fetch and set transaction summary metrics
				txSummary, err := fetchTransactionSummary(baseURL, child.ID)
				if err != nil {
					log.Printf("Error fetching transaction summary for agent %s: %v", child.ID, err)
					continue
				}

				// Set overall transaction count
				slowTraceTransactionTotalCount.With(prometheus.Labels{
					"agent_rollup": rollup.ID,
					"agent_id":     child.ID,
				}).Set(float64(txSummary.Overall.TransactionCount))

				// Set per-transaction counts
				for _, t := range txSummary.Transactions {
					slowTraceTransaction.With(prometheus.Labels{
						"agent_rollup":     rollup.ID,
						"agent_id":         child.ID,
						"transaction_name": t.TransactionName,
					}).Set(float64(t.TransactionCount))
				}
			}
		}

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
	log.Printf("Starting Glowroot exporter on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
