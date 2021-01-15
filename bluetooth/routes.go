package bluetooth

import (
	"fmt"
	"net/http"

	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

// handleConnect wrapper for connect
func handleConnect(w http.ResponseWriter, r *http.Request) {
	go Connect(core.Settings.Store.GetString("bluetooth.address"))
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
}

// handleConnectNetwork wrapper for connect
func handleConnectNetwork(w http.ResponseWriter, r *http.Request) {
	err := ConnectNetwork()
	response := "OK"
	if err != nil {
		response = err.Error()
	}
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: response, OK: err == nil})
}

// handleDisconnectNetwork wrapper for connect
func handleDisconnectNetwork(w http.ResponseWriter, r *http.Request) {
	err := DisconnectNetwork()
	response := "OK"
	if err != nil {
		response = err.Error()
	}
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: response, OK: err == nil})
}

// handleDisconnect bluetooth device
func handleDisconnect(w http.ResponseWriter, r *http.Request) {
	err := Disconnect()
	if err != nil {
		log.Error().Err(err).Msg("Disconnect failed")
		core.WriteNewResponse(&w, r, core.JSONResponse{Output: "Disconnect failed", OK: false})
	}
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
}

// handleGetDeviceInfo attempts to get metadata about connected device
func handleGetDeviceInfo(w http.ResponseWriter, r *http.Request) {
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: IsPlaying(), Status: "success", OK: true})
}

// handleGetMediaInfo attempts to get metadata about current track
func handleGetMediaInfo(w http.ResponseWriter, r *http.Request) {
	resp, err := GetMetadata()
	if err != nil {
		log.Error().Err(err).Msg("Failed to handle media info")
		core.WriteNewResponse(&w, r, core.JSONResponse{Output: fmt.Sprintf("Error getting media info: %s", err.Error()), Status: "fail", OK: false})
		return
	}

	log.Info().Msgf("%v", resp)

	// Echo back all info
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: resp, Status: "success", OK: true})
}

// handlePrev skips to previous track
func handlePrev(w http.ResponseWriter, r *http.Request) {
	Prev()
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
}

// handleNext skips to next track
func handleNext(w http.ResponseWriter, r *http.Request) {
	Next()
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
}

// handlePlay attempts to play bluetooth media
func handlePlay(w http.ResponseWriter, r *http.Request) {
	Play()
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
}

// handlePause attempts to pause bluetooth media
func handlePause(w http.ResponseWriter, r *http.Request) {
	Pause()
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: "OK", OK: true})
}
