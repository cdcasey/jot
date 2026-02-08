package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/chris/jot/internal/agent"
	"github.com/chris/jot/internal/db"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron       *cron.Cron
	webhookURL string
	db         *db.DB
	agent      *agent.Agent
}

func New(schedule, webhookURL string, database *db.DB, ag *agent.Agent) *Scheduler {
	c := cron.New()
	s := &Scheduler{
		cron:       c,
		webhookURL: webhookURL,
		db:         database,
		agent:      ag,
	}

	_, err := c.AddFunc(schedule, s.runCheckIn)
	if err != nil {
		log.Printf("invalid cron schedule %q: %v", schedule, err)
	}

	return s
}

func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("scheduler started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

func (s *Scheduler) runCheckIn() {
	prompt, err := agent.BuildCheckInPrompt(s.db)
	if err != nil {
		log.Printf("check-in error building prompt: %v", err)
		return
	}

	reply, _, err := s.agent.Run(context.Background(), nil, prompt)
	if err != nil {
		log.Printf("check-in error from agent: %v", err)
		return
	}

	// Store the check-in
	if _, err := s.db.CreateCheckIn(reply); err != nil {
		log.Printf("check-in error storing: %v", err)
	}

	// Record check-in as a system memory
	memContent := reply
	if len(memContent) > 200 {
		memContent = memContent[:200]
	}
	if _, err := s.db.SaveMemory(
		fmt.Sprintf("Check-in completed. Summary: %s", memContent),
		"event", "system", []string{"check-in"}, nil, "",
	); err != nil {
		log.Printf("check-in error saving memory: %v", err)
	}

	// Post to Discord webhook
	if err := postWebhook(s.webhookURL, reply); err != nil {
		log.Printf("check-in error posting webhook: %v", err)
	}

	log.Println("check-in completed")
}

func postWebhook(url, content string) error {
	payload := map[string]string{"content": content}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("posting webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
