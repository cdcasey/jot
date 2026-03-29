package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/chris/jot/internal/agent"
	"github.com/chris/jot/internal/db"
	"github.com/chris/jot/internal/llm"
)

// EvalCase defines a single evaluation scenario.
type EvalCase struct {
	Name     string   `json:"name"`
	Category string   `json:"category"` // tool_reliability, context_integration, reasoning
	Prompt   string   `json:"prompt"`
	Seed     SeedData `json:"seed"`
	Assert   Assert   `json:"assert"`
}

// SeedData populates the in-memory DB before the agent runs.
type SeedData struct {
	Things   []SeedThing       `json:"things,omitempty"`
	Memories []SeedMemory      `json:"memories,omitempty"`
	Notes    map[string]string `json:"notes,omitempty"`
}

type SeedThing struct {
	Title    string   `json:"title"`
	Notes    string   `json:"notes,omitempty"`
	Status   string   `json:"status,omitempty"`
	Priority string   `json:"priority,omitempty"`
	DueDate  string   `json:"due_date,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

type SeedMemory struct {
	Content  string   `json:"content"`
	Category string   `json:"category"`
	Tags     []string `json:"tags,omitempty"`
}

// Assert defines what to check after the agent responds.
type Assert struct {
	ToolCalled       []string `json:"tool_called,omitempty"`     // ALL of these must be called
	ToolCalledAny    []string `json:"tool_called_any,omitempty"` // at least ONE of these must be called
	ToolNotCalled    []string `json:"tool_not_called,omitempty"`
	ResponseContains []string `json:"response_contains,omitempty"`
	Rubric           string   `json:"rubric,omitempty"`
}

// CaseResult holds the outcome of a single eval case.
type CaseResult struct {
	Name     string
	Category string
	Passed   bool     // for tool_reliability (pass/fail)
	Score    int      // for rubric-scored cases (1-5)
	Reason   string   // judge reasoning or failure detail
	Response string   // the agent's full response
	Tools    []string // tools that were called
}

// LoadCases reads eval cases from a JSON file.
func LoadCases(path string) ([]EvalCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cases file: %w", err)
	}
	var cases []EvalCase
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("parsing cases JSON: %w", err)
	}
	return cases, nil
}

// RunEval executes all eval cases and prints results.
// agentClient runs the agent under test; judgeClient scores rubric-based cases.
// They can be the same client or different (e.g., Haiku as agent, Sonnet as judge).
func RunEval(t *testing.T, casePath string, agentClient, judgeClient llm.Client, model, judgeModel string) {
	cases, err := LoadCases(casePath)
	if err != nil {
		t.Fatalf("loading cases: %v", err)
	}

	// Preflight: verify both models are reachable before running cases.
	ctx := context.Background()
	ping := []llm.Message{{Role: "user", Content: "ping"}}

	if _, err := agentClient.Chat(ctx, "Respond with pong.", ping, nil); err != nil {
		t.Fatalf("agent model %q not reachable: %v", model, err)
	}

	hasJudge := false
	for _, ec := range cases {
		if ec.Assert.Rubric != "" {
			hasJudge = true
			break
		}
	}
	if hasJudge {
		if _, err := judgeClient.Chat(ctx, "Respond with pong.", ping, nil); err != nil {
			t.Fatalf("judge model %q not reachable: %v", judgeModel, err)
		}
	}

	var results []CaseResult
	for _, ec := range cases {
		t.Run(ec.Name, func(t *testing.T) {
			result := runCase(t, ec, agentClient, judgeClient)
			results = append(results, result)
		})
	}

	printResults(results, model, judgeModel)
}

func runCase(t *testing.T, ec EvalCase, agentClient, judgeClient llm.Client) CaseResult {
	// Fresh in-memory DB for each case.
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}
	defer database.Close()

	// Seed data.
	seedDB(t, database, ec.Seed)

	// Run the agent.
	a := agent.New(database, agentClient, 180000)
	ctx := context.Background()
	response, history, err := a.Run(ctx, nil, ec.Prompt)
	if err != nil {
		t.Fatalf("agent.Run: %v", err)
	}

	// Collect tool calls from history.
	toolsCalled := collectToolCalls(history)

	result := CaseResult{
		Name:     ec.Name,
		Category: ec.Category,
		Response: response,
		Tools:    toolsCalled,
	}

	// Check pass/fail assertions.
	if len(ec.Assert.ToolCalled) > 0 || len(ec.Assert.ToolCalledAny) > 0 || len(ec.Assert.ToolNotCalled) > 0 || len(ec.Assert.ResponseContains) > 0 {
		pass, reason := checkAssertions(ec.Assert, toolsCalled, response)
		result.Passed = pass
		result.Reason = reason
		if !pass {
			t.Errorf("FAIL: %s", reason)
		}
	}

	// LLM-as-judge scoring.
	if ec.Assert.Rubric != "" {
		score, reason, err := judgeResponse(ctx, judgeClient, ec.Prompt, response, ec.Assert.Rubric)
		if err != nil {
			t.Errorf("judge error: %v", err)
			result.Score = 0
			result.Reason = fmt.Sprintf("judge error: %v", err)
		} else {
			result.Score = score
			if result.Reason != "" {
				result.Reason += "; " + reason
			} else {
				result.Reason = reason
			}
		}
	}

	return result
}

func seedDB(t *testing.T, database *db.DB, seed SeedData) {
	for _, thing := range seed.Things {
		_, err := database.CreateThing(thing.Title, thing.Notes, thing.Priority, thing.DueDate, thing.Tags)
		if err != nil {
			t.Fatalf("seeding thing %q: %v", thing.Title, err)
		}
	}
	for _, mem := range seed.Memories {
		_, err := database.SaveMemory(mem.Content, mem.Category, "eval", mem.Tags, nil, "")
		if err != nil {
			t.Fatalf("seeding memory: %v", err)
		}
	}
	for key, value := range seed.Notes {
		if err := database.SetNote(key, value); err != nil {
			t.Fatalf("seeding note %q: %v", key, err)
		}
	}
}

func collectToolCalls(history []llm.Message) []string {
	seen := make(map[string]bool)
	var tools []string
	for _, msg := range history {
		for _, tc := range msg.ToolCalls {
			if !seen[tc.Name] {
				seen[tc.Name] = true
				tools = append(tools, tc.Name)
			}
		}
	}
	return tools
}

func checkAssertions(assert Assert, toolsCalled []string, response string) (bool, string) {
	toolSet := make(map[string]bool)
	for _, t := range toolsCalled {
		toolSet[t] = true
	}

	var failures []string

	for _, required := range assert.ToolCalled {
		if !toolSet[required] {
			failures = append(failures, fmt.Sprintf("tool %s not called", required))
		}
	}
	if len(assert.ToolCalledAny) > 0 {
		found := false
		for _, t := range assert.ToolCalledAny {
			if toolSet[t] {
				found = true
				break
			}
		}
		if !found {
			failures = append(failures, fmt.Sprintf("none of [%s] called", strings.Join(assert.ToolCalledAny, ", ")))
		}
	}
	for _, forbidden := range assert.ToolNotCalled {
		if toolSet[forbidden] {
			failures = append(failures, fmt.Sprintf("tool %s was called (forbidden)", forbidden))
		}
	}
	for _, substr := range assert.ResponseContains {
		if !strings.Contains(strings.ToLower(response), strings.ToLower(substr)) {
			failures = append(failures, fmt.Sprintf("response missing %q", substr))
		}
	}

	if len(failures) > 0 {
		return false, strings.Join(failures, "; ")
	}
	return true, toolsDetail(assert.ToolCalled, toolSet)
}

func toolsDetail(required []string, called map[string]bool) string {
	var parts []string
	for _, t := range required {
		if called[t] {
			parts = append(parts, t+" ✓")
		} else {
			parts = append(parts, t+" ✗")
		}
	}
	if len(parts) == 0 {
		return "passed"
	}
	return "tools: " + strings.Join(parts, ", ")
}

const judgeSystemPrompt = `You are an evaluation judge. Score a personal assistant's response on a 1-5 scale.

You will receive:
- The user's prompt
- The assistant's response
- A rubric describing what a good response looks like

Scoring guide:
1 = Completely misses the point or gives harmful advice
2 = Partially relevant but shallow or generic
3 = Adequate — addresses the prompt but nothing special
4 = Good — thoughtful, specific, engages with nuance
5 = Excellent — insightful, well-structured, would genuinely help the user

Respond with ONLY a JSON object, no other text:
{"score": <1-5>, "reasoning": "<1-2 sentences explaining the score>"}`

func judgeResponse(ctx context.Context, client llm.Client, prompt, response, rubric string) (int, string, error) {
	judgePrompt := fmt.Sprintf("User prompt: %s\n\nAssistant response: %s\n\nRubric: %s", prompt, response, rubric)

	resp, err := client.Chat(ctx, judgeSystemPrompt, []llm.Message{
		{Role: "user", Content: judgePrompt},
	}, nil) // no tools for judging
	if err != nil {
		return 0, "", fmt.Errorf("judge LLM call: %w", err)
	}

	// Parse the JSON response.
	var verdict struct {
		Score     int    `json:"score"`
		Reasoning string `json:"reasoning"`
	}

	// Try to extract JSON from the response (LLM might wrap it in markdown).
	content := resp.Content
	if idx := strings.Index(content, "{"); idx >= 0 {
		if end := strings.LastIndex(content, "}"); end > idx {
			content = content[idx : end+1]
		}
	}

	decoder := json.NewDecoder(strings.NewReader(content))
	if err := decoder.Decode(&verdict); err != nil {
		return 0, "", fmt.Errorf("parsing judge response %q: %w", resp.Content, err)
	}

	if verdict.Score < 1 || verdict.Score > 5 {
		return 0, "", fmt.Errorf("judge returned invalid score: %d", verdict.Score)
	}

	return verdict.Score, verdict.Reasoning, nil
}

func printResults(results []CaseResult, model, judgeModel string) {
	fmt.Println()
	fmt.Printf("  Model: %s\n", model)
	fmt.Printf("  Judge: %s\n", judgeModel)
	fmt.Println("──────────────────────────────────────────────────────────────")

	var toolPass, toolTotal int
	var contextSum, contextCount float64
	var reasonSum, reasonCount float64

	for _, r := range results {
		switch r.Category {
		case "tool_reliability":
			toolTotal++
			if r.Passed {
				toolPass++
				fmt.Printf(" PASS  %-30s %s\n", r.Name, r.Reason)
			} else {
				fmt.Printf(" FAIL  %-30s %s\n", r.Name, r.Reason)
			}
		case "context_integration":
			contextCount++
			contextSum += float64(r.Score)
			fmt.Printf(" %d/5   %-30s %q\n", r.Score, r.Name, r.Reason)
		case "reasoning":
			reasonCount++
			reasonSum += float64(r.Score)
			fmt.Printf(" %d/5   %-30s %q\n", r.Score, r.Name, r.Reason)
		}
	}

	fmt.Println("──────────────────────────────────────────────────────────────")
	if toolTotal > 0 {
		fmt.Printf("Tool reliability:      %d/%d passed\n", toolPass, toolTotal)
	}
	if contextCount > 0 {
		fmt.Printf("Context integration:   %.1f/5.0 avg\n", contextSum/contextCount)
	}
	if reasonCount > 0 {
		fmt.Printf("Reasoning:             %.1f/5.0 avg\n", reasonSum/reasonCount)
	}
	fmt.Println()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
