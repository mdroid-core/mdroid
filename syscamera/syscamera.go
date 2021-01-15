package syscamera

import (
	"syscall"
	"time"

	"github.com/qcasey/mdroid/systemd"
	"github.com/rs/zerolog/log"
)

// StartCameras and log errors
func StartCameras() {
	err := systemd.StartService("record")
	if err != nil {
		log.Error().Err(err).Msg("Failed to start record service")
	}

	err = systemd.StartService("stream")
	if err != nil {
		log.Error().Err(err).Msg("Failed to start stream service")
	}
	//c.Publish("cameras", core.Session, true)
	log.Info().Msg("Started cameras.")
}

// StopCameras and log errors
func StopCameras() {
	err := systemd.StopService("record")
	if err != nil {
		log.Error().Err(err).Msg("Failed to stop record service")
	}

	err = systemd.StopService("stream")
	if err != nil {
		log.Error().Err(err).Msg("Failed to stop stream service")
	}

	time.Sleep(time.Second * 2)
	err = syscall.Unmount("/videos", syscall.MNT_DETACH)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unmount videos")
	}
	//c.Publish("cameras", core.Session, false)
	log.Info().Msg("Stopped cameras.")
}
