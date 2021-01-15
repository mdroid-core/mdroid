package camera

import (
	"fmt"
	"os"
	"time"

	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
	"gocv.io/x/gocv"
)

// config holds configuration and status of each camera
type config struct {
	deviceNumber   string
	destination    string
	commandChannel chan int
	isRecording    bool
}

var (
	configs          map[string]*config
	finishedSetup    bool
	camerasToggledOn bool
)

// Start Cameras
func Start(srv *server.Server) {

	// Check if enabled
	if !core.Settings.Store.GetBool("camera.enabled") {
		log.Info().Msg("Started camera without enabling in the config. Skipping module...")
		return
	}

	camerasToggledOn = core.Settings.Store.GetBool("camera.toggledon")
	configs = make(map[string]*config)

	for deviceNumber := range core.Settings.Store.GetStringMap("camera.devices") {
		var c config
		core.Settings.Store.UnmarshalKey(fmt.Sprintf("camera.devices.%s", deviceNumber), &c)
		c.deviceNumber = deviceNumber
		c.commandChannel = make(chan int, 1)

		configs[deviceNumber] = &c

		// If cameras are not toggled on, don't start them
		if !camerasToggledOn {
			continue
		}

		if _, err := os.Stat(fmt.Sprintf("/dev/video%s", c.deviceNumber)); err == nil {
			log.Info().Msgf("Starting camera %s", deviceNumber)
			go recordCamera(configs[deviceNumber])
		} else {
			log.Info().Msgf("Camera %s not ready, but will start when it appears", deviceNumber)
		}
	}

	finishedSetup = true
	log.Info().Msgf("Connected to %d cameras successfully", len(configs))

	// Add subscriptions to turn on/off cameras
	go func() {
		cameraHook := make(chan core.Message, 1)
		core.Settings.Subscribe("camera.toggledOn", cameraHook)

		for {
			select {
			case message := <-cameraHook:
				// Toggle enginesound on/off
				if message.Topic != "camera.toggledon" {
					continue
				}
				newCamerasToggledOn := message.Value.(bool)
				if camerasToggledOn == newCamerasToggledOn {
					continue
				}

				for _, c := range configs {
					if c.isRecording && !newCamerasToggledOn {
						c.commandChannel <- 2
					} else if !c.isRecording && newCamerasToggledOn {
						// Start camera if it isn't already
						go recordCamera(c)
					}
				}
			}
		}
	}()
}

// StartRecording from all given cameras
func StartRecording() {
	for _, c := range configs {
		go recordCamera(c)
	}
}

// StopRecording from all given cameras
func StopRecording() {
	for _, c := range configs {
		go func(c *config) {
			c.commandChannel <- 2
		}(c)
	}
}

func recordCamera(c *config) {
	webcam, err := gocv.OpenVideoCapture(c.deviceNumber)
	if err != nil {
		log.Error().Err(err).Msgf("Error opening video capture device: %v\n", c.deviceNumber)
		return
	}
	defer webcam.Close()

	img := gocv.NewMat()
	defer img.Close()

	if ok := webcam.Read(&img); !ok {
		log.Error().Err(err).Msgf("Error reading video capture device: %v\n", c.deviceNumber)
		return
	}

	writer, err := gocv.VideoWriterFile(fmt.Sprintf("%s/video%s-%s.avi", c.destination, c.deviceNumber, time.Now().Format(time.RFC3339)), "MJPG", 25, img.Cols(), img.Rows(), true)
	if err != nil {
		log.Error().Err(err).Msgf("Error writing video capture device: %v\n", c.deviceNumber)
		return
	}
	defer writer.Close()
	c.isRecording = true
	log.Info().Msgf("Started recording from camera %s", c.deviceNumber)

	for {
		select {
		case i := <-c.commandChannel:
			if i == 2 {
				return
			}
		default:
		}

		if ok := webcam.Read(&img); !ok {
			log.Error().Err(err).Msgf("Device closed: %v\n", c.deviceNumber)
			return
		}
		if img.Empty() {
			continue
		}

		writer.Write(img)
	}
}
