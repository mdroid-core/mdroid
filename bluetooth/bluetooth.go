// Package bluetooth is a rudimentary interface between MDroid-Core and underlying BT dbus
package bluetooth

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"
	"github.com/gosimple/slug"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/agent"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	"github.com/muka/go-bluetooth/bluez/profile/media"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
)

// Metadata describes the current bluetooth media status
type Metadata struct {
	Artist      string
	Album       string
	ArtworkPath string
	Title       string
	Genre       string
	Subtype     string
	Duration    uint32
	Position    uint32
	IsPlaying   bool
}

// Bluetooth is the modular implementation of Bluetooth controls
var (
	Profiles             []string
	connectedDevice      *device.Device1
	connectedMediaPlayer *media.MediaControl1
)

// Start bluetooth with address
func Start(srv *server.Server) {
	// Check if enabled
	if !core.Settings.Store.GetBool("bluetooth.enabled") {
		log.Info().Msg("Started bluetooth without enabling in the config. Skipping module...")
		return
	}

	if !core.Settings.Store.IsSet("bluetooth.profiles") {
		log.Info().Msg("Started bluetooth without adding profiles in the config. Skipping module...")
		return
	}

	//
	// Bluetooth routes
	//
	srv.Router.HandleFunc("/bluetooth", handleGetDeviceInfo).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/getDeviceInfo", handleGetDeviceInfo).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/getMediaInfo", handleGetMediaInfo).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/connect", handleConnect).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/disconnect", handleDisconnect).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/network/connect", handleConnectNetwork).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/network/disconnect", handleDisconnectNetwork).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/prev", handlePrev).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/next", handleNext).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/pause", handlePause).Methods("GET")
	srv.Router.HandleFunc("/bluetooth/play", handlePlay).Methods("GET")

	bluetoothAddress := core.Settings.Store.GetString("bluetooth.address")
	Profiles = core.Settings.Store.GetStringSlice("bluetooth.profiles")
	//go startAutoRefresh(srv.Core)

	// Connect bluetooth device on startup
	go func() {
		err := Connect(bluetoothAddress)
		if err != nil {
			log.Error().Err(err).Msg("Failed to connect to bluetooth")
		}
	}()
}

// Connect bluetooth device
func Connect(bluetoothAddress string) error {
	log.Info().Msgf("Connecting to bluetooth device %s", bluetoothAddress)

	if bluetoothAddress == "" {
		return fmt.Errorf("Empty bluetooth address")
	}

	a, err := api.GetDefaultAdapter()
	if err != nil {
		return err
	}

	err = a.SetPowered(true)
	if err != nil {
		return err
	}

	err = a.SetDiscoverable(true)
	if err != nil {
		return err
	}

	err = a.SetPairable(true)
	if err != nil {
		return err
	}

	go func() {
		ch, _, err := a.OnDeviceDiscovered()
		if err != nil {
			log.Error().Err(err).Msg("Failed to set adapter to watch for new devices")
		}

		for {
			<-ch
			err := refreshConnectedDevice()
			if err != nil {
				log.Error().Err(err).Msg("Failed to refresh connected devices")
			}
		}
	}()

	devices, err := a.GetDevices()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get devices")
		return fmt.Errorf("GetDevices: %s", err)
	}
	if len(devices) == 0 {
		log.Error().Msg("Empty device list")
		return nil
	}

	for _, dev := range devices {
		log.Info().Msgf("Found device path %s", dev.Path())
		if dev.Properties.Address != bluetoothAddress {
			continue
		}

		if !dev.Properties.Paired {
			log.Info().Msgf("Pairing with %s", dev.Properties.Address)

			err := dev.Pair()
			if err != nil {
				return fmt.Errorf("Pair failed: %s", err)
			}
		}

		log.Debug().Msgf("Device's allowed profiles: %v", dev.Properties.UUIDs)

		log.Info().Msgf("Connecting device %s...", dev.Properties.Address)
		agent.SetTrusted(api.GetDefaultAdapterID(), dev.Path())

		for _, profile := range Profiles {
			time.Sleep(time.Millisecond * 50)
			err = dev.ConnectProfile(profile)
			if err != nil {
				if err.Error() == "In Progress" {
					continue
				}
				log.Error().Err(err).Msgf("Connection %s failed", profile)
				return nil
			}
		}

		connectedDevice = dev

		// Wait for A2DP sink to be registered
		time.Sleep(time.Millisecond * 500)

		connectedMediaPlayer, err = media.NewMediaControl1(dev.Path())
		if err != nil {
			return fmt.Errorf("Error creating new media control: %s", err)
		}

		time.Sleep(time.Millisecond * 500)

		core.Session.Publish("bluetooth.name", dev.Properties.Name)
		core.Settings.Publish("bluetooth.address", dev.Properties.Address)
		core.Session.Publish("bluetooth.connected", true)
		log.Info().Msgf("Connected %s successfully", dev.Properties.Address)

		watchProperties()
		GetMetadata()
	}

	return nil
}

