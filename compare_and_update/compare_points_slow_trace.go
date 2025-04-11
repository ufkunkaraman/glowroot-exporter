package compare

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// PointsSlowTraceJson represents detailed information about slow traces
// AgentRollup: The rollup identifier of the agent
// AgentID: The unique identifier of the agent
// TraceID: Unique identifier for the trace
// Status: Current status of the trace
// TransactionName: Name of the transaction being traced
// Value: Performance metric value (usually duration)
type PointsSlowTraceJson struct {
	AgentRollup     string  `json:"agent_rollup"`
	AgentID         string  `json:"agent_id"`
	TraceID         string  `json:"trace_id"`
	Status          string  `json:"status"`
	TransactionName string  `json:"transactionName"`
	Value           float64 `json:"value"`
}

// ComparePointsSlowTraceJsons compares old and new slow trace points and updates Prometheus metrics
// It tracks slow traces and their performance metrics over time
func ComparePointsSlowTraceJsons(oldList, newList []PointsSlowTraceJson, pointsSlowTrace *prometheus.GaugeVec) {
	// log.Println("\n====== POINTS SLOW TRACE COMPARISON REPORT ======")

	// Map to store trace status. Key: AgentRollup_AgentID_TraceID, Value: Status info
	traceMap := make(map[string]struct {
		trace PointsSlowTraceJson
		inOld bool
		inNew bool
	})

	// Add traces from old list to map
	for _, t := range oldList {
		key := fmt.Sprintf("%s_%s_%s", t.AgentRollup, t.AgentID, t.TraceID)
		traceMap[key] = struct {
			trace PointsSlowTraceJson
			inOld bool
			inNew bool
		}{trace: t, inOld: true, inNew: false}
	}

	// Check and update with new list
	for _, t := range newList {
		key := fmt.Sprintf("%s_%s_%s", t.AgentRollup, t.AgentID, t.TraceID)
		if existing, exists := traceMap[key]; exists {
			traceMap[key] = struct {
				trace PointsSlowTraceJson
				inOld bool
				inNew bool
			}{trace: existing.trace, inOld: true, inNew: true}
		} else {
			traceMap[key] = struct {
				trace PointsSlowTraceJson
				inOld bool
				inNew bool
			}{trace: t, inOld: false, inNew: true}
		}
	}

	// log.Println("\n1. Common in Both Lists:")
	for _, status := range traceMap {
		if status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, AgentID: %s, TraceID: %s\n   Status: %s, Transaction: %s, Value: %f\n", status.trace.AgentRollup, status.trace.AgentID, status.trace.TraceID, status.trace.Status, status.trace.TransactionName, status.trace.Value)
			pointsSlowTrace.With(prometheus.Labels{
				"agent_rollup":     status.trace.AgentRollup,
				"agent_id":         status.trace.AgentID,
				"trace_id":         status.trace.TraceID,
				"status":           status.trace.Status,
				"transaction_name": status.trace.TransactionName,
			}).Set(float64(status.trace.Value))
		}
	}

	// log.Println("\n2. Only in New List:")
	for _, status := range traceMap {
		if !status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, AgentID: %s, TraceID: %s\n   Status: %s, Transaction: %s, Value: %f\n", status.trace.AgentRollup, status.trace.AgentID, status.trace.TraceID, status.trace.Status, status.trace.TransactionName, status.trace.Value)
			pointsSlowTrace.With(prometheus.Labels{
				"agent_rollup":     status.trace.AgentRollup,
				"agent_id":         status.trace.AgentID,
				"trace_id":         status.trace.TraceID,
				"status":           status.trace.Status,
				"transaction_name": status.trace.TransactionName,
			}).Set(float64(status.trace.Value))

		}
	}

	// log.Println("\n3. Only in Old List:")
	for _, status := range traceMap {
		if status.inOld && !status.inNew {
			// log.Printf("   Rollup: %s, AgentID: %s, TraceID: %s\n   Status: %s, Transaction: %s, Value: %f\n", status.trace.AgentRollup, status.trace.AgentID, status.trace.TraceID, status.trace.Status, status.trace.TransactionName, status.trace.Value)
			pointsSlowTrace.DeleteLabelValues(
				status.trace.AgentRollup,
				status.trace.AgentID,
				status.trace.TraceID,
				status.trace.Status,
				status.trace.TransactionName,
			)
		}
	}
	// log.Println("\n======================================")
}
