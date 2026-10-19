// internal/http/handlers/helpers.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func parseChildID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeBadRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
}

func writeServerError(w http.ResponseWriter, err error) {
	// You can log here using your logger
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	_ = err
}

func parseHomeID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}