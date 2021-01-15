package mserial

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type readerMessage struct {
	key   string
	value interface{}
}

var allowedKeys [11]string

func init() {
	allowedKeys = [...]string{
		"angel_eyes",
		"usb_hub",
		"board",
		"door_locks",
		"key_power",
		"acc_power",
		"unlock_power",
		"rand_1",
		"aux_voltage_raw",
		"main_voltage_raw",
		"gps",
	}
}

func isKeyAllowed(key string) bool {
	for _, k := range allowedKeys {
		if key == k {
			return true
		}
	}
	return false
}

// readSerial takes one line from the serial device and parses it into the session
func (d *Device) read() ([]readerMessage, error) {
	var (
		returnMessages []readerMessage
		err            error
		msg            []byte
	)

	buf := make([]byte, 1)

	for {
		_, err = d.port.Read(buf)
		if err != nil {
			return returnMessages, nil
		}
		msg = append(msg, buf[0])

		if buf[0] == '\n' {
			break
		}
	}

	if len(msg) == 0 {
		return returnMessages, nil
	}

	if serialLog != nil {
		serialLog.Print(string(msg))
	}

	// Parse serial data
	var jsonData interface{}
	err = json.Unmarshal(msg, &jsonData)
	if err != nil {
		go func() {
			d.writeQueueLock.Lock()
			foundCheckItem := false
			for id, item := range d.writeQueue {
				if strings.TrimSpace(string(msg)) == item.message {
					foundCheckItem = true
					select {
					case *item.isConfirmed <- true:
						delete(d.writeQueue, id)
					case <-time.After(1000 * time.Millisecond):
					}
					break
				}
			}
			if !foundCheckItem {
				log.Error().Msgf("Could not marshal serial data '%v'", string(msg))
			}
			d.writeQueueLock.Unlock()
		}()
		return returnMessages, nil
	}
	if jsonData == nil {
		return returnMessages, nil
	}

	// Handle parse errors here instead of passing up
	data, ok := jsonData.(map[string]interface{})
	if !ok {
		return returnMessages, fmt.Errorf("Failed to cast json data to map: '%v'", jsonData)
	}

	// Switch through various types of JSON data
	for key, value := range data {
		if !isKeyAllowed(key) {
			log.Error().Msgf("Ignoring unknown key %s with value %v", key, value)
			continue
		}

		switch vv := value.(type) {
		case string, float64, int, bool:
			returnMessages = append(returnMessages, readerMessage{key: key, value: vv})
		case map[string]interface{}:
			if key == "gps" {
				for k, v := range vv {
					var (
						gpsValue interface{}
						ok       bool
					)

					gpsValueString, ok := v.(string)
					if !ok {
						return returnMessages, fmt.Errorf("Failed to cast GPS value to string %v", v)
					}
					gpsValueString = strings.TrimSpace(gpsValueString)

					if k == "speed" || k == "alt" {
						var err error
						gpsValue, err = strconv.ParseFloat(gpsValueString, 64)
						if err != nil {
							return returnMessages, fmt.Errorf("Failed to cast GPS value to float64 %v", v)
						}
					} else {
						gpsValue = gpsValueString
					}

					returnMessages = append(returnMessages, readerMessage{fmt.Sprintf("gps.%s", k), gpsValue})
				}
			}
		default:
			return returnMessages, fmt.Errorf("%s is of a type I don't know how to handle (%s: %s)", key, vv, value)
		}
	}

	return returnMessages, nil
}
