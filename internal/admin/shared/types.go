// Package shared provides common types for the admin UI.
package shared

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed templates/*.html templates/*/*.html
var templateFS embed.FS

//go:embed static/css/*.css static/js/*.js
var staticFS embed.FS

// TemplateRenderer renders HTML templates.
type TemplateRenderer struct {
	funcs     template.FuncMap
	layoutTpl *template.Template
	cache     map[string]*template.Template
	mu        sync.RWMutex
}

// NewTemplateRenderer creates a new template renderer.
func NewTemplateRenderer() (*TemplateRenderer, error) {
	funcs := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"json": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"lower":     strings.ToLower,
		"upper":     strings.ToUpper,
		"contains":  strings.Contains,
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"replace":   strings.ReplaceAll,
		"split":     strings.Split,
		"join":      strings.Join,
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"mul": func(a, b int) int {
			return a * b
		},
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},
		"seq": func(start, end int) []int {
			var result []int
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, _ := values[i].(string)
				dict[key] = values[i+1]
			}
			return dict
		},
		"list": func(values ...interface{}) []interface{} {
			return values
		},
	}

	// Parse layout template
	layoutContent, err := templateFS.ReadFile("templates/layout.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read layout template: %w", err)
	}

	layoutTpl, err := template.New("layout").Funcs(funcs).Parse(string(layoutContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse layout template: %w", err)
	}

	return &TemplateRenderer{
		funcs:     funcs,
		layoutTpl: layoutTpl,
		cache:     make(map[string]*template.Template),
	}, nil
}

// RenderData holds common data for all templates.
type RenderData struct {
	Title       string
	CurrentPath string
	Flash       *FlashMessage
	Data        interface{}
}

// Render renders a template to the response writer.
func (r *TemplateRenderer) Render(w http.ResponseWriter, name string, data RenderData) error {
	tmpl, err := r.getTemplate(name)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", name, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = buf.WriteTo(w)
	return err
}

// RenderPartial renders a template partial (without layout) to the response writer.
func (r *TemplateRenderer) RenderPartial(w http.ResponseWriter, name string, data interface{}) error {
	content, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Funcs(r.funcs).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	// Execute the named define block if it exists, otherwise execute the whole template
	defName := name
	if err := tmpl.ExecuteTemplate(&buf, defName, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", name, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err = buf.WriteTo(w)
	return err
}

func (r *TemplateRenderer) getTemplate(name string) (*template.Template, error) {
	r.mu.RLock()
	tmpl, ok := r.cache[name]
	r.mu.RUnlock()
	if ok {
		return tmpl, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if tmpl, ok := r.cache[name]; ok {
		return tmpl, nil
	}

	// Clone layout template
	tmpl, err := r.layoutTpl.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone layout template: %w", err)
	}

	// Read and parse the page template
	content, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", name, err)
	}

	// Parse the page template into the cloned layout
	_, err = tmpl.Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	// Also parse any related templates in the same directory that are partials
	// (don't define "content" - only partials like row.html, table.html)
	dir := ""
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		dir = name[:idx]
	}
	if dir != "" {
		entries, _ := templateFS.ReadDir("templates/" + dir)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			partialName := dir + "/" + entry.Name()
			if partialName == name {
				continue // Skip the main template we already parsed
			}
			partialContent, err := templateFS.ReadFile("templates/" + partialName)
			if err != nil {
				continue
			}
			// Skip templates that define "content" (they are full page templates, not partials)
			if strings.Contains(string(partialContent), `{{define "content"}}`) {
				continue
			}
			_, _ = tmpl.Parse(string(partialContent))
		}
	}

	r.cache[name] = tmpl
	return tmpl, nil
}

// StaticHandler returns an http.Handler for serving static files.
func StaticHandler() http.Handler {
	subFS, _ := fs.Sub(staticFS, "static")
	return http.StripPrefix("/static/", http.FileServer(http.FS(subFS)))
}

// FlashType represents the type of flash message.
type FlashType string

const (
	FlashSuccess FlashType = "success"
	FlashError   FlashType = "error"
	FlashWarning FlashType = "warning"
	FlashInfo    FlashType = "info"
)

// FlashMessage represents a flash message to display to the user.
type FlashMessage struct {
	Type    FlashType
	Message string
}

// SetFlashHeader sets the HX-Trigger header for HTMX flash messages.
func SetFlashHeader(w http.ResponseWriter, flashType FlashType, message string) {
	w.Header().Set("HX-Trigger", `{"showFlash": {"type": "`+string(flashType)+`", "message": "`+message+`"}}`)
}

// SetSuccessFlash sets a success flash message.
func SetSuccessFlash(w http.ResponseWriter, message string) {
	SetFlashHeader(w, FlashSuccess, message)
}

// SetErrorFlash sets an error flash message.
func SetErrorFlash(w http.ResponseWriter, message string) {
	SetFlashHeader(w, FlashError, message)
}
