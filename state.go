package main

import "time"

// RunnerState represents the lifecycle phases of the hot reloader.
type RunnerState string

const (
	StateIdle        RunnerState = "idle"
	StateBuilding    RunnerState = "building"
	StateStarting    RunnerState = "starting"
	StateRunning     RunnerState = "running"
	StateStopping    RunnerState = "stopping"
	StateBuildFailed RunnerState = "build_failed"
)

// RunnerStatus captures the current state and diagnostic details for clients.
type RunnerStatus struct {
	State     RunnerState `json:"state"`
	Message   string      `json:"message,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}
