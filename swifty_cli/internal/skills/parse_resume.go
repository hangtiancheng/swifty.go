package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/tools"
)

func init() {
	builtinToolFactories["parse_resume"] = func(schema ToolSchema) tools.Tool {
		return &parseResumeTool{schema: schema}
	}
}

// parseResumeTool is the compiled-in implementation of the parse_resume tool
// declared in fullstack-interview/tool.json. It does a light pass over a
// resume file and extracts structured signal: primary role, frontend stack,
// backend stack, projects, and years of experience. The output is fed back
// to the fullstack-interview SOP so the model can tailor questions without
// re-reading the raw resume on every turn.
type parseResumeTool struct {
	schema ToolSchema
}

func (t *parseResumeTool) Name() string { return t.schema.Name }

func (t *parseResumeTool) Description() string { return t.schema.Description }

func (t *parseResumeTool) Category() tools.ToolCategory { return tools.CategoryRead }

func (t *parseResumeTool) Schema() map[string]any {
	return map[string]any{
		"name":         t.schema.Name,
		"description":  t.schema.Description,
		"input_schema": t.schema.InputSchema,
	}
}

func (t *parseResumeTool) Execute(_ context.Context, args map[string]any) tools.ToolResult {
	path, _ := args["file_path"].(string)
	if path == "" {
		return tools.ToolResult{Output: "file_path is required", IsError: true}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return tools.ToolResult{Output: fmt.Sprintf("read resume: %v", err), IsError: true}
	}
	signal := extractResumeSignals(string(raw))
	out, _ := json.MarshalIndent(signal, "", "  ")
	return tools.ToolResult{Output: string(out)}
}

// resumeSignal carries the structured extraction result used by the
// fullstack-interview SOP. primary_role is one of "frontend", "backend",
// or "fullstack" and drives which question pools the interviewer draws from.
type resumeSignal struct {
	PrimaryRole       string   `json:"primary_role"`
	FrontendStack     []string `json:"frontend_stack"`
	BackendStack      []string `json:"backend_stack"`
	GeneralStack      []string `json:"general_stack"`
	Projects          []string `json:"projects"`
	YearsOfExperience int      `json:"years_of_experience"`
}

// frontendKeywords and backendKeywords are the detection pools. A keyword
// match pushes the candidate's score toward that track; the dominant track
// becomes primary_role. Ordering within each list does not matter -- we
// check presence, not frequency.
var (
	frontendKeywords = []string{
		"BOM", "CSS", "CSS Modules", "DOM", "Express", "GraphQL", "HTML", "Hono", "JavaScript", "Jest", "Jotai", "Koa", "MobX", "Next.js", "Node", "Node.js", "PWA", "Playwright", "REST API", "React", "Rollup", "SEO", "SPA", "SSE", "SSG", "SSR", "Sass", "Service Worker", "Shadow DOM", "TailwindCSS", "Testing Library", "TypeScript", "Vite", "Vitest", "Vue", "Web Components", "Web Workers", "WebSocket", "Webpack", "Zustand", "esbuild", "styled-components",
	}

	backendKeywords = []string{
		"Bash", "C++", "CDN", "Cache", "Circuit Breaker", "ClickHouse", "Consensus", "Distributed", "Docker", "Elasticsearch", "Go", "Golang", "Grafana", "Index Design", "Kafka", "Kubernetes", "Linux", "Microservices", "MongoDB", "MySQL", "MySQL Tuning", "Nginx", "OpenTelemetry", "PostgreSQL", "Prometheus", "Protobuf", "Query Optimization", "Raft", "Rate Limiting", "Redis", "Service Mesh", "Shell", "Thrift", "gRPC", "systemd",
	}

	// generalKeywords appear in both tracks and do not influence
	// primary_role scoring but are still surfaced for question tailoring.
	generalKeywords = []string{
		"REST", "GraphQL", "WebSocket", "CI/CD", "Git",
		"Agile", "Scrum", "TDD", "BDD", "DDD",
		"System Design", "Architecture",
	}

	yoeRegex     = regexp.MustCompile(`(?i)(\d{1,2})\s*\+?\s*(years?|yrs?)`)
	projectRegex = regexp.MustCompile(`(?im)^\s*(?:[-*•·]|\d+\.)\s+(.{8,140})$`)
)

// extractResumeSignals does a naive single-pass extraction. The interview
// SOP only needs rough signal (which techs to ask about, which projects
// look substantial, what the candidate's primary track is); we don't aim
// for full NER here.
func extractResumeSignals(text string) resumeSignal {
	lower := strings.ToLower(text)

	var sig resumeSignal
	seen := map[string]bool{}

	var feScore, beScore int

	for _, kw := range frontendKeywords {
		if !seen[kw] && strings.Contains(lower, strings.ToLower(kw)) {
			sig.FrontendStack = append(sig.FrontendStack, kw)
			seen[kw] = true
			feScore++
		}
	}

	for _, kw := range backendKeywords {
		if !seen[kw] && strings.Contains(lower, strings.ToLower(kw)) {
			sig.BackendStack = append(sig.BackendStack, kw)
			seen[kw] = true
			beScore++
		}
	}

	for _, kw := range generalKeywords {
		if !seen[kw] && strings.Contains(lower, strings.ToLower(kw)) {
			sig.GeneralStack = append(sig.GeneralStack, kw)
			seen[kw] = true
		}
	}

	switch {
	case feScore > 0 && beScore > 0 && feScore >= beScore/2 && beScore >= feScore/2:
		sig.PrimaryRole = "fullstack"
	case feScore > beScore:
		sig.PrimaryRole = "frontend"
	case beScore > 0:
		sig.PrimaryRole = "backend"
	default:
		sig.PrimaryRole = "fullstack"
	}

	if m := yoeRegex.FindStringSubmatch(text); len(m) >= 2 {
		var n int
		_, _ = fmt.Sscanf(m[1], "%d", &n)
		sig.YearsOfExperience = n
	}

	matches := projectRegex.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		line := strings.TrimSpace(m[1])
		if len(line) < 10 {
			continue
		}
		sig.Projects = append(sig.Projects, line)
		if len(sig.Projects) >= 8 {
			break
		}
	}

	return sig
}
