package mqtt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
)

// Config holds configuration and status of MQTT
type Config struct {
	Address         string `mapstructure:"address"`
	Clientid        string `mapstructure:"client_id"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	IsVerboseClient bool   `mapstructure:"verbose"`
	client          mqtt.Client

	outage         bool
	waitingPackets int
	lock           sync.Mutex
}

type remoteMessage struct {
	Method   string `json:"method,omitempty"`
	Path     string `json:"path,omitempty"`
	PostData string `json:"postData,omitempty"`
}

const (
	disconnectedWaitTime = 200
)

var (
	configs       []*Config
	verboseTopics []string
	finishedSetup bool
)

// Start MQTT
func Start(srv *server.Server) {

	// Check if enabled
	if !core.Settings.Store.GetBool("mqtt.enabled") {
		log.Warn().Msg("Started mqtt without enabling in the config. Skipping module...")
		return
	}

	err := core.Settings.Store.UnmarshalKey("mqtt.connections", &configs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decode MQTT instance")
		return
	}

	verboseTopics = core.Settings.Store.GetStringSlice("mqtt.verbose_topics")

	for _, mqttInstance := range configs {
		connect(mqttInstance)
	}
	finishedSetup = true
	log.Info().Msgf("Added %d MQTT servers", len(configs))

	go func() {
		// Setup channels
		mqttSettingsHook := make(chan core.Message, 1)
		mqttSessionHook := make(chan core.Message, 1)

		core.Settings.Subscribe("*", mqttSettingsHook)
		core.Session.Subscribe("*", mqttSessionHook)
		for {
			select {
			case message := <-mqttSettingsHook:
				handleStateUpdate(fmt.Sprintf("settings/%s", message.Topic), message.Value)
			case message := <-mqttSessionHook:
				handleStateUpdate(fmt.Sprintf("session/%s", message.Topic), message.Value)
			}
		}
	}()
}

func handleStateUpdate(topic string, value interface{}) {
	var valueString string
	switch vv := value.(type) {
	case bool:
		valueString = fmt.Sprintf("%t", vv)
	case uint8:
		valueString = fmt.Sprintf("%d", vv)
	case uint16:
		valueString = fmt.Sprintf("%d", vv)
	case int:
		valueString = fmt.Sprintf("%d", vv)
	case int64:
		valueString = fmt.Sprintf("%d", vv)
	case float64:
		valueString = fmt.Sprintf("%f", vv)
	case string:
		valueString = strings.ToLower(vv)
	case []string:
		valueString = strings.Join(vv, ",")
	case []interface{}:
		if len(vv) > 1 {
			valueString = fmt.Sprintf("%v", vv)
		} else {
			handleStateUpdate(topic, vv[0])
		}
	case map[interface{}]interface{}:
		for newKey, newValue := range vv {
			handleStateUpdate(fmt.Sprintf("%s/%s", topic, newKey.(string)), newValue)
		}
		return
	default:
		log.Warn().Msgf("Received invalid interface (%s) for key %s. Got: '%v'", reflect.TypeOf(value).String(), topic, vv)
		return
	}

	validMQTTtopic := strings.ToLower(strings.ReplaceAll(topic, ".", "/"))

	// Don't publish high speed data to remote
	isVerboseMessage := false
	for _, t := range verboseTopics {
		if strings.ToLower(t) == validMQTTtopic {
			isVerboseMessage = true
		}
	}

	go Publish(validMQTTtopic, valueString, isVerboseMessage)
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Info().Msgf("MQTT Message: %s => %s", msg.Topic(), msg.Payload())

	request := remoteMessage{}
	err := json.Unmarshal(msg.Payload(), &request)

	var response *http.Response

	if request.Method == "POST" {
		jsonStr := []byte(request.PostData)
		req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:5353%s", request.Path), bytes.NewBuffer(jsonStr))
		if err != nil {
			log.Error().Err(err).Msg("Could not forward request from websocket.")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		httpClient := &http.Client{}
		go func() {
			response, err = httpClient.Do(req)
			if err != nil {
				log.Error().Err(err).Msg("Could not forward request from websocket.")
				return
			}
			if response != nil {
				response.Body.Close()
			}
		}()
	} else if request.Method == "GET" {
		go func() {
			response, err = http.Get(fmt.Sprintf("http://localhost:5353%s", request.Path))
			if err != nil {
				log.Error().Err(err).Msg("Could not forward request from websocket.")
				return
			}
			if response != nil {
				response.Body.Close()
			}
		}()
	}
}

// Publish writes the given message to the given topic and wait
func Publish(topic string, message string, isVerboseMessage bool) error {
	for _, m := range configs {
		// Don't publish verbose messages to remote servers
		if isVerboseMessage && !m.IsVerboseClient {
			continue
		}

		flaggedWaiting := false
		for {
			if !finishedSetup {
				log.Debug().Msgf("MQTT setup is not complete")
			} else if m.client == nil {
				log.Debug().Msgf("%s client is nil", m.Address)
			} else if !m.client.IsConnectionOpen() {
				log.Debug().Msgf("%s client is not connected, cannot publish %s", m.Address, topic)
			} else {
				break
			}

			// Mark this packet as being unsent and waited on
			if !flaggedWaiting {
				m.lock.Lock()
				m.waitingPackets++
				flaggedWaiting = true
				// Manage outages
				if m.waitingPackets%5 == 0 {
					m.outage = true
					log.Warn().Msgf("Still not connected, %d packets in queue", m.waitingPackets)
				}
				m.lock.Unlock()
			}

			time.Sleep(disconnectedWaitTime * time.Millisecond)
		}
		for {
			if token := m.client.Publish(fmt.Sprintf("vehicle/%s", topic), 2, true, message); token.Wait() && token.Error() != nil {
				log.Error().Err(token.Error()).Msgf("Failed to write %s to %s", message, topic)
				time.Sleep(disconnectedWaitTime * time.Millisecond)
			} else {
				break
			}
		}

		if m.waitingPackets > 0 {
			m.lock.Lock()
			m.waitingPackets--
			if m.outage && m.waitingPackets == 0 {
				log.Info().Msgf("Successfully published all waiting packets to %s.", m.Address)
			}
			m.outage = false
			m.lock.Unlock()
		}
	}

	return nil
}

// ForceReconnection to reestablish remote MQTT connections
func ForceReconnection() {
	for _, m := range configs {
		if !m.IsVerboseClient {
			if m.client.IsConnectionOpen() {
				m.client.Disconnect(0)
			}
			m.client.Connect()
		}
	}
}

func checkReconnection(config *Config) {
	for {
		if finishedSetup && !config.client.IsConnected() {
			log.Error().Msgf("Connection to %s lost. Retrying...", config.Address)
			if token := config.client.Connect(); token.Wait() && token.Error() != nil {
				log.Error().Msgf("Failed to reconnect to %s. Retrying...", config.Address)
				continue
			}
			log.Info().Msgf("Reconnected to %s successfully.", config.Address)
			//connect(config)
		}
		time.Sleep(1000 * time.Millisecond)
	}
}

func connect(mqttConfig *Config) {
	// Remote Client
	opts := mqtt.NewClientOptions().AddBroker(mqttConfig.Address).SetClientID(mqttConfig.Clientid).SetAutoReconnect(true)
	opts.SetCleanSession(false)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetUsername(mqttConfig.Username)
	opts.SetPassword(mqttConfig.Password)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(15 * time.Second)

	mqttConfig.client = mqtt.NewClient(opts)
	if token := mqttConfig.client.Connect(); token.Wait() && token.Error() != nil {
		log.Error().Err(token.Error()).Msgf("Failed to setup %s, waiting half a second and retrying...", mqttConfig.Address)
		go func() {
			time.Sleep(500 * time.Millisecond)
			connect(mqttConfig)
		}()
		return
	}

	if token := mqttConfig.client.Subscribe("vehicle/requests/#", 2, nil); token.Wait() && token.Error() != nil {
		log.Error().Err(token.Error()).Msgf("Failed to subscribe")
	}

	go checkReconnection(mqttConfig)

	log.Info().Msgf("Successfully connected to %s", mqttConfig.Address)
}
