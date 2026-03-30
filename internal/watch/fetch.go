package watch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// FetchResult holds the extracted text (or error) for a single URL.
type FetchResult struct {
	URL  string
	Text string
	Err  error
}

// Fetch retrieves each URL and strips the HTML down to readable text.
// It processes URLs sequentially to be polite to small servers.
// Returns one FetchResult per URL; callers should check each .Err individually.
func Fetch(ctx context.Context, urls []string) []FetchResult {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	results := make([]FetchResult, len(urls))
	for i, u := range urls {
		results[i] = fetchOne(ctx, client, u)
	}
	return results
}

func fetchOne(ctx context.Context, client *http.Client, url string) FetchResult {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return FetchResult{URL: url, Err: fmt.Errorf("building request: %w", err)}
	}
	// Present as a normal browser — some sites block default Go UA.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:137.0) Gecko/20100101 Firefox/137.0")

	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{URL: url, Err: fmt.Errorf("fetching: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FetchResult{URL: url, Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}

	// Cap how much we read — 2MB is plenty for any theatre page.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return FetchResult{URL: url, Err: fmt.Errorf("reading body: %w", err)}
	}

	text := extractText(string(body))
	return FetchResult{URL: url, Text: text}
}

// extractText strips HTML tags and returns readable text content.
// It skips script, style, and other non-visible elements.
func extractText(rawHTML string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(rawHTML))

	var b strings.Builder
	skip := 0 // depth counter for elements we're skipping

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}

		switch tt {
		case html.StartTagToken:
			tn, _ := tokenizer.TagName()
			tag := string(tn)
			if isSkippedTag(tag) {
				skip++
			}
			// Insert whitespace for block-level elements so text doesn't run together.
			if skip == 0 && isBlockTag(tag) {
				b.WriteByte('\n')
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			tag := string(tn)
			if isSkippedTag(tag) && skip > 0 {
				skip--
			}

		case html.TextToken:
			if skip == 0 {
				text := strings.TrimSpace(tokenizer.Token().Data)
				if text != "" {
					b.WriteString(text)
					b.WriteByte(' ')
				}
			}
		}
	}

	return collapseWhitespace(b.String())
}

// isSkippedTag returns true for elements whose text content should not be extracted.
func isSkippedTag(tag string) bool {
	switch tag {
	case "script", "style", "noscript", "head", "svg", "template":
		return true
	}
	return false
}

// isBlockTag returns true for elements that should produce a line break in output.
func isBlockTag(tag string) bool {
	switch tag {
	case "p", "div", "br", "h1", "h2", "h3", "h4", "h5", "h6",
		"li", "tr", "blockquote", "section", "article", "header", "footer",
		"hr", "dt", "dd":
		return true
	}
	return false
}

// collapseWhitespace reduces runs of whitespace/blank lines to single newlines
// and trims trailing spaces on each line.
func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	prevBlank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !prevBlank {
				out = append(out, "")
			}
			prevBlank = true
		} else {
			out = append(out, line)
			prevBlank = false
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
