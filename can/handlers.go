package can

import (
	"fmt"
	"strconv"

	"github.com/brutella/can"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

func handleCANFrame(frm can.Frame) {
	//		logFrameToConsole(frm)
	if canLog != nil {
		canLog.Println(fmt.Sprintf("%-4x %-3s % -24X\n", frm.ID, fmt.Sprintf("[%x]", frm.Length), frm.Data[:]))
	}

	switch frm.ID {
	case 339:
		// ASC1

		// Parse speed
		// Speed is MSB+LSB
		// Bit length of 12, discard last 4 bits of LSB
		// Gain of 1/8, so 0x0008 is 1kmph
		speedInt, err := strconv.ParseInt(fmt.Sprintf("%0.2X%X", frm.Data[2], frm.Data[1])[0:3], 16, 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert speed to int")
		}
		speed := .125 * float64(speedInt)
		if speed <= 0.5 {
			speed = 0
		}
		core.Session.Publish("Speed", speed)

	case 790:
		// DME1
		rpmInt, err := strconv.ParseInt(fmt.Sprintf("%x%x", frm.Data[3], frm.Data[2]), 16, 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert rpm to int")
		}
		rpm := float64(rpmInt) / 6.4
		//if rpm > 300 {
		core.Session.Publish("RPM", rpm)
		//}

	case 809:
		// DME2
		core.Session.Publish("Engine_Temp_C", (0.75*float64(frm.Data[1]))-48.373)
		core.Session.Publish("Cruise_Control", frm.Data[3]&128 == 1) // bit 7

		// If both bits 6 and 5 are 1, then cruise has resumed
		if frm.Data[3]&64 == 1 && frm.Data[3]&32 == 1 {
			core.Session.Publish("Cruise_Control_Resume_Pressed", true)
		} else {
			core.Session.Publish("Cruise_Control_Down_Pressed", frm.Data[3]&64 == 1) // bit 6
			core.Session.Publish("Cruise_Control_Up_Pressed", frm.Data[3]&32 == 1)   // bit 5
		}
		core.Session.Publish("Throttle_Position", frm.Data[5]) // max value 0xFE
		core.Session.Publish("Kickdown_Switch", frm.Data[6] == 4)
		core.Session.Publish("Brake_Pedal_Pressed", frm.Data[6] == 1)

	case 824:
		// DME3
		core.Session.Publish("Sport_Mode_On", frm.Data[2] == 2)
		core.Session.Publish("Sport_Mode_Error", frm.Data[2] == 3)

	case 1349:
		// DME4

		// Byte 0
		core.Session.Publish("Check_Engine_Light", frm.Data[0]&2 == 1)   // bit 1
		core.Session.Publish("Cruise_Control_Light", frm.Data[0]&8 == 1) // bit 3
		core.Session.Publish("EML_Light", frm.Data[0]&16 == 1)           // bit 4
		core.Session.Publish("Check_Gas_Cap_Light", frm.Data[0]&64 == 1) // bit 6

		// Byte 3
		core.Session.Publish("Oil_Level_Light", frm.Data[3]&2 == 1) // bit 1
		core.Session.Publish("Overheat_Light", frm.Data[3]&8 == 1)  // bit 3
		core.Session.Publish("7000_RPM_Light", frm.Data[3]&16 == 1) // bit 4
		core.Session.Publish("6500_RPM_Light", frm.Data[3]&32 == 1) // bit 5
		core.Session.Publish("5500_RPM_Light", frm.Data[3]&64 == 1) // bit 6

		// Byte 4
		core.Session.Publish("Oil_Temp_C", float64(frm.Data[4])-48.373)

	case 1555:
		// IC
		if frm.Data[2] == 128 {
			// Empty tank
			core.Session.Publish("Fuel_Level", 0)
		} else if frm.Data[2] <= 135 && frm.Data[2] > 128 {
			core.Session.Publish("Fuel_Level", (frm.Data[2]-128)/64)
		} else {
			// between 0 and 57, add another 7 to account for rollover
			core.Session.Publish("Fuel_Level", (frm.Data[2]+7)/64)
		}

		odometerInt, err := strconv.ParseInt(fmt.Sprintf("%x%x", frm.Data[1], frm.Data[0]), 16, 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert odometer to int")
		}
		core.Session.Publish("Odometer", float64(odometerInt)*10*0.621) // Value * 10 = Odometer in KM, then convert to miles

		clockInt, err := strconv.ParseInt(fmt.Sprintf("%x%x", frm.Data[4], frm.Data[3]), 16, 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert clock to int")
		}
		core.Session.Publish("ECU_Uptime", clockInt) // Minutes since battery power was lost

	case 1557:
		// AC
		/*
			x being temperature in Deg C,
			#(x>=0 deg C,DEC2HEX(x),DEC2HEX(-x)+128) x range min -40 C max 50 C
			#if format(data[3], 'x') > 50:
			#	temp = format(data[3], 'x')+(128)
			#else:
			#	temp = format(data[3], 'x')*/
		core.Session.Publish("Exterior_Temperature_C", frm.Data[3])
		core.Session.Publish("Air_Conditioning_On", frm.Data[0]&128 == 1)

	case 504:
		core.Session.Publish("Brake_Pressure", frm.Data[2])
	}
}
