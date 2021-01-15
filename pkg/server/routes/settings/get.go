package settings

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

// GetAll returns all current settings
func GetAll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug().Msg("Responding to GET request with entire settings map.")
		returnMap := make(map[string]string)
		resp := core.JSONResponse{Output: core.Settings.Store.Unmarshal(returnMap), Status: "success", OK: true}
		resp.Write(&w, r)
	}
}

// Get returns all the values of a specific setting
func Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		componentName := core.FormatName(params["key"])

		log.Debug().Msgf("Responding to GET request for setting component %s", componentName)

		resp := core.JSONResponse{Output: core.Settings.Store.Get(params["key"]), OK: true}
		if !core.Settings.Store.IsSet(params["key"]) {
			resp = core.JSONResponse{Output: "Setting not found.", OK: false}
		}

		resp.Write(&w, r)
	}
}
