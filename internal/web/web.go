// Package web is the embedded, server-rendered web UI for Jot. It reads and
// writes the same SQLite database used by the agent, scheduler, and watch
// runner — no separate service, no Node, no front-end build step. Templates and
// static assets (htmx + Sortable.js, vendored) are embedded via embed.FS.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/chris/jot/internal/db"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// gridDays is how many days back (inclusive of today) the habit grid shows.
const gridDays = 30

// Server holds the dependencies for the web UI. It owns no state beyond the DB
// handle and the parsed templates.
type Server struct {
	db    *db.DB
	pages map[string]*template.Template
}

// pageFiles lists each content page that is composed with the shared layout.
// Each page defines "content"; parsing layout + exactly one page per set keeps
// the definitions from colliding (all sharing one set would let the last-parsed
// "content" win for every route).
var pageFiles = []string{"board.html", "habits.html"}

// New builds a Server, parsing the embedded layout + each page once at startup.
func New(database *db.DB) (*Server, error) {
	pages := make(map[string]*template.Template, len(pageFiles))
	for _, p := range pageFiles {
		t, err := template.ParseFS(templateFS, "templates/layout.html", "templates/"+p)
		if err != nil {
			return nil, fmt.Errorf("parsing web page %s: %w", p, err)
		}
		pages[p] = t
	}
	return &Server{db: database, pages: pages}, nil
}

// Handler returns the HTTP routes for the web UI.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	mux.HandleFunc("GET /", s.handleBoard)
	mux.HandleFunc("GET /habits", s.handleHabits)
	mux.HandleFunc("POST /things/move", s.handleMove)

	return mux
}

// --- view models ---

type boardColumn struct {
	Status string
	Label  string
	Things []db.Thing
}

type boardData struct {
	Title   string
	Columns []boardColumn
}

// boardColumns maps statuses to their on-board labels, in display order.
// 'dropped' is intentionally absent: archived items are off the board.
var boardColumns = []struct{ status, label string }{
	{"open", "Ideas"},
	{"active", "Active"},
	{"done", "Done"},
}

func (s *Server) handleBoard(w http.ResponseWriter, r *http.Request) {
	data := boardData{Title: "Board"}
	for _, c := range boardColumns {
		things, err := s.db.ThingsByStatus(c.status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data.Columns = append(data.Columns, boardColumn{Status: c.status, Label: c.label, Things: things})
	}
	s.render(w, "board.html", data)
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	status := r.FormValue("status")
	if !validStatus(status) {
		http.Error(w, "bad status", http.StatusBadRequest)
		return
	}
	afterID, _ := strconv.ParseInt(r.FormValue("after_id"), 10, 64)
	beforeID, _ := strconv.ParseInt(r.FormValue("before_id"), 10, 64)

	if err := s.db.MoveThing(id, status, afterID, beforeID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- habit grid ---

type gridDate struct {
	Date  string
	Label string
}

type gridCell struct {
	Date string
	Done bool
}

type gridRow struct {
	Name  string
	Cells []gridCell
}

type gridData struct {
	Title string
	Dates []gridDate
	Rows  []gridRow
}

func (s *Server) handleHabits(w http.ResponseWriter, r *http.Request) {
	end := time.Now()
	start := end.AddDate(0, 0, -(gridDays - 1))

	var dates []gridDate
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, gridDate{Date: d.Format("2006-01-02"), Label: d.Format("1/2")})
	}

	rows, err := s.db.HabitGrid(start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := gridData{Title: "Habits", Dates: dates}
	for _, hr := range rows {
		row := gridRow{Name: hr.Habit.Name}
		for _, d := range dates {
			row.Cells = append(row.Cells, gridCell{Date: d.Date, Done: hr.Done[d.Date]})
		}
		data.Rows = append(data.Rows, row)
	}
	s.render(w, "habits.html", data)
}

// --- helpers ---

func (s *Server) render(w http.ResponseWriter, page string, data any) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "unknown page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// layout.html defines "layout" and pulls in the page's "content".
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func validStatus(s string) bool {
	switch s {
	case "open", "active", "done", "dropped":
		return true
	}
	return false
}
