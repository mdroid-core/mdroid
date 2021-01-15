package main

import (
	"github.com/qcasey/mdroid/artwork"
	"github.com/qcasey/mdroid/bluetooth"
	"github.com/qcasey/mdroid/can"
	"github.com/qcasey/mdroid/enginesound"
	"github.com/qcasey/mdroid/kbus"
	"github.com/qcasey/mdroid/mqtt"
	"github.com/qcasey/mdroid/mserial"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/qcasey/mdroid/prometheus"
)

func main() {
	// srv.Router is open to route modifications, middleware, etc
	srv := server.New()

	// Create subscriptions
	go customHooks()

	// Start modules
	prometheus.Start(srv)
	mserial.Start(srv)
	bluetooth.Start(srv)
	kbus.Start(srv)
	can.Start(srv)
	mqtt.Start(srv)
	enginesound.Start(srv)
	artwork.Start(srv)

	// Flush settings
	go core.Flush()

	// Start MDroid Core
	srv.Start()
}
