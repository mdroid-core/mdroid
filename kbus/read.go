package kbus

import (
	"encoding/binary"
	"fmt"

	"github.com/qcasey/gokbus"
	"github.com/qcasey/gokbus/pkg/prepackets"
	"github.com/qcasey/gokbus/pkg/translations"
	"github.com/qcasey/mdroid/bluetooth"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

func interpret(p *gokbus.Packet) error {
	log.Debug().Msg(p.Pretty())

	packetMeaning, err := translations.GetMeaning(p)
	if err != nil {
		return err
	}

	publishMeaning(p, packetMeaning)
	return nil
}

func publishMeaning(p *gokbus.Packet, m translations.PacketMessageMeaning) {
	flatData := fmt.Sprintf("%02X", p.Data)

	switch m {

	case translations.IgnitionOff:
		core.Session.Publish("IGNITION", false)

	case translations.KeyIn:
		core.Session.Publish("KEY_DETECTED", true)

	case translations.KeyOut:
		core.Session.Publish("KEY_DETECTED", false)
		bluetooth.Disconnect()

	case translations.KeyNotDetected:
		core.Session.Publish("KEY_DETECTED", false)

	case translations.KeyDetected:
		core.Session.Publish("KEY_DETECTED", true)

	case translations.TopClosed:
		core.Session.Publish("CONVERTIBLE_TOP_OPEN", false)

	case translations.TopOpen:
		core.Session.Publish("CONVERTIBLE_TOP_OPEN", true)

	case translations.CarUnlocked:
		core.Session.Publish("DOORS_LOCKED", false)
		core.Session.Publish("DOOR_LOCKED_PASSENGER", false)
		core.Session.Publish("DOOR_LOCKED_DRIVER", false)

	case translations.CarLocked:
		core.Session.Publish("DOORS_LOCKED", true)
		core.Session.Publish("DOOR_LOCKED_PASSENGER", true)
		core.Session.Publish("DOOR_LOCKED_DRIVER", true)

	case translations.PassengerDoorLocked:
		core.Session.Publish("DOOR_LOCKED_PASSENGER", true)

	case translations.DriverDoorLocked:
		core.Session.Publish("DOOR_LOCKED_DRIVER", true)

	case translations.AuxHeatingOff:
		core.Session.Publish("CLIMATE.AUX_HEATING", false)

	case translations.SeatMemory1:
		core.Session.Publish("SEAT_MEMORY_1", true)

	case translations.SeatMemory2:
		core.Session.Publish("SEAT_MEMORY_2", true)

	case translations.SeatMemory3:
		core.Session.Publish("SEAT_MEMORY_3", true)

	case translations.SeatMemoryAny:
		core.Session.Publish("SEAT_MEMORY_PUSHED", true)

	case translations.SteeringWheelNextPressed:
		bluetooth.Next()

	case translations.SteeringWheelPreviousPressed:
		bluetooth.Prev()

	case translations.SteeringWheelRTPressed:
		WritePackets(prepackets.PressMode)
		WritePackets(prepackets.PressNum6)

	case translations.SteeringWheelSpeakPressed:
		WritePackets(prepackets.PressMode)

	case translations.VehicleStatus:
		if p.Data[0] == 0x54 && len(p.Data) > 14 {
			// VIN number is in plaintext, first two model letters are ASCII
			core.Session.Publish("VIN", fmt.Sprintf("%x%x%02X", p.Data[1], p.Data[2], []byte{p.Data[3], p.Data[4], p.Data[5]}))

			// Odometer, rounded to the nearest hundred in KM
			core.Session.Publish("ODOMETER_ESTIMATE", 100*binary.LittleEndian.Uint32([]byte{p.Data[6], p.Data[7]}))

			// Liters since last service, first byte and first 4 bits in second byte
			// I.E. '58 02' would be 88+0 or 880 liters
			core.Session.Publish("LITERS_SINCE_LAST_SERVICE", fmt.Sprintf("%d", int(p.Data[9])+int(p.Data[10])))

			// Days since last service
			core.Session.Publish("DAYS_SINCE_LAST_SERVICE", binary.LittleEndian.Uint32([]byte{p.Data[12], p.Data[13]}))
		}

	case translations.WindowDoorMessage:
		// Door status
		core.Session.Publish("DOORS_LOCKED", p.Data[1]&32 == 32)
		core.Session.Publish("DOOR_OPEN_LEFT_REAR", p.Data[1]&8 == 8)
		core.Session.Publish("DOOR_OPEN_RIGHT_REAR", p.Data[1]&4 == 4)
		core.Session.Publish("DOOR_OPEN_PASSENGER_FRONT", p.Data[1]&2 == 2)
		core.Session.Publish("DOOR_OPEN_DRIVER_FRONT", p.Data[1]&1 == 1)

		// Window status
		core.Session.Publish("WINDOW_OPEN_LEFT_REAR", p.Data[2]&8 == 8)
		core.Session.Publish("WINDOW_OPEN_RIGHT_REAR", p.Data[2]&4 == 4)
		core.Session.Publish("WINDOW_OPEN_PASSENGER_FRONT", p.Data[2]&2 == 2)
		core.Session.Publish("WINDOW_OPEN_DRIVER_FRONT", p.Data[2]&1 == 1)

		// Lid status
		core.Session.Publish("SUNROOF_OPEN", p.Data[2]&16 == 16)
		core.Session.Publish("TRUNK_OPEN", p.Data[2]&32 == 32)
		core.Session.Publish("HOOD_OPEN", p.Data[2]&64 == 64)

		// Light status
		core.Session.Publish("INTERIOR_LIGHT_ON", p.Data[1]&64 == 64)

	case translations.RainLightSensorStatus:
		if p.Data[0] == 0x59 {
			switch p.Data[2] {
			case 0x01:
				core.Session.Publish("LIGHT_SENSOR_REASON", "TWILIGHT")
			case 0x02:
				core.Session.Publish("LIGHT_SENSOR_REASON", "DARKNESS")
			case 0x04:
				core.Session.Publish("LIGHT_SENSOR_REASON", "RAIN")
			case 0x08:
				core.Session.Publish("LIGHT_SENSOR_REASON", "TUNNEL")
			case 0x10:
				core.Session.Publish("LIGHT_SENSOR_REASON", "BASEMENT_GARAGE")
			}

			core.Session.Publish("LIGHT_SENSOR_ON", p.Data[1]&128 == 128)
			core.Session.Publish("LIGHT_SENSOR_INTENSITY", p.Data[1])
		}
		core.Session.Publish("RAIN_LIGHT_SENSOR_STATUS", flatData)

	case translations.SensorStatus:
		core.Session.Publish("HANDBRAKE", p.Data[1]&1 == 1)
		core.Session.Publish("WARNINGS.OIL_PRESSURE", p.Data[1]&2 == 2)
		core.Session.Publish("WARNINGS.BRAKE_PADS", p.Data[1]&4 == 4)
		core.Session.Publish("WARNINGS.TRANSMISSION", p.Data[1]&8 == 8)

		core.Session.Publish("IGNITION", p.Data[2]&1 == 1)
		core.Session.Publish("WARNINGS.DOOR_OPEN", p.Data[2]&2 == 2)

		//core.Session.Publish("CLIMATE.AUX_VENT", p.Data[3]&8 == 8)

		switch p.Data[2] & 0xF0 {
		case 0x00:
			core.Session.Publish("GEAR", "NONE")
		case 0xB0:
			core.Session.Publish("GEAR", "PARK")
		case 0x10:
			core.Session.Publish("GEAR", "REVERSE")
		case 0x70:
			core.Session.Publish("GEAR", "NEUTRAL")
		case 0x80:
			core.Session.Publish("GEAR", "DRIVE")
		case 0x20:
			core.Session.Publish("GEAR", "FIRST")
		case 0x60:
			core.Session.Publish("GEAR", "SECOND")
		case 0xD0:
			core.Session.Publish("GEAR", "THIRD")
		case 0xC0:
			core.Session.Publish("GEAR", "FOURTH")
		case 0xE0:
			core.Session.Publish("GEAR", "FIFTH")
		case 0xF0:
			core.Session.Publish("GEAR", "SIXTH")
		}

	case translations.TemperatureStatus:
		core.Session.Publish("AMBIENT_TEMPERATURE_C", int(p.Data[1]))
		core.Session.Publish("COOLANT_TEMPERATURE_C", int(p.Data[2]))

	case translations.ClimateControl:
		core.Session.Publish("CLIMATE.AIR_CONDITIONING_ON", flatData == "838008")
		core.Session.Publish("CLIMATE.CONTROL_STATUS", flatData)

	case translations.Diagnostic:
		core.Session.Publish("DIAGNOSTIC", flatData)

	case translations.IgnitionStatus: // Ignition Status
		switch p.Data[1] {
		case 0x00: // Key out
			core.Session.Publish("KEY_DETECTED", false)
		case 0x01: // Key on ACC 1
			core.Session.Publish("KEY_POSITION", 1)
		case 0x03: // Key on ACC 2
			core.Session.Publish("KEY_POSITION", 2)
		case 0x07: // Key on Ignition Start
			core.Session.Publish("KEY_POSITION", 3)
		}

	case translations.OdometerStatus: // Odometer reading, in response to request
		core.Session.Publish("ODOMETER", binary.BigEndian.Uint64([]byte{p.Data[1], p.Data[2], p.Data[3]}))

	case translations.SpeedRPMStatus: // Speed / RPM Info, broadcasted every 2 seconds
		core.Session.Publish("KBUS_SPEED", int(p.Data[1])*2)
		core.Session.Publish("KBUS_RPM", int(p.Data[2])*100)

	case translations.RangeStatus:
		if p.Data[1] == 0x06 {
			core.Session.Publish("RANGE_KM", binary.BigEndian.Uint32([]byte{p.Data[3], p.Data[4], p.Data[5], p.Data[6]}))
		} else if p.Data[1] == 0x0A {
			core.Session.Publish("AVG_SPEED", binary.BigEndian.Uint32([]byte{p.Data[3], p.Data[4], p.Data[5], p.Data[6]}))
		}

	case translations.IkeStatus:
		core.Session.Publish("IKE_STATUS", flatData)
	}
}
