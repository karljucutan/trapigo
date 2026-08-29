package problemdetails

import (
	"encoding/json"
	"net/http"
)

// ProblemDetails follows RFC 7807 problem details standard.
// See: https://tools.ietf.org/html/rfc7807
type ProblemDetails struct {
	Type       string         `json:"type"`               // URI identifying the problem type
	Status     int            `json:"status"`             // HTTP status code
	Title      string         `json:"title"`              // Human-readable summary
	Detail     string         `json:"detail,omitempty"`   // Explanation specific to this instance
	Instance   string         `json:"instance,omitempty"` // URI identifying this instance (e.g., correlation ID)
	Extensions map[string]any `json:"-"`                  // Custom extensions added via AddExtension()
}

// Builder provides a fluent API for constructing ProblemDetails.
type Builder struct {
	pd *ProblemDetails
}

// New creates a new ProblemDetails builder.
func New() *Builder {
	return &Builder{
		pd: &ProblemDetails{
			Extensions: make(map[string]any),
		},
	}
}

// WithType sets the problem type URI.
func (b *Builder) WithType(typeURI string) *Builder {
	b.pd.Type = typeURI
	return b
}

// WithStatus sets the HTTP status code.
func (b *Builder) WithStatus(status int) *Builder {
	b.pd.Status = status
	return b
}

// WithTitle sets the human-readable title.
func (b *Builder) WithTitle(title string) *Builder {
	b.pd.Title = title
	return b
}

// WithDetail sets the instance-specific detail message.
func (b *Builder) WithDetail(detail string) *Builder {
	b.pd.Detail = detail
	return b
}

// WithInstance sets the instance URI (typically includes correlation ID).
func (b *Builder) WithInstance(instance string) *Builder {
	b.pd.Instance = instance
	return b
}

// AddExtension adds a custom field to the problem details.
func (b *Builder) AddExtension(key string, value any) *Builder {
	b.pd.Extensions[key] = value
	return b
}

// Build returns the constructed ProblemDetails and marshals extensions into top-level.
func (b *Builder) Build() *ProblemDetails {
	return b.pd
}

// MarshalJSON customizes JSON encoding to include extensions at top level.
func (pd *ProblemDetails) MarshalJSON() ([]byte, error) {
	// Build a map with standard fields
	m := map[string]any{
		"type":   pd.Type,
		"status": pd.Status,
		"title":  pd.Title,
	}

	// Add optional fields if not empty
	if pd.Detail != "" {
		m["detail"] = pd.Detail
	}
	if pd.Instance != "" {
		m["instance"] = pd.Instance
	}

	// Add extensions at top level
	for k, v := range pd.Extensions {
		m[k] = v
	}

	return json.Marshal(m)
}

// NewFromError creates a ProblemDetails from an error and status code.
// This is a convenience function; use Builder for more control.
func NewFromError(status int, title string, err error, instance string) *ProblemDetails {
	detail := ""
	if err != nil {
		detail = err.Error()
	}

	return New().
		WithStatus(status).
		WithTitle(title).
		WithDetail(detail).
		WithInstance(instance).
		Build()
}

// NewNotFound creates a 404 Not Found problem details.
func NewNotFound(title, detail, instance string) *ProblemDetails {
	return New().
		WithStatus(http.StatusNotFound).
		WithTitle(title).
		WithDetail(detail).
		WithInstance(instance).
		WithType("about:blank"). // or custom type URI
		Build()
}

// NewBadRequest creates a 400 Bad Request problem details.
func NewBadRequest(title, detail, instance string) *ProblemDetails {
	return New().
		WithStatus(http.StatusBadRequest).
		WithTitle(title).
		WithDetail(detail).
		WithInstance(instance).
		WithType("about:blank").
		Build()
}

// NewInternalServerError creates a 500 Internal Server Error problem details.
func NewInternalServerError(instance string) *ProblemDetails {
	return New().
		WithStatus(http.StatusInternalServerError).
		WithTitle("Internal Server Error").
		WithDetail("An unexpected error occurred").
		WithInstance(instance).
		WithType("about:blank").
		Build()
}
