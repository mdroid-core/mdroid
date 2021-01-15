package main

import (
	"fmt"
	"math"
	"time"

	"github.com/qcasey/mdroid/bluetooth"
	"github.com/qcasey/mdroid/mserial"
	"github.com/qcasey/mdroid/network"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/core/component"
	"github.com/qcasey/mdroid/syscamera"
	"github.com/rs/zerolog/log"
)

func customHooks() {
	go network.PollNetworkState("can0", "network.can0")
	go network.PollNetworkState("usb0", "lte")
	go network.PollDefaultRoute()

	////
	// Angel Eyes
	////
	angelEyes := component.NewWithDefaults(
		"angel_eyes",
		func() (bool, string) {
			hasAuxPower := core.Session.Store.GetBool("acc_power")
			lightSensor := core.Session.Store.GetString("light_sensor_on") == "FALSE"
			reason := fmt.Sprintf("lightSensor: %t, hasPower: %t", lightSensor, hasAuxPower)
			return lightSensor && hasAuxPower, reason
		},
	)
	core.Settings.Subscribe("components.angel_eyes", angelEyes.Hook)
	core.Session.Subscribe("light_sensor_reason", angelEyes.Hook)
	core.Session.Subscribe("acc_power", angelEyes.Hook)
	core.Session.Subscribe("angel_eyes", angelEyes.Hook)

	////
	// Cameras
	////
	cameras := component.New(
		"cameras",
		func() (bool, string) {
			hasAuxPower := core.Session.Store.GetBool("unlock_power")
			usbHubPowered := core.Session.Store.GetBool("usb_hub")
			reason := fmt.Sprintf("unlock_power: %v, usb_hub: %v", hasAuxPower, usbHubPowered)
			return hasAuxPower && usbHubPowered, reason
		},
		syscamera.StartCameras,
		syscamera.StopCameras,
	)
	core.Settings.Subscribe("components.usb_hub", cameras.Hook)
	core.Settings.Subscribe("components.cameras", cameras.Hook)

	////
	// usb_hub
	////
	usbHub := component.New(
		"usb_hub",
		func() (bool, string) {
			hasAuxPower := core.Session.Store.GetBool("unlock_power")
			reason := fmt.Sprintf("unlock_power: %v", hasAuxPower)
			return hasAuxPower, reason
		},
		func() {
			mserial.Await("powerOn:usb_hub")
		},
		func() {
			syscamera.StopCameras()
			mserial.Await("powerOff:usb_hub")
		},
	)
	core.Settings.Subscribe("components.usb_hub", usbHub.Hook)
	core.Session.Subscribe("unlock_power", usbHub.Hook)

	////
	// LTE
	////
	lte := component.New(
		"LTE",
		func() (bool, string) {
			eth0 := core.Session.Store.GetBool("network.eth0")
			wlan0 := core.Session.Store.GetBool("network.wlan0")
			wlan1 := core.Session.Store.GetBool("network.wlan1")
			bnep0 := core.Session.Store.GetBool("network.bnep0")

			shouldBeOn := !wlan0 && !wlan1 && !bnep0
			reason := fmt.Sprintf("bnep0: %t, wlan0: %t, wlan1: %t, eth0: %t", bnep0, wlan0, wlan1, eth0)
			return shouldBeOn, reason
		},
		func() {
			err := network.ConnectLTE()
			if err != nil {
				log.Error().Msg(err.Error())
				return
			}
			log.Info().Msgf("Successfully connected LTE")
		},
		func() {
			err := network.DisconnectLTE()
			if err != nil {
				log.Error().Msg(err.Error())
				return
			}
			log.Info().Msgf("Successfully disconnected LTE")
		},
	)
	core.Settings.Subscribe("components.lte", lte.Hook)
	core.Session.Subscribe("lte", lte.Hook)
	core.Session.Subscribe("network.bnep0", lte.Hook)
	core.Session.Subscribe("network.wlan0", lte.Hook)
	core.Session.Subscribe("network.wlan1", lte.Hook)
	core.Session.Subscribe("network.eth0", lte.Hook)

	///
	// Misc. Hooks
	///
	bluetoothConnectedHook := make(chan core.Message, 1)
	core.Session.Subscribe("bluetooth.connected", bluetoothConnectedHook)

	accPowerHook := make(chan core.Message, 1)
	core.Session.Subscribe("acc_power", accPowerHook)

	keyPowerHook := make(chan core.Message, 1)
	core.Session.Subscribe("key_power", keyPowerHook)

	mainVoltageHook := make(chan core.Message, 1)
	core.Session.Subscribe("main_voltage_raw", mainVoltageHook)

	auxVoltageHook := make(chan core.Message, 1)
	core.Session.Subscribe("aux_voltage_raw", auxVoltageHook)

	for {
		select {
		case bluetoothConnected := <-bluetoothConnectedHook:
			go evalBluetoothDeviceState()
			go evalBluetoothNetworkState(bluetoothConnected.Value.(bool))
		case mainVoltageRaw := <-mainVoltageHook:
			go mainVoltage(mainVoltageRaw)
		case auxVoltageRaw := <-auxVoltageHook:
			go auxVoltage(auxVoltageRaw)
		case <-accPowerHook:
			go evalBluetoothDeviceState()
		case <-keyPowerHook:
			go evalBluetoothDeviceState()
		case <-lte.Hook:
			go lte.Evaluate()
		case <-usbHub.Hook:
			go usbHub.Evaluate()
		case <-cameras.Hook:
			go cameras.Evaluate()
		case <-angelEyes.Hook:
			go angelEyes.Evaluate()
		}
	}
}

func mainVoltage(m core.Message) {
	core.Session.Publish("main_voltage", math.Round(m.Value.(float64)/1024.0*33.3*100)/100)
}

func auxVoltage(m core.Message) {
	auxVoltageRounded := math.Round(m.Value.(float64)/1024.0*33.3*100) / 100
	core.Session.Publish("aux_voltage", auxVoltageRounded)

	// Calculate battery percentage
	if auxVoltageRounded < 11.2 {
		core.Session.Publish("battery_percent", 0)
		return
	}
	core.Session.Publish("battery_percent", math.Round((auxVoltageRounded-11.2)/1.3*100))
}

func evalBluetoothNetworkState(bluetoothConnected bool) {
	deviceName := core.Session.Store.GetString("bluetooth.name")
	log.Info().Msgf("Device name: '%s'", deviceName)

	if bluetoothConnected {
		if deviceName != "zuljin" {
			if deviceName != "" {
				log.Warn().Msgf("Wrong device %s, not leeching LTE", deviceName)
			}
			return
		}

		time.Sleep(time.Millisecond * 1000)
		if bterr := bluetooth.ConnectNetwork(); bterr != nil {
			log.Error().Msg(bterr.Error())
			return
		}

		// Wait for device to come online
		time.Sleep(time.Millisecond * 1000)
	}
}

func evalBluetoothDeviceState() {
	// Play / pause bluetooth media on key in/out
	if core.Session.Store.GetBool("acc_power") {
		bluetooth.Play()
	} else {
		bluetooth.Pause()
	}
}
