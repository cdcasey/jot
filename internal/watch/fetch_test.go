package watch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchBasicHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html>
		<head><title>Auditions</title><style>body{color:red}</style></head>
		<body>
			<h1>Upcoming Auditions</h1>
			<p>Hamlet - <b>Austin Playhouse</b> - March 15</p>
			<p>Macbeth - City Theatre - April 1</p>
			<script>alert('hi')</script>
		</body>
		</html>`))
	}))
	defer srv.Close()

	results := Fetch([]string{srv.URL})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if !strings.Contains(r.Text, "Upcoming Auditions") {
		t.Errorf("expected heading text, got: %s", r.Text)
	}
	if !strings.Contains(r.Text, "Austin Playhouse") {
		t.Errorf("expected body text, got: %s", r.Text)
	}
	// Script and style content should be stripped
	if strings.Contains(r.Text, "alert") {
		t.Error("script content should be stripped")
	}
	if strings.Contains(r.Text, "color:red") {
		t.Error("style content should be stripped")
	}
}

func TestFetchStripsScriptAndStyle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<div>
			<script>var x = "secret";</script>
			<style>.hidden { display: none; }</style>
			<noscript>Enable JavaScript</noscript>
			<p>Visible content</p>
		</div>`))
	}))
	defer srv.Close()

	results := Fetch([]string{srv.URL})
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	if strings.Contains(r.Text, "secret") {
		t.Error("script text should be stripped")
	}
	if strings.Contains(r.Text, "display") {
		t.Error("style text should be stripped")
	}
	if strings.Contains(r.Text, "Enable JavaScript") {
		t.Error("noscript text should be stripped")
	}
	if !strings.Contains(r.Text, "Visible content") {
		t.Errorf("expected visible text, got: %s", r.Text)
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	results := Fetch([]string{srv.URL})
	r := results[0]
	if r.Err == nil {
		t.Error("expected error for 404 response")
	}
	if !strings.Contains(r.Err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", r.Err)
	}
}

func TestFetchMultipleURLs(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<p>Page One</p>`))
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<p>Page Two</p>`))
	}))
	defer srv2.Close()

	results := Fetch([]string{srv1.URL, srv2.URL})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !strings.Contains(results[0].Text, "Page One") {
		t.Errorf("expected Page One, got: %s", results[0].Text)
	}
	if !strings.Contains(results[1].Text, "Page Two") {
		t.Errorf("expected Page Two, got: %s", results[1].Text)
	}
}

func TestFetchBadURL(t *testing.T) {
	results := Fetch([]string{"http://localhost:1/nonexistent"})
	r := results[0]
	if r.Err == nil {
		t.Error("expected error for unreachable URL")
	}
}

func TestFetchPreservesBlockStructure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<h1>Title</h1><p>First paragraph.</p><p>Second paragraph.</p><ul><li>Item A</li><li>Item B</li></ul>`))
	}))
	defer srv.Close()

	results := Fetch([]string{srv.URL})
	r := results[0]
	if r.Err != nil {
		t.Fatalf("unexpected error: %v", r.Err)
	}
	// Block elements should produce line breaks, not run together.
	if strings.Contains(r.Text, "Title First") {
		t.Error("expected line break between h1 and p")
	}
	if strings.Contains(r.Text, "paragraph. Second") {
		t.Error("expected line break between paragraphs")
	}
}

func TestExtractTextEmpty(t *testing.T) {
	text := extractText("")
	if text != "" {
		t.Errorf("expected empty string, got: %q", text)
	}
}

func TestExtractTextPlainText(t *testing.T) {
	// If given plain text (no HTML), it should pass through.
	text := extractText("just some plain text")
	if !strings.Contains(text, "just some plain text") {
		t.Errorf("expected plain text passthrough, got: %q", text)
	}
}
