package kbus

import (
	"fmt"
	logger "log"
	"time"

	"github.com/qcasey/gokbus"
	"github.com/qcasey/gokbus/pkg/prepackets"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/logfile"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
)

var (
	// Enabled if the module has been set up
	Enabled    = false
	kbusDevice *gokbus.KBUS
	kbusLog    *logger.Logger
)

// Start will set up the serial port and ReadSerial goroutine
func Start(srv *server.Server) {
	// Check if enabled
	if !core.Settings.Store.GetBool("kbus.enabled") {
		log.Info().Msg("Started kbus without enabling in the config. Skipping module...")
		return
	}

	devicePath := core.Settings.Store.GetString("kbus.device")
	if devicePath == "" {
		log.Info().Msg("Started kbus without adding device in the config. Skipping module...")
		return
	}

	var err error
	kbusLog = logfile.NewLogFile("/var/log/mdroid/kbus/")

	//
	// KBus Routes
	//
	srv.Router.HandleFunc("/kbus/{src}/{dest}/{data}/{checksum}", HandleWrite).Methods("POST")
	srv.Router.HandleFunc("/kbus/{src}/{dest}/{data}", HandleWrite).Methods("POST")
	srv.Router.HandleFunc("/kbus/{command}/{checksum}", HandleWrite).Methods("GET")
	srv.Router.HandleFunc("/kbus/{command}", HandleWrite).Methods("GET")

	//
	// Catch-Alls for (hopefully) a pre-approved kbus function
	// i.e. /doors/lock
	//
	srv.Router.HandleFunc("/{device}/{command}", parseCommand()).Methods("GET")

	// Setup devices and read channels
	kbusDevice, err = gokbus.New(devicePath, 9600)
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to set up KBus with device %s", devicePath)
	}
	Enabled = true

	// Start the read and write channels
	go kbusDevice.Start()

	go func() {
		for {
			select {
			case newPacket := <-kbusDevice.ReadChannel:
				go interpret(&newPacket)
				if kbusLog != nil {
					kbusLog.Println(newPacket.Flatten())
				}
			case newErr := <-kbusDevice.ErrorChannel:
				log.Error().Err(newErr).Msg("Failed to read from kbus device")
			}
		}
	}()

	log.Info().Msgf("Successfully added device %s", devicePath)

	// Begin continuous writes
	go repeatCommand("RequestIgnitionStatus", 10*time.Second)
	go repeatCommand("RequestLampStatus", 20*time.Second)
	go repeatCommand("RequestVehicleStatus", 30*time.Second)
	go repeatCommand("RequestDoorStatus", 55*time.Second)
	go repeatCommand("RequestOdometer", 45*time.Second)
	go repeatCommand("RequestTimeStatus", 60*time.Second)
	go repeatCommand("RequestTemperatureStatus", 120*time.Second)

	go WritePackets([]gokbus.Packet{prepackets.RequestIgnitionStatus})
	go WritePackets([]gokbus.Packet{prepackets.RequestVehicleStatus})
	go WritePackets([]gokbus.Packet{prepackets.RequestDoorStatus})
	go WritePackets([]gokbus.Packet{prepackets.TurnOnClownNose})

	go func() {
		// Setup channels for meta window/door status
		windowStatus := make(chan core.Message, 1)
		doorStatus := make(chan core.Message, 1)

		core.Session.Subscribe("window_open_driver_front", windowStatus)
		core.Session.Subscribe("window_open_left_rear", windowStatus)
		core.Session.Subscribe("window_open_passenger_front", windowStatus)
		core.Session.Subscribe("window_open_right_rear", windowStatus)

		core.Session.Subscribe("door_open_driver_front", doorStatus)
		core.Session.Subscribe("door_open_left_rear", doorStatus)
		core.Session.Subscribe("door_open_passenger_front", doorStatus)
		core.Session.Subscribe("door_open_right_rear", doorStatus)
		for {
			select {
			case <-windowStatus:
				windowsOpen := core.Session.Store.GetBool("window_open_driver_front") ||
					core.Session.Store.GetBool("window_open_left_rear") ||
					core.Session.Store.GetBool("window_open_passenger_front") ||
					core.Session.Store.GetBool("window_open_right_rear")
				core.Session.Publish("windows_open", windowsOpen)
			case <-doorStatus:
				doorsOpen := core.Session.Store.GetBool("door_open_driver_front") ||
					core.Session.Store.GetBool("door_open_left_rear") ||
					core.Session.Store.GetBool("door_open_passenger_front") ||
					core.Session.Store.GetBool("door_open_right_rear")
				core.Session.Publish("doors_open", doorsOpen)
			}
		}
	}()
}

// IsPositiveRequest helps translate UP or LOCK into true or false
func isPositiveRequest(request string) (bool, error) {
	switch request {
	case "on", "up", "lock", "open", "toggle", "push":
		return true, nil
	case "off", "down", "unlock", "close":
		return false, nil
	}

	// Command didn't match any of the above, get out of here
	return false, fmt.Errorf("Error: %s is an invalid command", request)
}

// repeatCommand endlessly, helps with request functions
func repeatCommand(command string, sleepTime time.Duration) {
	log.Info().Msgf("Running KBUS command %s every %f seconds", command, sleepTime.Seconds())
	ticker := time.NewTicker(sleepTime)
	for {
		// Only push repeated KBUS commands when powered, otherwise the car won't sleep
		<-ticker.C
		if core.Session.Store.GetBool("unlock_power") {
			WriteCommand(command)
		}
	}
}
