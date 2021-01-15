package settings

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

// Set is the http wrapper for our setting setter
func Set() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		// Parse out params
		key := params["key"]
		value := params["value"]

		// Log if requested
		log.Debug().Msgf("Responding to POST request for setting %s to be value %s", key, value)

		// Do the dirty work elsewhere
		core.Settings.Publish(key, value)

		// Respond with OK
		response := core.JSONResponse{Output: key, OK: true}
		response.Write(&w, r)
	}
}
