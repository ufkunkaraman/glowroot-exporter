package compare

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// agentRollup represents the mapping between agent rollup IDs and their display names
	// Labels:
	//   - agent_rollup: The ID of the agent rollup
	//   - agent_rollup_display_name: The human-readable name of the agent rollup
	AgentRollup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "glowroot_agent_rollup",
			Help: "Information about Glowroot agent rollups",
		},
		[]string{"agent_rollup", "agent_rollup_display_name"},
	)
)

// AgentRollupJson represents the mapping between agent IDs and display names
// AgentRollup: The rollup identifier of the agent
// AgentRollupDisplayName: Human-readable display name for the agent
// Value: Numeric value associated with the agent rollup
type AgentRollupJson struct {
	AgentRollup            string `json:"agent_rollup"`
	AgentRollupDisplayName string `json:"agent_rollup_display_name"`
	Value                  int    `json:"value"`
}

// CompareAgentRollupJsons compares old and new agent rollup lists and updates Prometheus metrics
// It tracks changes in agent rollups and their display names between different time periods
func CompareAgentRollupJsons(agents, agentsNew []AgentRollupJson, agentRollup *prometheus.GaugeVec) {
	// log.Println("\n====== AgentRollupJson COMPARISON REPORT ======")

	// Map to store agent status. Key: AgentRollup_AgentRollupDisplayName, Value: Status info
	agentMap := make(map[string]struct {
		agent AgentRollupJson
		inOld bool
		inNew bool
	})

	// Add agents from old list to map
	for _, a := range agents {
		key := fmt.Sprintf("%s_%s", a.AgentRollup, a.AgentRollupDisplayName)
		agentMap[key] = struct {
			agent AgentRollupJson
			inOld bool
			inNew bool
		}{agent: a, inOld: true, inNew: false}
	}

	// Check and add agents from new list
	for _, a := range agentsNew {
		key := fmt.Sprintf("%s_%s", a.AgentRollup, a.AgentRollupDisplayName)
		if existing, exists := agentMap[key]; exists {
			agentMap[key] = struct {
				agent AgentRollupJson
				inOld bool
				inNew bool
			}{agent: existing.agent, inOld: true, inNew: true}
		} else {
			agentMap[key] = struct {
				agent AgentRollupJson
				inOld bool
				inNew bool
			}{agent: a, inOld: false, inNew: true}
		}
	}

	// Print results
	// log.Println("\n1. Common in Both Lists:")
	for _, status := range agentMap {
		if status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, ID: %s, Value: %d\n", status.agent.AgentRollup, status.agent.AgentRollupDisplayName, status.agent.Value)
			agentRollup.With(prometheus.Labels{
				"agent_rollup":              status.agent.AgentRollup,
				"agent_rollup_display_name": status.agent.AgentRollupDisplayName,
			}).Set(1)
		}
	}

	// log.Println("\n2. Only in New List (agentsNew):")
	for _, status := range agentMap {
		if !status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, ID: %s, Value: %d\n", status.agent.AgentRollup, status.agent.AgentRollupDisplayName, status.agent.Value)
			agentRollup.With(prometheus.Labels{
				"agent_rollup":              status.agent.AgentRollup,
				"agent_rollup_display_name": status.agent.AgentRollupDisplayName,
			}).Set(1)
		}
	}

	// log.Println("\n3. Only in Old List (agents):")
	for _, status := range agentMap {
		if status.inOld && !status.inNew {
			// log.Printf("   Rollup: %s, ID: %s, Value: %d\n", status.agent.AgentRollup, status.agent.AgentRollupDisplayName, status.agent.Value)
			agentRollup.DeleteLabelValues(
				status.agent.AgentRollup,
				status.agent.AgentRollupDisplayName,
			)
		}
	}
	// log.Println("\n======================================")
}
