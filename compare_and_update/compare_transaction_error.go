package compare

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// TransactionErrorJson represents error information for a specific transaction
// AgentRollup: The rollup identifier of the agent
// AgentID: The unique identifier of the agent
// Transaction: Name of the transaction
// Value: Number of errors in the transaction
type TransactionErrorJson struct {
	AgentRollup string `json:"agent_rollup"`
	AgentID     string `json:"agent_id"`
	Transaction string `json:"transaction_name"`
	Value       int    `json:"value"`
}

// CompareTransactionErrorJsons compares old and new transaction error lists and updates Prometheus metrics
// It monitors changes in transaction errors across different time periods
func CompareTransactionErrorJsons(oldList, newList []TransactionErrorJson, transactionError *prometheus.GaugeVec) {
	// log.Println("\n====== TransactionErrorJson COMPARISON REPORT ======")

	// Map to store transaction status. Key: AgentRollup_AgentID_Transaction, Value: Status info
	transMap := make(map[string]struct {
		trans TransactionErrorJson
		inOld bool
		inNew bool
	})

	// Add transactions from old list to map
	for _, t := range oldList {
		key := fmt.Sprintf("%s_%s_%s", t.AgentRollup, t.AgentID, t.Transaction)
		transMap[key] = struct {
			trans TransactionErrorJson
			inOld bool
			inNew bool
		}{trans: t, inOld: true, inNew: false}
	}

	// Check and update with new list
	for _, t := range newList {
		key := fmt.Sprintf("%s_%s_%s", t.AgentRollup, t.AgentID, t.Transaction)
		if existing, exists := transMap[key]; exists {
			transMap[key] = struct {
				trans TransactionErrorJson
				inOld bool
				inNew bool
			}{trans: existing.trans, inOld: true, inNew: true}
		} else {
			transMap[key] = struct {
				trans TransactionErrorJson
				inOld bool
				inNew bool
			}{trans: t, inOld: false, inNew: true}
		}
	}

	// log.Println("\n1. Common in Both Lists:")
	for _, status := range transMap {
		if status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, AgentID: %s, Transaction: %s, Value: %d\n", status.trans.AgentRollup, status.trans.AgentID, status.trans.Transaction, status.trans.Value)
			transactionError.With(prometheus.Labels{
				"agent_rollup":     status.trans.AgentRollup,
				"agent_id":         status.trans.AgentID,
				"transaction_name": status.trans.Transaction,
			}).Set(float64(status.trans.Value))
		}
	}

	// log.Println("\n2. Only in New List:")
	for _, status := range transMap {
		if !status.inOld && status.inNew {
			// log.Printf("   Rollup: %s, AgentID: %s, Transaction: %s, Value: %d\n", status.trans.AgentRollup, status.trans.AgentID, status.trans.Transaction, status.trans.Value)
			transactionError.With(prometheus.Labels{
				"agent_rollup":     status.trans.AgentRollup,
				"agent_id":         status.trans.AgentID,
				"transaction_name": status.trans.Transaction,
			}).Set(float64(status.trans.Value))
		}
	}

	// log.Println("\n3. Only in Old List:")
	for _, status := range transMap {
		if status.inOld && !status.inNew {
			// log.Printf("   Rollup: %s, AgentID: %s, Transaction: %s, Value: %d\n", status.trans.AgentRollup, status.trans.AgentID, status.trans.Transaction, status.trans.Value)
			transactionError.DeleteLabelValues(
				status.trans.AgentRollup,
				status.trans.AgentID,
				status.trans.Transaction,
			)

		}
	}
	// log.Println("\n======================================")
}
