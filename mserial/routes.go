package mserial

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/qcasey/mdroid/pkg/core"
)

// writeSerial handles messages sent through the server
func writeSerial() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		if params["command"] == "" {
			core.WriteNewResponse(&w, r, core.JSONResponse{Output: "Empty command", OK: false})
		}
		Await(params["command"])
		core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
	}
}
