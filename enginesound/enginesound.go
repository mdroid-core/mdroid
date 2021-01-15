// Package enginesound is a simple socket interface designed to send RPMs to the rust engine sound generator
// https://github.com/DasEtwas/enginesound
package enginesound

import (
	"net"
	"os"

	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/qcasey/mdroid/systemd"
	"github.com/rs/zerolog/log"
)

var (
	socketAddress string
	essToggledOn  bool
	essRunning    bool
	keyDetected   bool
)

// Start enginesound rpm socket
func Start(srv *server.Server) {

	// Check if enabled
	if !core.Settings.Store.GetBool("enginesound.enabled") {
		log.Info().Msg("Started enginesound without enabling in the config. Skipping module...")
		return
	}

	socketAddress = core.Settings.Store.GetString("enginesound.socket")
	essToggledOn = core.Settings.Store.GetBool("enginesound.toggledOn")

	// Setup channels, subscribe to RPMs and toggle setting
	go func() {
		enginesoundHook := make(chan core.Message, 1)
		rpmHook := make(chan core.Message, 1)
		var ess net.Conn

		core.Session.Subscribe("rpm", rpmHook)
		core.Session.Subscribe("key_detected", enginesoundHook)
		core.Settings.Subscribe("enginesound.toggledOn", enginesoundHook)

		for {
			select {
			case message := <-enginesoundHook:
				messageValue := message.Value.(bool)

				// Toggle enginesound on/off
				if message.Topic == "enginesound.toggledon" {
					essToggledOn = messageValue
				} else if message.Topic == "key_detected" {
					keyDetected = messageValue
				}
				if messageValue {
					turnOn()
				} else {
					if ess != nil {
						err := ess.Close()
						if err != nil {
							log.Error().Err(err).Msg("Failed to close enginesound socket")
						}
					}
					turnOff()
				}

			case message := <-rpmHook:
				// Only run if enginesound is toggled on
				if essRunning && ess != nil {
					// Write RPMs to socket
					_, err := ess.Write([]byte(message.Value.(string)))
					if err != nil {
						log.Error().Err(err).Msgf("Failed to write RPM %s to socket", message.Value.(string))
					}
				}
			}
		}
	}()
}

// turnOn enginesound, starting the service and socket
func turnOn() net.Conn {
	// If already running, or not enabled, or key not detected, do not change state
	if essRunning ||
		!essToggledOn ||
		!keyDetected {
		return nil
	}

	var (
		err      error
		tempSock net.Conn
	)

	// Remove old socket
	if err := os.RemoveAll(socketAddress); err != nil {
		log.Error().Err(err).Msg("Failed to remove old enginesound socket")
		return nil
	}
	// Start enginesound service
	if err := systemd.RestartService("enginesound.service"); err != nil {
		log.Error().Err(err).Msg("Failed to start enginesound service")
		return nil
	}
	// Begin listening on new socket
	if tempSock, err = net.Dial("unix", socketAddress); err != nil {
		log.Error().Err(err).Msg("Failed to create new enginesound socket")
		return nil
	}
	essRunning = true
	return tempSock
}

// turnOff enginesound, closing the service
func turnOff() {
	if !essRunning {
		return
	}
	// Stop enginesound service
	if err := systemd.StopService("enginesound.service"); err != nil {
		log.Error().Err(err).Msg("Failed to stop enginesound service")
	}
	essRunning = false
}
