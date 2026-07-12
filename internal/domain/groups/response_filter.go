package groups

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
)

type responseCapture struct {
	header     http.Header
	statusCode int
	body       bytes.Buffer
}

func newResponseCapture() *responseCapture {
	return &responseCapture{header: http.Header{}}
}

func (c *responseCapture) Header() http.Header { return c.header }

func (c *responseCapture) WriteHeader(statusCode int) {
	if c.statusCode == 0 {
		c.statusCode = statusCode
	}
}

func (c *responseCapture) Write(raw []byte) (int, error) {
	if c.statusCode == 0 {
		c.statusCode = http.StatusOK
	}
	return c.body.Write(raw)
}

func (c *responseCapture) status() int {
	if c.statusCode == 0 {
		return http.StatusOK
	}
	return c.statusCode
}

func filterModelListResponse(w http.ResponseWriter, r *http.Request, next http.Handler, snapshot *SnapshotStore, userID, providerName string) {
	capture := newResponseCapture()
	next.ServeHTTP(capture, r)

	statusCode := capture.status()
	body := capture.body.Bytes()
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		writeCapturedResponse(w, capture.header, statusCode, body, false)
		return
	}

	filtered, changed := filteredModelListBody(body, snapshot, userID, providerName)
	writeCapturedResponse(w, capture.header, statusCode, filtered, changed)
}

func writeCapturedResponse(w http.ResponseWriter, header http.Header, statusCode int, body []byte, changed bool) {
	for key, values := range header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if changed {
		w.Header().Del("Content-Length")
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func filteredModelListBody(raw []byte, snapshot *SnapshotStore, userID, providerName string) ([]byte, bool) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw, false
	}

	changed := false
	for _, key := range []string{"data", "models"} {
		models, ok := payload[key]
		if !ok {
			continue
		}
		filtered, nextChanged, recognized := filterModelArray(models, snapshot, userID, providerName)
		if !recognized {
			return raw, false
		}
		if nextChanged {
			payload[key] = filtered
			changed = true
		}
	}
	if !changed {
		return raw, false
	}

	filtered, err := json.Marshal(payload)
	if err != nil {
		return raw, false
	}
	return filtered, true
}

func filterModelArray(raw json.RawMessage, snapshot *SnapshotStore, userID, providerName string) (json.RawMessage, bool, bool) {
	var models []json.RawMessage
	if err := json.Unmarshal(raw, &models); err != nil {
		return raw, false, false
	}

	filtered := make([]json.RawMessage, 0, len(models))
	for _, model := range models {
		identifier := modelListIdentifier(model)
		if identifier == "" || !snapshot.Allows(userID, providerName, identifier) {
			continue
		}
		filtered = append(filtered, model)
	}
	if len(filtered) == len(models) {
		return raw, false, true
	}

	next, err := json.Marshal(filtered)
	if err != nil {
		return raw, false, false
	}
	return next, true, true
}

func modelListIdentifier(raw json.RawMessage) string {
	var payload struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	for _, value := range []string{payload.ID, payload.Name, payload.DisplayName} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
