package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/chris/jot/internal/agent"
	"github.com/chris/jot/internal/db"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron       *cron.Cron
	webhookURL string
	db         *db.DB
	agent      *agent.Agent
	dmSend     func(userID, content string) error
	mu         sync.Mutex
	entryIDs   map[int64]cron.EntryID // scheduleID -> cron entry
}

func New(database *db.DB, ag *agent.Agent, webhookURL string, dmSend func(userID, content string) error) *Scheduler {
	return &Scheduler{
		cron:       cron.New(),
		webhookURL: webhookURL,
		db:         database,
		agent:      ag,
		dmSend:     dmSend,
		entryIDs:   make(map[int64]cron.EntryID),
	}
}

func (s *Scheduler) Start() {
	s.loadSchedules()
	s.cron.Start()

	// Reload schedules every 5 minutes to pick up agent-created changes
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			s.loadSchedules()
		}
	}()

	// Poll for due reminders every 60 seconds
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for range t.C {
			s.fireReminders()
		}
	}()

	log.Println("scheduler started")
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// SeedDefaultSchedule inserts a morning check-in if the schedules table is empty.
func (s *Scheduler) SeedDefaultSchedule(cronExpr string) {
	schedules, err := s.db.ListSchedules(false)
	if err != nil {
		log.Printf("scheduler: checking schedules: %v", err)
		return
	}
	if len(schedules) == 0 && cronExpr != "" {
		_, err := s.db.CreateSchedule(
			"morning-checkin",
			cronExpr,
			"Perform a morning check-in. Summarize pending work, mention overdue items, suggest priorities for the day.",
		)
		if err != nil {
			log.Printf("scheduler: seeding default schedule: %v", err)
		} else {
			log.Printf("scheduler: seeded default schedule with cron %q", cronExpr)
		}
	}
}

func (s *Scheduler) loadSchedules() {
	schedules, err := s.db.ListSchedules(true)
	if err != nil {
		log.Printf("scheduler: loading schedules: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove all existing entries and re-register
	// Fine for this scale. Actual diffing would be way more complex.
	for _, entryID := range s.entryIDs {
		s.cron.Remove(entryID)
	}
	s.entryIDs = make(map[int64]cron.EntryID)

	for _, sched := range schedules {
		sched := sched // capture for closure
		entryID, err := s.cron.AddFunc(sched.CronExpr, func() {
			s.runSchedule(sched)
		})
		if err != nil {
			log.Printf("scheduler: invalid cron %q for schedule %q: %v", sched.CronExpr, sched.Name, err)
			continue
		}
		s.entryIDs[sched.ID] = entryID
	}

	log.Printf("scheduler: loaded %d schedule(s)", len(s.entryIDs))
}

func (s *Scheduler) runSchedule(sched db.Schedule) {
	prompt, err := agent.BuildCheckInPrompt(s.db)
	if err != nil {
		log.Printf("scheduler[%s]: building prompt: %v", sched.Name, err)
		return
	}
	fullPrompt := prompt + "\n\n" + sched.Prompt

	reply, _, err := s.agent.Run(context.Background(), nil, fullPrompt)
	if err != nil {
		log.Printf("scheduler[%s]: agent error: %v", sched.Name, err)
		return
	}

	if err := s.db.RecordScheduleRun(sched.ID); err != nil {
		log.Printf("scheduler[%s]: recording run: %v", sched.Name, err)
	}

	if _, err := s.db.CreateCheckIn(reply); err != nil {
		log.Printf("scheduler[%s]: storing check-in: %v", sched.Name, err)
	}

	s.deliver(fmt.Sprintf("scheduler[%s]", sched.Name), reply)

	log.Printf("scheduler[%s]: completed", sched.Name)
}

func (s *Scheduler) fireReminders() {
	pending, err := s.db.ListPendingReminders()
	if err != nil {
		log.Printf("scheduler: listing reminders: %v", err)
		return
	}
	for _, r := range pending {
		r := r
		msg := fmt.Sprintf("A reminder you set: %s", r.Prompt)
		reply, _, err := s.agent.Run(context.Background(), nil, msg)
		if err != nil {
			log.Printf("scheduler: reminder %d agent error: %v", r.ID, err)
			continue
		}
		if err := s.db.MarkReminderFired(r.ID); err != nil {
			log.Printf("scheduler: marking reminder %d fired: %v", r.ID, err)
		}
		s.deliver(fmt.Sprintf("reminder[%d]", r.ID), reply)
		log.Printf("scheduler: fired reminder %d", r.ID)
	}
}

func (s *Scheduler) deliver(label, content string) {
	// Try DM first
	if s.dmSend != nil {
		note, err := s.db.GetNote("discord_user_id")
		if err == nil && note != "" {
			if err := s.dmSend(note, content); err != nil {
				log.Printf("%s: DM send failed: %v", label, err)
			} else {
				return
			}
		}
	}
	// Fall back to webhook
	if s.webhookURL != "" {
		if err := postWebhook(s.webhookURL, content); err != nil {
			log.Printf("%s: webhook failed: %v", label, err)
		}
		return
	}
	log.Printf("%s: no delivery method available (no DM user and no webhook)", label)
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
