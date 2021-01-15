package core

import (
	"time"

	"github.com/rs/zerolog/log"
)

// Message bundles the interface with the topic
type Message struct {
	Topic string
	Value interface{}
}

var (
	Settings *Datastore
	Session  *Datastore

	StartTime time.Time
)

// New creates a publish / subscribe interface for administering the program
func New() {
	Settings = NewDatastore(true)
	Session = NewDatastore(false)
	StartTime = time.Now()

	Settings.Store.SetConfigName("config") // name of config file (without extension)
	Settings.Store.SetConfigType("yaml")
	Settings.Store.AddConfigPath("/etc/mdroid/")
	Settings.Store.AddConfigPath(".")    // optionally look for config in the working directory
	err := Settings.Store.ReadInConfig() // Find and read the config file
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read config")
	}

	// Enable debugging from settings
	configureLogging(Settings.Store.GetBool("mdroid.debug"))

	log.Info().Msgf("Settings (core): %v", Settings.Store.AllSettings())
}

// Flush all settings, triggering their respective hooks
func Flush() {
	for _, key := range Settings.Store.AllKeys() {
		go publishToSubscribers(Settings.subscribers, key, Settings.Store.Get(key))
	}
}
