package main

// hermes_stubs.go — Minimal stubs for types from deleted hermes_orchestrator.go
// Referenced by hermes_agent_service.go and db_hermes.go

// OrchestratorMessage — message in orchestrator history (used by db_hermes.go)
type OrchestratorMessage struct {
	Role    string
	Content string
}

// Orchestrator — minimal stub for hermes_agent_service.go
type Orchestrator struct {
	remoteManager *RemoteAgentManager
}

// AIChatSettings — stub for ai_agent_executor.go and ai_v2.go
type AIChatSettings struct {
	UserAPIKey    string
	ModelOverride string
}
