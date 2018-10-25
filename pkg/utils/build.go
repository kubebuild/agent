package utils

import (
	"github.com/argoproj/argo/pkg/apis/workflow/v1alpha1"
	"github.com/kubebuild/agent/pkg/types"
)

// State for db builds
type State string

// Workflow state
const (
	Scheduled State = "scheduled"
	Running   State = "running"
	Passed    State = "passed"
	Skipped   State = "skipped"
	Failed    State = "failed"
	Canceled  State = "canceled"
	Blocked   State = "blocked"
)

var stateMap = buildMap()

func buildMap() map[v1alpha1.NodePhase]State {
	states := make(map[v1alpha1.NodePhase]State)
	states[v1alpha1.NodePending] = Scheduled
	states[v1alpha1.NodeSucceeded] = Passed
	states[v1alpha1.NodeFailed] = Failed
	states[v1alpha1.NodeSkipped] = Skipped
	states[v1alpha1.NodeError] = Failed
	states[v1alpha1.NodeRunning] = Running
	return states
}

// MapPhaseToState function that maps the state from argo -> db states
func MapPhaseToState(nodePhase v1alpha1.NodePhase, isSuspended bool) types.String {
	if nodePhase == "" {
		return types.String(Scheduled)
	}
	if isSuspended {
		return types.String(Blocked)
	}
	return types.String(stateMap[nodePhase])
}
