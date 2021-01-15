package mserial

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/tarm/serial"
)

// Device exposes a hardware serial device with two channels
type Device struct {
	Name       string `mapstructure:"device"`
	Baud       int    `mapstructure:"baud"`
	port       *serial.Port
	IsWritable bool `mapstructure:"iswriter"`

	readerMessages chan readerMessage

	writeQueueLock sync.Mutex
	writeQueue     map[uuid.UUID]*writeQueueItem
}

func (d *Device) begin() {
	for {
		log.Info().Msgf("Opening serial device %s at baud %d", d.Name, d.Baud)
		var err error
		serialConfig := &serial.Config{Name: d.Name, Baud: d.Baud, ReadTimeout: time.Second * 10}
		d.port, err = serial.OpenPort(serialConfig)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to open serial port %s", d.Name)
			time.Sleep(time.Second * 2)
			continue
		}

		// Continually read from serial port
		for {
			returnMessages, err := d.read()
			if err != nil {
				// The device is nil, break out of this read loop
				log.Error().Err(err).Msg("Failed to read from serial port")
				break
			}

			for _, msg := range returnMessages {
				d.readerMessages <- msg
			}
		}
		log.Error().Msg("Serial disconnected, closing port and reopening in 10 seconds.")

		d.port.Close()
		time.Sleep(time.Second * 10)
		log.Error().Msg("Reopening serial port...")
	}
}

// write pushes out a message to the open serial port
func (d *Device) write(msg string) error {
	if len(msg) == 0 {
		return fmt.Errorf("Empty message, not writing to serial")
	}

	if d.port == nil {
		return fmt.Errorf("Serial port for device %s not initialized", d.Name)
	}

	// Add this item to the write queue
	newID := uuid.New()
	newAwaitChannel := make(chan bool, 1)
	newItem := writeQueueItem{message: msg, isConfirmed: &newAwaitChannel, id: newID}
	d.writeQueueLock.Lock()
	d.writeQueue[newID] = &newItem
	d.writeQueueLock.Unlock()

	d.writeItem(&newItem)

	return nil
}
