package core

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

// JSONResponse for common return value to API
type JSONResponse struct {
	Output interface{} `json:"output,omitempty"`
	Status string      `json:"status,omitempty"`
	OK     bool        `json:"ok"`
	Method string      `json:"method,omitempty"`
	ID     int         `json:"id,omitempty"`
}

// Write to an http writer, adding extra info and HTTP status as needed
func (response *JSONResponse) Write(w *http.ResponseWriter, r *http.Request) {
	// Deref writer
	writer := *w

	writer.Header().Set("Content-Type", "application/json")

	// Add string Status if it doesn't exist, add appropriate headers
	if response.OK {
		if response.Status == "" {
			response.Status = "success"
		}
		writer.WriteHeader(http.StatusOK)
	} else {
		if response.Status == "" {
			response.Status = "fail"
			writer.WriteHeader(http.StatusBadRequest)
		} else if response.Status == "error" {
			writer.WriteHeader(http.StatusNoContent)
		} else {
			writer.WriteHeader(http.StatusBadRequest)
		}
	}

	// Log this to debug
	log.Debug().
		Str("Path", r.URL.Path).
		Str("Method", r.Method).
		Str("Output", fmt.Sprintf("%v", response.Output)).
		Str("Status", response.Status).
		Bool("OK", response.OK).
		Msg("Full Response:")

	// Write out this response
	json.NewEncoder(writer).Encode(response.Output)
}

// WriteNewResponse exports all known stat requests
func WriteNewResponse(w *http.ResponseWriter, r *http.Request, response JSONResponse) {
	// Echo back message
	response.Write(w, r)
}