// Refresh connected device for metadata, without connecting
func refreshConnectedDevice() error {
	log.Info().Msg("Refreshing connected device")

	a, err := api.GetDefaultAdapter()
	if err != nil {
		return err
	}
	devices, err := a.GetDevices()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get devices")
		return fmt.Errorf("GetDevices: %s", err)
	}
	if len(devices) == 0 {
		log.Error().Msg("Empty device list")
		return nil
	}

	for _, dev := range devices {
		if dev.Properties.Connected {
			connectedDevice = dev
			connectedMediaPlayer, err = media.NewMediaControl1(dev.Path())
			if err != nil {
				return fmt.Errorf("Error creating new media control: %s", err)
			}
			core.Session.Publish("bluetooth.name", dev.Properties.Name)
			core.Settings.Publish("bluetooth.address", dev.Properties.Address)
			core.Session.Publish("bluetooth.connected", true)
			log.Info().Msgf("Connected %s successfully", dev.Properties.Address)
			watchProperties()
		}
	}
	return nil
}

func watchProperties() {
	go func() {
		cd, err := connectedDevice.WatchProperties()
		if err != nil {
			log.Error().Err(err).Msg("Error creating new device control channel")
		}

		log.Info().Msg("Created properties channel for device")
		for {
			<-cd
			log.Debug().Msg("Control Properties updated")
			_, err := GetMetadata()
			if err != nil {
				log.Error().Err(err).Msg("Error refreshing device metadata")
				core.Session.Publish("bluetooth.connected", connectedDevice.Properties.Connected)
			}
		}
	}()

	go func() {
		path, err := connectedMediaPlayer.GetPlayer()
		if err != nil {
			log.Error().Err(err).Msg("Error getting path from control device")
			return
		}
		player, err := media.NewMediaPlayer1(path)
		if err != nil {
			log.Error().Err(err).Msg("Error creating new player from control path")
			return
		}
		cd, err := player.WatchProperties()
		if err != nil {
			log.Error().Err(err).Msg("Error creating new device media channel")
		}

		log.Info().Msg("Created properties channel for player")
		for {
			<-cd
			log.Debug().Msg("Media Properties updated.")
			_, err := GetMetadata()
			if err != nil {
				log.Error().Err(err).Msg("Error refreshing player metadata")
				core.Session.Publish("bluetooth.connected", connectedDevice.Properties.Connected)
			}
		}
	}()
}

// Disconnect bluetooth device
func Disconnect() error {
	log.Info().Msg("Disconnecting from bluetooth device...")

	if connectedDevice == nil {
		return fmt.Errorf("No Bluetooth device to run on")
	}
	return connectedDevice.Disconnect()
}

// ConnectNetwork starts bluetooth LTE tethering
func ConnectNetwork() error {
	if connectedDevice == nil {
		return fmt.Errorf("No Bluetooth device to run on")
	}
	address := core.Settings.Store.GetString("bluetooth.address")
	log.Info().Msgf("Connecting to bluetooth network on device %s...", address)

	// leech off device's network
	cmd := exec.Command("nmcli", "dev", "connect", address)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to enable Bluetooth Device networking: %s", err.Error())
	}

	log.Info().Msgf("Bluetooth Network Connection output: %s", string(output))
	return nil
}

// DisconnectNetwork stops bluetooth LTE tethering
func DisconnectNetwork() error {
	if connectedDevice == nil {
		return fmt.Errorf("No Bluetooth device to run on")
	}
	address := core.Settings.Store.GetString("bluetooth.address")
	log.Info().Msgf("Disconnecting bluetooth network on device %s...", address)

	cmd := exec.Command("nmcli", "dev", "disconnect", address)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to disable Bluetooth Device networking: %s", err.Error())
	}

	log.Info().Msgf("Bluetooth Network Connection output: %s", string(output))
	return nil
}

