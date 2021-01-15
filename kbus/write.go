package kbus

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/qcasey/gokbus"
	"github.com/qcasey/gokbus/pkg/prepackets"
	"github.com/qcasey/mdroid/mserial"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

// WritePackets adds a raw packet to the KBUS write channel
func WritePackets(packets []gokbus.Packet) error {
	if kbusDevice == nil {
		return fmt.Errorf("kbus device is nil")
	}
	for _, p := range packets {
		kbusDevice.WriteChannel <- p
	}
	return nil
}

// WriteCommand adds a directive to the KBUS write channel
func WriteCommand(command string) error {
	if kbusDevice == nil {
		return fmt.Errorf("kbus device is nil")
	}

	//
	// Special cases, where one packet doesn't do the full job
	//
	switch command {
	case "RollWindowsUp":
		go WritePackets(prepackets.PopWindowsUp)
		go WritePackets(prepackets.PopWindowsUp)
		return nil
	case "RollWindowsDown":
		go WritePackets(prepackets.PopWindowsDown)
		go WritePackets(prepackets.PopWindowsDown)
		return nil
	}

	packets := prepackets.RequestToPacket(command)
	if packets == nil {
		return fmt.Errorf("Command '%s' not found in prepared list of packets", command)
	}

	return WritePackets(packets)
}

// WriteData with given src, dest, and data to the kbus
func WriteData(src string, dest string, data string) error {
	if len(src) != 2 {
		return fmt.Errorf("%s incorrect length, must represent byte", src)
	}
	if len(dest) != 2 {
		return fmt.Errorf("%s incorrect length, must represent byte", dest)
	}
	if kbusDevice == nil {
		return fmt.Errorf("kbus device is nil")
	}

	newPacket := gokbus.Packet{
		Source:      []byte(src)[0],
		Destination: []byte(dest)[0],
		Data:        []byte(data),
	}

	kbusDevice.WriteChannel <- newPacket

	return nil
}

// ParseCommand is a list of pre-approved routes to KBUS for easier routing
func parseCommand() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)

		if len(params["device"]) == 0 || len(params["command"]) == 0 {
			core.WriteNewResponse(&w, r, core.JSONResponse{Output: "Error: One or more required params is empty", OK: false})
			return
		}

		// Format similarly to the rest of MDroid suite, removing plurals
		// Formatting allows for fuzzier requests
		var err error
		device := strings.TrimSuffix(core.FormatName(params["device"]), "s")
		command := strings.TrimSuffix(core.FormatName(params["command"]), "s")

		// Parse command into a bool, make either "on" or "off" effectively
		isPositive, err := isPositiveRequest(command)
		cannotBeParsedIntoBoolean := err != nil

		// Check if we care that the request isn't formatted into an "on" or "off"
		if cannotBeParsedIntoBoolean {
			switch device {
			case "door", "top", "convertible_top", "hazard", "flasher", "interior":
				log.Error().Err(err).Msgf("The given command %s could not be parsed into a bool", device)
				return
			}
		}

		log.Info().Msgf("Attempting to send command %s to device %s", command, device)

		// If the car's ACC power isn't on, it won't be ready for requests. Wake it up first
		if !core.Session.Store.GetBool("acc_power") {
			err = WritePackets([]gokbus.Packet{prepackets.RequestIgnitionStatus}) // this will be swallowed
			if err != nil {
				log.Error().Err(err).Msgf("Failed to parse command")
				return
			}
		}

		// All I wanted was a moment or two
		// To see if you could do that switch-a-roo
		switch device {
		case "door":
			doorsAreLocked := core.Session.Store.GetBool("doors_locked")
			if mserial.Enabled &&
				((isPositive && !doorsAreLocked) || (!isPositive && doorsAreLocked)) {
				mserial.Await("toggleDoorLocks")
			} else {
				log.Info().Msgf("Request to %s doors denied, door status is %t", command, doorsAreLocked)
			}
		case "window":
			if command == "popdown" {
				err = WritePackets(prepackets.PopWindowsDown)
			} else if command == "popup" {
				err = WritePackets(prepackets.PopWindowsUp)
			} else if isPositive {
				err = WriteCommand("RollWindowsUp")
			} else {
				err = WriteCommand("RollWindowsDown")
			}
		case "trunk":
			err = WritePackets([]gokbus.Packet{prepackets.OpenTrunk})
		case "hazard":
			if isPositive {
				err = WritePackets([]gokbus.Packet{prepackets.FlashHazards})
			} else {
				err = WritePackets([]gokbus.Packet{prepackets.TurnOffAllExteriorLights})
			}
		case "flasher":
			if isPositive {
				err = WritePackets([]gokbus.Packet{prepackets.FlashLowBeamsAndHazards})
			} else {
				err = WritePackets([]gokbus.Packet{prepackets.TurnOffAllExteriorLights})
			}
		case "interior":
			if isPositive {
				err = WritePackets([]gokbus.Packet{prepackets.ToggleInteriorLights})
			} else {
				err = WritePackets([]gokbus.Packet{prepackets.ToggleInteriorLights})
			}
		case "clown", "nose":
			err = WritePackets([]gokbus.Packet{prepackets.TurnOnClownNose})
		case "mode":
			err = WritePackets(prepackets.PressMode)
		case "radio", "nav", "stereo":
			switch command {
			case "am":
				err = WritePackets(prepackets.PressAM)
			case "fm":
				err = WritePackets(prepackets.PressFM)
			case "next":
				err = WritePackets(prepackets.PressNext)
			case "prev":
				err = WritePackets(prepackets.PressPrev)
			case "mode":
				err = WritePackets(prepackets.PressMode)
			case "1":
				err = WritePackets(prepackets.PressNum1)
			case "2":
				err = WritePackets(prepackets.PressNum2)
			case "3":
				err = WritePackets(prepackets.PressNum3)
			case "4":
				err = WritePackets(prepackets.PressNum4)
			case "5":
				err = WritePackets(prepackets.PressNum5)
			case "6":
				err = WritePackets(prepackets.PressNum6)
			default:
				err = WritePackets(prepackets.PressStereoPower)
			}
		default:
			log.Error().Msgf("Invalid device %s", device)
			response := core.JSONResponse{Output: fmt.Sprintf("Invalid device %s", device), OK: false}
			response.Write(&w, r)
			return
		}

		if err != nil {
			log.Error().Err(err).Msgf("Error parsing command %s", command)
			core.WriteNewResponse(&w, r, core.JSONResponse{Output: err.Error(), OK: false})
			return
		}

		// Yay
		log.Info().Msgf("Successfully wrote %s to %s.", command, device)
		core.WriteNewResponse(&w, r, core.JSONResponse{Output: device, OK: true})
	}
}
