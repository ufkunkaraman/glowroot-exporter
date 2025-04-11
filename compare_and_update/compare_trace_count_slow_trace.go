package compare

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// TraceCountSlowTraceJson represents the count of slow traces for an agent
// AgentRollup: The rollup identifier of the agent
// AgentID: The unique identifier of the agent
// Value: Number of slow traces detected
type TraceCountSlowTraceJson struct {
	AgentRollup string `json:"agent_rollup"`
	AgentID     string `json:"agent_id"`
	Value       int    `json:"value"`
}

// CompareTraceCountSlowTraceJsons compares old and new slow trace counts and updates Prometheus metrics
// It monitors changes in slow trace counts between different time periods
func CompareTraceCountSlowTraceJsons(agents, agentsNew []TraceCountSlowTraceJson, traceCountSlowTrace *prometheus.GaugeVec) {
	// log.Println("\n====== TraceCountSlowTraceJson COMPARISON REPORT ======")

	// Map to store agent status. Key: AgentRollup_AgentID, Value: Status info
	agentMap := make(map[string]struct {
		agent TraceCountSlowTraceJson
		inOld bool
		inNew bool
	})

	// Add agents from old list to map
	for _, a := range agents {
		key := fmt.Sprintf("%s_%s", a.AgentRollup, a.AgentID)
		agentMap[key] = struct {
			agent TraceCountSlowTraceJson
			inOld bool
			inNew bool
		}{agent: a, inOld: true, inNew: false}
	}

	// Check and add agents from new list
	for _, a := range agentsNew {
		key := fmt.Sprintf("%s_%s", a.AgentRollup, a.AgentID)
		if existing, exists := agentMap[key]; exists {
			agentMap[key] = struct {
				agent TraceCountSlowTraceJson
				inOld bool
				inNew bool
			}{agent: existing.agent, inOld: true, inNew: true}
		} else {
			agentMap[key] = struct {
				agent TraceCountSlowTraceJson
				inOld bool
				inNew bool
			}{agent: a, inOld: false, inNew: true}
		}
	}

	// Print results
	// log.Println("\n1. Common in Both Lists:")
	for _, status := range agentMap {
		if status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, ID: %s, Value: %d\n", status.agent.AgentRollup, status.agent.AgentID, status.agent.Value)
			traceCountSlowTrace.With(prometheus.Labels{
				"agent_rollup": status.agent.AgentRollup,
				"agent_id":     status.agent.AgentID,
			}).Set(float64(status.agent.Value))

		}
	}

	// log.Println("\n2. Only in New List (agentsNew):")
	for _, status := range agentMap {
		if !status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, ID: %s, Value: %d\n", status.agent.AgentRollup, status.agent.AgentID, status.agent.Value)
			traceCountSlowTrace.With(prometheus.Labels{
				"agent_rollup": status.agent.AgentRollup,
				"agent_id":     status.agent.AgentID,
			}).Set(float64(status.agent.Value))
		}
	}

	// log.Println("\n3. Only in Old List (agents):")
	for _, status := range agentMap {
		if status.inOld && !status.inNew {
			// log.Printf("   Rollup: %s, ID: %s, Value: %d\n", status.agent.AgentRollup, status.agent.AgentID, status.agent.Value)
			traceCountSlowTrace.DeleteLabelValues(
				status.agent.AgentRollup,
				status.agent.AgentID,
			)
		}
	}
	// log.Println("\n======================================")
}
