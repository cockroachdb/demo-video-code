package models

type AgentType string

const (
	AgentTypeAnomalyDetection AgentType = "anomaly_detection"
	AgentTypeReasoning        AgentType = "reasoning"
	AgentTypeNotification     AgentType = "notification"
)
