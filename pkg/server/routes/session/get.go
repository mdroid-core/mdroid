package session

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/qcasey/mdroid/pkg/core"
)

// GetAll responds to an HTTP request for the entire session
func GetAll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//requestingMin := r.URL.Query().Get("min") == "1"
		response := core.JSONResponse{OK: true}
		response.Output = core.Session.Store.AllSettings()
		response.Write(&w, r)
	}
}

// Get returns a specific session value
func Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		params := mux.Vars(r)

		// Return meta values
		if params["name"] == "meta" {
			core.WriteNewResponse(&w, r, core.JSONResponse{Output: core.Session.Stats.AllSettings(), OK: true})
			return
		}

		sessionValue := core.Session.Store.Get(params["name"])
		response := core.JSONResponse{Output: sessionValue, OK: true}
		if !core.Session.Store.IsSet(params["name"]) {
			response.Output = "Does not exist"
			response.OK = false
		}
		response.Write(&w, r)
	}
}
