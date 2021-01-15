package mserial

import (
	logger "log"
	"time"

	"github.com/google/uuid"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/logfile"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
)

var (
	// Enabled if the module has been set up
	Enabled   = false
	devices   []*Device
	serialLog *logger.Logger
)

// Start will set up the serial port and ReadSerial goroutine
func Start(srv *server.Server) {
	// Check if enabled
	if !core.Settings.Store.GetBool("mserial.enabled") {
		log.Info().Msg("Started mSerial without enabling in the config. Skipping module...")
		return
	}

	// Setup routes
	srv.Router.HandleFunc("/serial/{command}", writeSerial()).Methods("POST", "GET")

	err := core.Settings.Store.UnmarshalKey("mserial.connections", &devices)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode Serial devices")
		return
	}

	// Setup devices and read channels
	for _, device := range devices {
		device.readerMessages = make(chan readerMessage, 1)
		device.writeQueue = make(map[uuid.UUID]*writeQueueItem, 5)

		go device.begin()

		go func(d *Device) {
			for {
				msg := <-d.readerMessages
				core.Session.Publish(msg.key, msg.value)
			}
		}(device)
	}

	// Open log file
	serialLog = logfile.NewLogFile("/var/log/mdroid/serial/")

	Enabled = true
}

// Await queues a message for writing, and waits for it to be sent
func Await(msg string) {
	if len(devices) == 0 {
		log.Error().Msgf("No serial devices configured to handle message: %s", msg)
		return
	}
	log.Info().Msgf("Writing %s to %d devices", msg, len(devices))
	for _, d := range devices {
		sleeps := 0
		for d.port == nil {
			sleeps++
			time.Sleep(time.Millisecond * 100)
			if sleeps%10 == 0 {
				log.Warn().Msgf("Slept %d times on message %s, waiting on port %s", sleeps, msg, d.Name)
			}
		}

		err := d.write(msg)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to write to device %s", d.Name)
		}
		log.Info().Msgf("Successfully wrote %s", msg)
	}
}