// IsPlaying determines if the given media player is playing
func IsPlaying() bool {
	if connectedMediaPlayer == nil {
		log.Error().Msg("No Bluetooth device to run on")
		return false
	}
	path, err := connectedMediaPlayer.GetPlayer()
	if err != nil {
		log.Error().Err(err).Msgf("Error getting Control player")
		return false
	}

	player, err := media.NewMediaPlayer1(path)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting Control player")
		return false
	}
	core.Session.Publish("bluetooth.status.isplaying", player.Properties.Status == "playing")

	return player.Properties.Status == "playing"
}

// GetMetadata determines if the given media player is playing
func GetMetadata() (Metadata, error) {
	m := Metadata{}

	if connectedMediaPlayer == nil {
		return m, fmt.Errorf("No Bluetooth device to run on")
	}
	path, err := connectedMediaPlayer.GetPlayer()
	if err != nil {
		return m, fmt.Errorf("Error getting Control player: %s", err)
	}

	player, err := media.NewMediaPlayer1(path)
	if err != nil {
		return m, fmt.Errorf("Error getting Control player: %s", err)
	}

	props, err := player.GetProperties()
	if err != nil {
		return m, fmt.Errorf("Error getting Control player properties: %s", err)
	}

	if props.Track["Artist"] != nil {
		m.Artist = props.Track["Artist"].(dbus.Variant).Value().(string)
	}
	if props.Track["Album"] != nil {
		m.Album = props.Track["Album"].(dbus.Variant).Value().(string)
	}
	if props.Track["Title"] != nil {
		m.Title = props.Track["Title"].(dbus.Variant).Value().(string)
	}
	if props.Track["Duration"] != nil {
		m.Duration = props.Track["Duration"].(dbus.Variant).Value().(uint32)
	}

	m.Genre = props.Genre
	m.Position = props.Position
	m.Subtype = props.Subtype
	m.IsPlaying = props.Status == "playing"

	// Use metachange intent / now playing instead of standard AVRCP
	if m.Duration == 0 {
		albumSlices := strings.Split(m.Album, "|")
		artistSlices := strings.Split(m.Artist, "-")

		if len(albumSlices) == 2 {
			durationInt, err := strconv.Atoi(strings.TrimSpace(albumSlices[0]))
			if err == nil {
				m.Duration = uint32(durationInt)
			}
			m.Subtype = strings.TrimSpace(albumSlices[1])
		}
		if len(artistSlices) == 2 {
			m.Artist = strings.TrimSpace(artistSlices[0])
			m.Album = strings.TrimSpace(artistSlices[1])
		}
	}

	m.ArtworkPath = slug.Make(m.Artist) + "/" + slug.Make(m.Album) + ".jpg"

	core.Session.Publish("bluetooth.Artist", m.Artist)
	core.Session.Publish("bluetooth.Album", m.Album)
	core.Session.Publish("bluetooth.Title", m.Title)
	core.Session.Publish("bluetooth.Duration", int(m.Duration))
	core.Session.Publish("bluetooth.Position", int(m.Position))
	core.Session.Publish("bluetooth.Genre", m.Genre)
	core.Session.Publish("bluetooth.Subtype", m.Subtype)
	core.Session.Publish("bluetooth.IsPlaying", m.IsPlaying)
	core.Session.Publish("bluetooth.ArtworkPath", m.ArtworkPath)

	return m, nil
}

// Next attempts to skip the current track
func Next() {
	if connectedMediaPlayer == nil {
		log.Error().Msg("No Bluetooth device to run on")
		return
	}
	log.Info().Msg("Going to next track...")
	err := connectedMediaPlayer.Next()
	if err != nil {
		log.Error().Err(err).Msgf("Error playing next track")
	}
}

// Prev attempts to seek backwards
func Prev() {
	if connectedMediaPlayer == nil {
		log.Error().Msg("No Bluetooth device to run on")
		return
	}
	log.Info().Msg("Going to previous track...")
	err := connectedMediaPlayer.Previous()
	if err != nil {
		log.Error().Err(err).Msgf("Error playing prev track")
	}
}

// Play attempts to play bluetooth media
func Play() {
	if connectedMediaPlayer == nil {
		log.Error().Msg("No Bluetooth device to run on")
		return
	}
	log.Info().Msg("Attempting to play media...")
	err := connectedMediaPlayer.Play()
	if err != nil {
		log.Error().Err(err).Msgf("Error playing media")
	}
}

// Pause attempts to pause bluetooth media
func Pause() {
	if connectedMediaPlayer == nil {
		log.Error().Msg("No Bluetooth device to run on")
		return
	}
	log.Info().Msg("Attempting to pause media...")
	err := connectedMediaPlayer.Pause()
	if err != nil {
		log.Error().Err(err).Msgf("Error pausing media")
	}
}
