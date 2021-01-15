package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/qcasey/viper"
	"github.com/rs/zerolog/log"
)

// Datastore helps select between sessions or settings backend
type Datastore struct {
	Store          *viper.Viper
	Stats          *viper.Viper
	subscribers    map[string][]chan Message
	mutex          sync.Mutex
	hasIndexOnDisk bool
}

// NewDatastore creates a new datastore with default values
func NewDatastore(hasIndexOnDisk bool) *Datastore {
	return &Datastore{
		Store:          viper.New(),
		Stats:          viper.New(),
		subscribers:    make(map[string][]chan Message),
		mutex:          sync.Mutex{},
		hasIndexOnDisk: hasIndexOnDisk,
	}
}

// Subscribe will add the given channel as a listener to a topic,
// Returning a well formatted message when that topic is updated
// Topic is expected to be compatible with a Viper selector
// Channel is expected to be buffered
func (ds *Datastore) Subscribe(topic string, ch chan Message) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	formattedTopic := strings.ToLower(topic)
	ds.subscribers[formattedTopic] = append(ds.subscribers[formattedTopic], ch)
}

// Publish a given message to all subscribed entities
// Topic is expected to be compatible with a Viper selector
func (ds *Datastore) Publish(topic string, m interface{}) {
	itemExists := ds.Store.IsSet(topic)
	oldItem := ds.Store.Get(topic)
	subscribers := ds.subscribers

	ds.Store.Set(fmt.Sprintf("%s", topic), m)
	ds.Stats.Set(fmt.Sprintf("%s.write_date", topic), time.Now())
	ds.Stats.Set(fmt.Sprintf("%s.writes", topic), ds.Stats.GetInt(fmt.Sprintf("%s.writes", topic))+1)

	// write to disk if configured
	if ds.hasIndexOnDisk {
		err := ds.Store.WriteConfig()
		if err != nil {
			log.Error().Err(err).Msg("Failed to write viper config file")
		}

	} else if itemExists {
		// (Session typically)
		// Does not have disk index, meaning this holds less important states.
		// Exit if this is not a new item
		if oldItem == m {
			return
		}

		// Special case for GPS values, calculate difference in distance
		// IFF both Lat/Long have been defined
		// AND the last write date was within 15 minutes
		if (topic == "gps.lat" || topic == "gps.lng") &&
			ds.Store.IsSet("gps.lat") && ds.Store.IsSet("gps.lng") &&
			time.Since(ds.Stats.GetTime(fmt.Sprintf("%s.write_date", topic))) < time.Minute*15 {

			significantlyDifferent, err := ds.gpsPointsAreSignificantlyDifferent(topic, m)
			if err != nil {
				log.Error().Err(err).Msg("Failed to compare GPS points")
				return
			}

			if !significantlyDifferent {
				return
			}
		}
	}

	go publishToSubscribers(subscribers, topic, m)
}

func publishToSubscribers(subscribers map[string][]chan Message, topic string, m interface{}) {
	for _, ch := range subscribers[strings.ToLower(topic)] {
		select {
		case ch <- Message{Topic: topic, Value: m}:
			continue
		case <-time.After(2 * time.Second):
			log.Error().Msgf("A subscriber on topic %s took too long to consume message. Timing out and moving on.", topic)
			continue

		}
	}

	// Global subscribers
	for _, ch := range subscribers["*"] {
		select {
		case ch <- Message{Topic: topic, Value: m}:
			continue
		case <-time.After(2 * time.Second):
			log.Error().Msgf("A subscriber on topic * took too long to consume message. Timing out and moving on.")
			continue
		}
	}
}
