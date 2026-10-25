package groups

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"
)

var errInvalidModelListResponse = errors.New("invalid model list response")

type responseCapture struct {
	header     http.Header
	statusCode int
	body       bytes.Buffer
	maxBytes   int64
	overflow   bool
}

func newResponseCapture(maxBytes int64) *responseCapture {
	return &responseCapture{header: http.Header{}, maxBytes: maxBytes}
}

func (c *responseCapture) Header() http.Header { return c.header }

func (c *responseCapture) WriteHeader(statusCode int) {
	// Informational responses do not complete an HTTP exchange. ReverseProxy can
	// forward 100/102/103 before the final status; remembering one as final would
	// make a subsequent 200 catalog take the non-success bypass path below.
	if statusCode >= 100 && statusCode < 200 && statusCode != http.StatusSwitchingProtocols {
		return
	}
	if c.statusCode == 0 {
		c.statusCode = statusCode
	}
}

func (c *responseCapture) Write(raw []byte) (int, error) {
	if c.statusCode == 0 {
		c.statusCode = http.StatusOK
	}
	if c.overflow || len(raw) == 0 {
		return len(raw), nil
	}
	remaining := c.maxBytes - int64(c.body.Len())
	if c.maxBytes <= 0 || int64(len(raw)) > remaining {
		c.overflow = true
		c.body = bytes.Buffer{}
		return len(raw), nil
	}
	_, _ = c.body.Write(raw)
	return len(raw), nil
}

func (c *responseCapture) status() int {
	if c.statusCode == 0 {
		return http.StatusOK
	}
	return c.statusCode
}

func filterModelListResponse(w http.ResponseWriter, r *http.Request, next http.Handler, snapshot *SnapshotStore, userID, providerName string, maxBytes int64) {
	capture := newResponseCapture(maxBytes)
	upstreamRequest := r.Clone(r.Context())
	upstreamRequest.Header = r.Header.Clone()
	// Model catalogs must be inspected before they are returned. Requesting an
	// identity representation prevents compressed bytes (and decompression
	// bombs) from bypassing the JSON filter.
	upstreamRequest.Header.Set("Accept-Encoding", "identity")
	next.ServeHTTP(capture, upstreamRequest)

	if capture.overflow {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "model_response_too_large"})
		return
	}

	statusCode := capture.status()
	body := capture.body.Bytes()
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		writeCapturedResponse(w, capture.header, statusCode, body, false)
		return
	}
	if hasNonIdentityContentEncoding(capture.header.Values("Content-Encoding")) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid_model_response"})
		return
	}

	filtered, changed, err := filteredModelListBody(body, snapshot, userID, providerName)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid_model_response"})
		return
	}
	writeCapturedResponse(w, capture.header, statusCode, filtered, changed)
}

func writeCapturedResponse(w http.ResponseWriter, header http.Header, statusCode int, body []byte, changed bool) {
	for key, values := range header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if changed {
		for _, name := range []string{
			"Content-Length",
			"Content-Md5",
			"Content-Digest",
			"Digest",
			"Etag",
			"Last-Modified",
			"Repr-Digest",
			"Content-Encoding",
		} {
			w.Header().Del(name)
		}
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

func filteredModelListBody(raw []byte, snapshot *SnapshotStore, userID, providerName string) ([]byte, bool, error) {
	if err := validateUniqueJSONKeys(raw); err != nil {
		return nil, false, errInvalidModelListResponse
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false, errInvalidModelListResponse
	}

	changed := false
	recognized := false
	for _, key := range []string{"data", "models"} {
		models, ok := payload[key]
		if !ok {
			continue
		}
		recognized = true
		filtered, nextChanged, err := filterModelArray(models, snapshot, userID, providerName)
		if err != nil {
			return nil, false, err
		}
		if nextChanged {
			payload[key] = filtered
			changed = true
		}
		if sanitizeModelListPagination(payload, filtered) {
			changed = true
		}
	}
	if !recognized {
		return nil, false, errInvalidModelListResponse
	}
	if !changed {
		return raw, false, nil
	}

	filtered, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	return filtered, true, nil
}

func validateUniqueJSONKeys(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := validateUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errInvalidModelListResponse
		}
		return err
	}
	return nil
}

func validateUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errInvalidModelListResponse
			}
			foldedKey := simpleFoldJSONKey(key)
			if _, exists := seen[foldedKey]; exists {
				return errInvalidModelListResponse
			}
			seen[foldedKey] = struct{}{}
			if err := validateUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errInvalidModelListResponse
		}
	case '[':
		for decoder.More() {
			if err := validateUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errInvalidModelListResponse
		}
	default:
		return errInvalidModelListResponse
	}
	return nil
}

func simpleFoldJSONKey(value string) string {
	var folded strings.Builder
	folded.Grow(len(value))
	for _, current := range value {
		minimum := current
		for candidate := unicode.SimpleFold(current); candidate != current; candidate = unicode.SimpleFold(candidate) {
			if candidate < minimum {
				minimum = candidate
			}
		}
		folded.WriteRune(minimum)
	}
	return folded.String()
}

func sanitizeModelListPagination(payload map[string]json.RawMessage, filtered json.RawMessage) bool {
	var models []json.RawMessage
	if err := json.Unmarshal(filtered, &models); err != nil {
		return false
	}
	changed := false
	identifiers := make([]string, 0, len(models))
	for _, model := range models {
		if identifier := modelListIdentifier(model); identifier != "" {
			identifiers = append(identifiers, identifier)
		}
	}

	_, hadFirstID := payload["first_id"]
	_, hadLastID := payload["last_id"]
	if len(identifiers) == 0 {
		if hadFirstID {
			delete(payload, "first_id")
			changed = true
		}
		if hadLastID {
			delete(payload, "last_id")
			changed = true
		}
	} else {
		if hadFirstID {
			next, _ := json.Marshal(identifiers[0])
			if !bytes.Equal(payload["first_id"], next) {
				payload["first_id"] = next
				changed = true
			}
		}
		if hadLastID {
			next, _ := json.Marshal(identifiers[len(identifiers)-1])
			if !bytes.Equal(payload["last_id"], next) {
				payload["last_id"] = next
				changed = true
			}
		}
	}
	if current, ok := payload["has_more"]; ok && !bytes.Equal(bytes.TrimSpace(current), []byte("false")) {
		payload["has_more"] = json.RawMessage("false")
		changed = true
	}
	// Opaque cursors can encode identifiers for entries removed above. Returning
	// no continuation is conservative: it can truncate discovery, but cannot
	// disclose or navigate into a model outside the caller's grants.
	for _, key := range []string{"next", "next_page", "next_cursor"} {
		if _, ok := payload[key]; ok {
			delete(payload, key)
			changed = true
		}
	}
	return changed
}

func filterModelArray(raw json.RawMessage, snapshot *SnapshotStore, userID, providerName string) (json.RawMessage, bool, error) {
	var models []json.RawMessage
	if err := json.Unmarshal(raw, &models); err != nil {
		return nil, false, errInvalidModelListResponse
	}
	if models == nil {
		return nil, false, errInvalidModelListResponse
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
		return raw, false, nil
	}

	next, err := json.Marshal(filtered)
	if err != nil {
		return nil, false, err
	}
	return next, true, nil
}

func hasNonIdentityContentEncoding(values []string) bool {
	for _, value := range values {
		for encoding := range strings.SplitSeq(value, ",") {
			encoding = strings.TrimSpace(encoding)
			if encoding != "" && !strings.EqualFold(encoding, "identity") {
				return true
			}
		}
	}
	return false
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
