package kbus

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/qcasey/mdroid/pkg/core"
)

// HandleWrite handles incoming requests to the kbus program, will add routines to the queue
func HandleWrite(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	src, srcOK := params["src"]
	dest, destOK := params["dest"]
	data, dataOK := params["data"]

	if srcOK && destOK && dataOK && len(src) == 2 && len(dest) == 2 && len(data) > 0 {
		WriteData(src, dest, data)
	} else if params["command"] != "" {
		WriteCommand(params["command"])
	} else {
		core.WriteNewResponse(&w, r, core.JSONResponse{Output: "Invalid command", OK: false})
		return
	}

	core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
}
