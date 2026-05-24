// Package helper provides standalone string utilities used across the
// compiler. By design, this package depends only on the Go standard library
// — no other compiler module — so it can be imported from anywhere without
// risk of cycles.
package helper

import (
	"net/http"
	"strings"
	"unicode"
)

// ToPascalCase converts a snake_case identifier to PascalCase.
// Each segment separated by underscores is capitalized and joined.
//
//	"article"         → "Article"
//	"research_report" → "ResearchReport"
//	""                → ""
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

// WebhookHandler registers one HTTP route. Handler receives the live request (body,
// query including run_id) and the response stream channel used by the CLI listener.
type WebhookHandler struct {
	Endpoint string
	Handler  func(w http.ResponseWriter, r *http.Request, stream chan any)
}

// HttpStreamingHandler is a helper function to create a HTTP handler that streams data to the client.
//
// Example:
//
//	http.HandleFunc("/stream/numbers", HttpStreamingHandler(func(w http.ResponseWriter, f http.Flusher) {
//		for i := 1; i <= 5; i++ {
//			fmt.Fprintf(w, "Number: %d\n", i)
//			f.Flush()
//			time.Sleep(1 * time.Second)
//		}
//	}))
func HttpStreamingHandler(streamLogic func(w http.ResponseWriter, f http.Flusher)) http.HandlerFunc {

	// It returns a standard HTTP handler
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Set the necessary headers once for any endpoint that uses this wrapper
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Execute the custom data logic
		streamLogic(w, flusher)
	}
}
