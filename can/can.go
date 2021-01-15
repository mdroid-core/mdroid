package can

import (
	"fmt"
	logger "log"

	"github.com/brutella/can"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/logfile"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
)

var (
	canLog *logger.Logger
)

// Start will set up the serial port and ReadSerial goroutine
func Start(srv *server.Server) {
	// Check if enabled
	if !core.Settings.Store.GetBool("can.enabled") {
		log.Info().Msg("Started can without enabling in the config. Skipping module...")
		return
	}

	if !core.Settings.Store.IsSet("can.device") {
		log.Info().Msg("Started can without adding device in the config. Skipping module...")
		return
	}

	go Connect()

	go func() {
		// Setup channels
		can0Hook := make(chan core.Message, 1)

		core.Session.Subscribe("network.can0", can0Hook)
		for {
			select {
			case message := <-can0Hook:
				// Reconnect CAN0 when it comes back online
				if message.Value == true {
					go Connect()
				}
			}
		}
	}()
}

// Connect to the CAN bus
func Connect() {
	devicePath := core.Settings.Store.GetString("can.device")
	log.Info().Msgf("Opening CAN device %s...", devicePath)
	bus, err := can.NewBusForInterfaceWithName(devicePath)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to set up can with device %s", devicePath)
		return
	}
	bus.SubscribeFunc(handleCANFrame)

	// Set up file logging
	canLog = logfile.NewLogFile("/var/log/mdroid/can/")
	log.Info().Msgf("Successfully started %s", devicePath)

	err = bus.ConnectAndPublish()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to set up can with device %s", devicePath)
		return
	}
}

func logFrameToConsole(frm can.Frame) {
	length := fmt.Sprintf("[%x]", frm.Length)
	log.Info().Msgf("%-4x %-3s % -24X", frm.ID, length, frm.Data[:])
}

// https://github.com/brutella/can/blob/master/cmd/candump.go
// trim returns a subslice of s by slicing off all trailing b bytes.
func trimSuffix(s []byte, b byte) []byte {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != b {
			return s[:i+1]
		}
	}

	return []byte{}
}

// https://github.com/brutella/can/blob/master/cmd/candump.go
// printableString creates a string from s and replaces non-printable bytes (i.e. 0-32, 127)
// with '.' – similar how candump from can-utils does it.
func printableString(s []byte) string {
	var ascii []byte
	for _, b := range s {
		if b < 32 || b > 126 {
			b = byte('.')

		}
		ascii = append(ascii, b)
	}

	return string(ascii)
}
