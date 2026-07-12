package httpapi

import "net/http"

func (s *server) handleFAQ(w http.ResponseWriter, r *http.Request) {
	if s.faq == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "faq service unavailable"})
		return
	}
	entries, err := s.faq.ListPublished(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
