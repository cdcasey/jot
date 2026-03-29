package watch

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/chris/jot/internal/db"
	"github.com/chris/jot/internal/llm"
)

const extractionSystemPrompt = `You are an extraction assistant. You will receive text content scraped from one or more web pages, along with instructions on what to extract.

Return ONLY a JSON array of objects. Each object must have these fields:
- "title": short identifying name for the item (required)
- "body": relevant details, summary, or description (optional, empty string if none)
- "source_url": the URL this item came from, if identifiable (optional, empty string if unknown)

If no matching items are found, return an empty array: []

Do not include any text outside the JSON array. No markdown, no explanation, just the JSON.`

// Runner coordinates watch execution: fetch → extract → dedup → store.
type Runner struct {
	db     *db.DB
	client llm.Client
}

// NewRunner creates a Runner with the given dependencies.
func NewRunner(database *db.DB, client llm.Client) *Runner {
	return &Runner{db: database, client: client}
}

// extractedItem is what the LLM returns for each found item.
type extractedItem struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	SourceURL string `json:"source_url"`
}

// RunWatch executes a single watch: fetches URLs, extracts items via LLM,
// deduplicates against stored results, and returns only the new ones.
func (r *Runner) RunWatch(ctx context.Context, w db.Watch) ([]db.WatchResult, error) {
	if len(w.URLs) == 0 {
		return nil, fmt.Errorf("watch %q has no URLs", w.Name)
	}

	// 1. Fetch all URLs.
	fetched := Fetch(w.URLs)

	// Build the content block for the LLM. Include all successful fetches,
	// log failures but don't abort — partial results are still useful.
	var contentParts []string
	for _, f := range fetched {
		if f.Err != nil {
			log.Printf("watch[%s]: fetch error for %s: %v", w.Name, f.URL, f.Err)
			continue
		}
		if strings.TrimSpace(f.Text) == "" {
			log.Printf("watch[%s]: empty content from %s", w.Name, f.URL)
			continue
		}
		contentParts = append(contentParts, fmt.Sprintf("--- Content from %s ---\n%s", f.URL, f.Text))
	}

	if len(contentParts) == 0 {
		return nil, fmt.Errorf("watch %q: all URL fetches failed or returned empty content", w.Name)
	}

	// 2. Ask the LLM to extract items.
	userMessage := fmt.Sprintf("%s\n\n%s", w.Prompt, strings.Join(contentParts, "\n\n"))
	messages := []llm.Message{{Role: "user", Content: userMessage}}

	resp, err := r.client.Chat(ctx, extractionSystemPrompt, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction: %w", err)
	}

	items, err := parseExtractedItems(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w", err)
	}

	if len(items) == 0 {
		log.Printf("watch[%s]: LLM found no items", w.Name)
		if err := r.db.RecordWatchRun(w.ID); err != nil {
			log.Printf("watch[%s]: recording run: %v", w.Name, err)
		}
		return nil, nil
	}

	// 3. Dedup and store.
	var newResults []db.WatchResult
	for _, item := range items {
		hash := contentHash(item.Title)
		id, err := r.db.SaveWatchResult(w.ID, hash, item.Title, item.Body, item.SourceURL)
		if err != nil {
			log.Printf("watch[%s]: saving result %q: %v", w.Name, item.Title, err)
			continue
		}
		if id == 0 {
			continue // duplicate
		}
		newResults = append(newResults, db.WatchResult{
			ID:          id,
			WatchID:     w.ID,
			ContentHash: hash,
			Title:       item.Title,
			Body:        item.Body,
			SourceURL:   item.SourceURL,
		})
	}

	if err := r.db.RecordWatchRun(w.ID); err != nil {
		log.Printf("watch[%s]: recording run: %v", w.Name, err)
	}

	log.Printf("watch[%s]: found %d items, %d new", w.Name, len(items), len(newResults))
	return newResults, nil
}

// parseExtractedItems parses the LLM's JSON response into extracted items.
// Tolerates markdown code fences around the JSON.
func parseExtractedItems(raw string) ([]extractedItem, error) {
	cleaned := strings.TrimSpace(raw)
	// Strip markdown code fences if the LLM wraps them.
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var items []extractedItem
	if err := json.Unmarshal([]byte(cleaned), &items); err != nil {
		return nil, fmt.Errorf("invalid JSON from LLM: %w\nraw response: %s", err, truncate(raw, 500))
	}
	return items, nil
}

// contentHash returns a SHA-256 hex digest of the normalized title.
func contentHash(title string) string {
	normalized := strings.ToLower(strings.TrimSpace(title))
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
