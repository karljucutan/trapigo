package correlation

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// ExtractOrGenerateRequestID extracts the X-Request-ID header from the request,
// or generates a new UUID if not provided.
func ExtractOrGenerateRequestID(r *http.Request) string {
	requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
	if requestID != "" {
		return requestID
	}
	return "urn:uuid:" + uuid.New().String()
}

// SetRequestIDHeader sets the X-Request-ID header in the response if not already present.
func SetRequestIDHeader(w http.ResponseWriter, requestID string) {
	if w.Header().Get(requestIDHeader) == "" {
		w.Header().Set(requestIDHeader, requestID)
	}
}
