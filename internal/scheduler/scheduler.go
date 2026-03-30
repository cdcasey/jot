package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chris/jot/internal/agent"
	"github.com/chris/jot/internal/db"
	"github.com/chris/jot/internal/watch"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron          *cron.Cron
	webhookURL    string
	db            *db.DB
	agent         *agent.Agent
	watchRunner   *watch.Runner
	dmSend        func(userID, content string) error
	mu            sync.Mutex
	entryIDs      map[int64]cron.EntryID // scheduleID -> cron entry
	watchEntryIDs map[int64]cron.EntryID // watchID -> cron entry
}

func New(database *db.DB, ag *agent.Agent, webhookURL string, dmSend func(userID, content string) error, wr *watch.Runner) *Scheduler {
	return &Scheduler{
		cron:          cron.New(),
		webhookURL:    webhookURL,
		db:            database,
		agent:         ag,
		watchRunner:   wr,
		dmSend:        dmSend,
		entryIDs:      make(map[int64]cron.EntryID),
		watchEntryIDs: make(map[int64]cron.EntryID),
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
		// Skip one-shot schedules — they're handled by the polling ticker, not cron.
		if sched.FireAt != "" {
			continue
		}
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

	// Load watches into cron (only if runner is configured).
	s.loadWatches()
}

func (s *Scheduler) runSchedule(sched db.Schedule) {
	var reply string
	var err error

	if userID := s.resolveUserID(); userID != "" {
		reply, err = s.agent.RunWithConversation(context.Background(), userID, sched.Prompt)
	} else {
		reply, _, err = s.agent.Run(context.Background(), nil, sched.Prompt)
	}

	if err != nil {
		log.Printf("scheduler[%s]: agent error: %v", sched.Name, err)
		return
	}

	if err := s.db.RecordScheduleRun(sched.ID); err != nil {
		log.Printf("scheduler[%s]: recording run: %v", sched.Name, err)
	}

	s.deliver(fmt.Sprintf("scheduler[%s]", sched.Name), reply)

	log.Printf("scheduler[%s]: completed", sched.Name)
}

func (s *Scheduler) fireReminders() {
	pending, err := s.db.ListPendingOneShots()
	if err != nil {
		log.Printf("scheduler: listing one-shots: %v", err)
		return
	}
	for _, r := range pending {
		msg := fmt.Sprintf("A reminder just fired. The user asked to be reminded: %q. Deliver this reminder to them in a brief, friendly message. Do NOT create a new reminder or ask clarifying questions — just notify them.", r.Prompt)
		var reply string
		var err error
		if userID := s.resolveUserID(); userID != "" {
			reply, err = s.agent.RunWithConversation(context.Background(), userID, msg)
		} else {
			reply, _, err = s.agent.Run(context.Background(), nil, msg)
		}
		if err != nil {
			log.Printf("scheduler: one-shot %d agent error: %v", r.ID, err)
			continue
		}
		if err := s.db.MarkOneShotFired(r.ID); err != nil {
			log.Printf("scheduler: marking one-shot %d fired: %v", r.ID, err)
		}
		s.deliver(fmt.Sprintf("reminder[%d]", r.ID), reply)
		log.Printf("scheduler: fired one-shot %d", r.ID)
	}
}

// loadWatches registers enabled watches with cron expressions into the cron scheduler.
// Must be called with s.mu held.
func (s *Scheduler) loadWatches() {
	if s.watchRunner == nil {
		return
	}

	for _, entryID := range s.watchEntryIDs {
		s.cron.Remove(entryID)
	}
	s.watchEntryIDs = make(map[int64]cron.EntryID)

	watches, err := s.db.ListWatches(true)
	if err != nil {
		log.Printf("scheduler: loading watches: %v", err)
		return
	}

	for _, w := range watches {
		if w.CronExpr == "" {
			continue // manual-only watch
		}
		entryID, err := s.cron.AddFunc(w.CronExpr, func() {
			s.runWatch(w)
		})
		if err != nil {
			log.Printf("scheduler: invalid cron %q for watch %q: %v", w.CronExpr, w.Name, err)
			continue
		}
		s.watchEntryIDs[w.ID] = entryID
	}

	if len(s.watchEntryIDs) > 0 {
		log.Printf("scheduler: loaded %d watch(es)", len(s.watchEntryIDs))
	}
}

func (s *Scheduler) runWatch(w db.Watch) {
	newResults, err := s.watchRunner.RunWatch(context.Background(), w)
	if err != nil {
		log.Printf("watch[%s]: error: %v", w.Name, err)
		return
	}

	if len(newResults) == 0 {
		log.Printf("watch[%s]: no new items", w.Name)
		return
	}

	msg := formatWatchResults(w.Name, newResults)
	s.deliver(fmt.Sprintf("watch[%s]", w.Name), msg)

	// Mark delivered results as notified.
	ids := make([]int64, len(newResults))
	for i, r := range newResults {
		ids[i] = r.ID
	}
	if err := s.db.MarkResultsNotified(ids); err != nil {
		log.Printf("watch[%s]: marking notified: %v", w.Name, err)
	}

	log.Printf("watch[%s]: delivered %d new item(s)", w.Name, len(newResults))
}

func formatWatchResults(watchName string, results []db.WatchResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**Watch: %s** — %d new item(s):\n\n", watchName, len(results))
	for _, r := range results {
		fmt.Fprintf(&b, "• **%s**\n", r.Title)
		if r.Body != "" {
			fmt.Fprintf(&b, "  %s\n", r.Body)
		}
		if r.SourceURL != "" {
			fmt.Fprintf(&b, "  %s\n", r.SourceURL)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
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

// resolveUserID looks up the discord_user_id note. Returns empty string if not set.
func (s *Scheduler) resolveUserID() string {
	note, err := s.db.GetNote("discord_user_id")
	if err != nil || note == "" {
		return ""
	}
	return note
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
