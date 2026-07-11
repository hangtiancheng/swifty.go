package teams

// CoordinatorMode restricts the Lead agent's tools to coordination-only.
// When active, Lead can only use: Agent, TaskStop, SendMessage, and
// task management tools (TaskCreate, TaskGet, TaskList, TaskUpdate).
//
// The four-phase workflow:
// 1. Research: Lead explores the problem space
// 2. Synthesis: Lead creates a plan and task decomposition
// 3. Implementation: Lead spawns teammates to execute tasks
// 4. Verification: Lead verifies results and resolves conflicts

var CoordinatorAllowedTools = map[string]bool{
	"Agent":       true,
	"SendMessage": true,
	"TaskCreate":  true,
	"TaskGet":     true,
	"TaskList":    true,
	"TaskUpdate":  true,

	"TeamCreate": true,
	"TeamDelete": true,
	"ReadFile":   true,
	"Glob":       true,
	"Grep":       true,
	"Bash":       true,
}

// IsCoordinatorTool checks if a tool is allowed in Coordinator Mode.
func IsCoordinatorTool(name string) bool {
	return CoordinatorAllowedTools[name]
}

// CoordinatorToolFilter returns the tool filter function for the Lead agent.
// When enabled is false, returns nil (no restriction); when true, restricts the tool set when a Team exists.
func CoordinatorToolFilter(teamMgr *TeamManager, enabled bool) func(name string) bool {
	if teamMgr == nil || !enabled {
		return nil
	}
	return func(name string) bool {
		if len(teamMgr.ListTeams()) == 0 {
			return true
		}
		return IsCoordinatorTool(name)
	}
}
