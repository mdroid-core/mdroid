package prometheus

import (
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
)

var (
	messageCounterRegistry = make(map[string]prometheus.Counter)
	messageGaugeRegistry   = make(map[string]prometheus.Gauge)
	registryLock           sync.Mutex
)

// Start will set up the prometheus metric handler
func Start(srv *server.Server) {
	// Check if enabled
	if !core.Settings.Store.GetBool("prometheus.enabled") {
		log.Info().Msg("Started Prometheus Exporter without enabling in the config. Skipping module...")
		return
	}

	//
	// Prometheus Exporter Routes
	//
	srv.Router.Path("/metrics").Handler(promhttp.Handler())

	log.Info().Msgf("Successfully started prometheus exporter")

	go func() {
		// Setup channels for meta window/door status
		allMessages := make(chan core.Message, 1)
		core.Session.Subscribe("*", allMessages)
		for {
			select {
			case m := <-allMessages:
				go exportMessage(m)
			}
		}
	}()
}

func exportMessage(m core.Message) {
	registryLock.Lock()
	counter, counterExists := messageCounterRegistry[m.Topic]
	gauge, gaugeExists := messageGaugeRegistry[m.Topic]
	registryLock.Unlock()

	if !counterExists || !gaugeExists {
		validName := strings.Replace(m.Topic, ".", "_", -1)
		counter = prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_updates", validName),
			},
		)
		gauge = prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: validName,
			},
		)
		prometheus.Register(counter)
		prometheus.Register(gauge)

		registryLock.Lock()
		messageCounterRegistry[m.Topic] = counter
		messageGaugeRegistry[m.Topic] = gauge
		registryLock.Unlock()

		log.Debug().Msgf("Added %s to list of %d metrics", validName, len(messageCounterRegistry))
	}
	gauge.Set(core.Session.Store.GetFloat64(m.Topic))
	counter.Inc()
}
