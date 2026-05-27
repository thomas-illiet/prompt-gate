package admin

import (
	"errors"
	"net/http"

	"promptgate/backend/internal/domain/mcp"
)

// HandleAdminListMCPServers lists configured MCP servers.
func (h *Handler) HandleAdminListMCPServers(w http.ResponseWriter, r *http.Request) {
	query := parseListQuery(r, "name", "asc")
	servers, err := h.mcp.ListServersPaged(r.Context(), mcp.ListParams{
		Page:     query.Page,
		PageSize: query.PageSize,
		SortBy:   query.SortBy,
		SortDir:  query.SortDir,
	})
	if err != nil {
		if errors.Is(err, mcp.ErrInvalidSort) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sort"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, servers)
}

// HandleAdminGetMCPServer returns one MCP server by id.
func (h *Handler) HandleAdminGetMCPServer(w http.ResponseWriter, r *http.Request) {
	server, err := h.mcp.GetServer(r.Context(), r.PathValue("id"))
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, server)
}

// HandleAdminCreateMCPServer creates an MCP server configuration.
func (h *Handler) HandleAdminCreateMCPServer(w http.ResponseWriter, r *http.Request) {
	var input mcp.CreateServerInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	server, err := h.mcp.CreateServer(r.Context(), input)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, server)
}

// HandleAdminUpdateMCPServer patches an MCP server configuration.
func (h *Handler) HandleAdminUpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	var input mcp.UpdateServerInput
	if !decodeRequestBody(w, r, &input) {
		return
	}
	server, err := h.mcp.UpdateServer(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, server)
}

// HandleAdminDeleteMCPServer deletes an MCP server configuration.
func (h *Handler) HandleAdminDeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	if err := h.mcp.DeleteServer(r.Context(), r.PathValue("id")); err != nil {
		writeMCPError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeMCPError maps MCP service errors to HTTP responses.
func writeMCPError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := err.Error()
	switch {
	case errors.Is(err, mcp.ErrServerNotFound):
		status = http.StatusNotFound
		code = "mcp_server_not_found"
	case errors.Is(err, mcp.ErrNameConflict):
		status = http.StatusConflict
		code = "name_conflict"
	case errors.Is(err, mcp.ErrInvalidName):
		status = http.StatusBadRequest
		code = "invalid_name"
	case errors.Is(err, mcp.ErrInvalidURL):
		status = http.StatusBadRequest
		code = "invalid_url"
	case errors.Is(err, mcp.ErrInvalidHeader):
		status = http.StatusBadRequest
		code = "invalid_header"
	case errors.Is(err, mcp.ErrInvalidRegex):
		status = http.StatusBadRequest
		code = "invalid_regex"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
