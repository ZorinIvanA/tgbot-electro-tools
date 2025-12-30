package metrics

import (
	"fmt"
	"strings"

	"github.com/ZorinIvanA/tgbot-electro-tools/internal/storage"
)

// Collector collects and exports metrics
type Collector struct {
	storage storage.Storage
}

// NewCollector creates a new metrics collector
func NewCollector(storage storage.Storage) *Collector {
	return &Collector{
		storage: storage,
	}
}

// Export exports metrics in Prometheus text format
func (c *Collector) Export() (string, error) {
	var sb strings.Builder

	// Active users in last 24h
	activeUsers, err := c.storage.GetActiveUsersCount24h()
	if err != nil {
		return "", fmt.Errorf("failed to get active users count: %w", err)
	}

	sb.WriteString("# HELP telegram_bot_active_users_total Number of unique users in last 24h\n")
	sb.WriteString("# TYPE telegram_bot_active_users_total gauge\n")
	sb.WriteString(fmt.Sprintf("telegram_bot_active_users_total{period=\"24h\"} %d\n", activeUsers))
	sb.WriteString("\n")

	// Total messages count
	totalMessages, err := c.storage.GetTotalMessagesCount()
	if err != nil {
		return "", fmt.Errorf("failed to get total messages count: %w", err)
	}

	sb.WriteString("# HELP telegram_bot_messages_total Total messages processed\n")
	sb.WriteString("# TYPE telegram_bot_messages_total counter\n")
	sb.WriteString(fmt.Sprintf("telegram_bot_messages_total %d\n", totalMessages))
	sb.WriteString("\n")

	// Users per FSM state
	usersByState, err := c.storage.GetUsersByFSMState()
	if err != nil {
		return "", fmt.Errorf("failed to get users by FSM state: %w", err)
	}

	sb.WriteString("# HELP telegram_bot_fsm_state Users per FSM state\n")
	sb.WriteString("# TYPE telegram_bot_fsm_state gauge\n")
	for state, count := range usersByState {
		sb.WriteString(fmt.Sprintf("telegram_bot_fsm_state{state=\"%s\"} %d\n", state, count))
	}

	return sb.String(), nil
}

// PrometheusMetric represents a single metric
type PrometheusMetric struct {
	Name   string
	Help   string
	Type   string
	Value  interface{}
	Labels map[string]string
}

// FormatMetric formats a metric in Prometheus text format
func FormatMetric(metric PrometheusMetric) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# HELP %s %s\n", metric.Name, metric.Help))
	sb.WriteString(fmt.Sprintf("# TYPE %s %s\n", metric.Name, metric.Type))

	if len(metric.Labels) > 0 {
		labelPairs := make([]string, 0, len(metric.Labels))
		for k, v := range metric.Labels {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=\"%s\"", k, v))
		}
		sb.WriteString(fmt.Sprintf("%s{%s} %v\n", metric.Name, strings.Join(labelPairs, ","), metric.Value))
	} else {
		sb.WriteString(fmt.Sprintf("%s %v\n", metric.Name, metric.Value))
	}

	return sb.String()
}
