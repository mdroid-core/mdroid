package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server/routes/session"
	"github.com/qcasey/mdroid/pkg/server/routes/settings"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Server binds the interal MDroid core and router together
type Server struct {
	Router *mux.Router
}

// Module specifies the start and stop for MDroid Server modules
type Module func(*Server, *sync.WaitGroup)

// mdroidRoute holds information for our meta /routes output
type mdroidRoute struct {
	Path    string `json:"Path"`
	Methods string `json:"Methods"`
}

var routes []mdroidRoute
var moduleCount int
var moduleGroup *sync.WaitGroup

// New creates a new server with underlying Core
func New() *Server {
	srv := &Server{
		Router: mux.NewRouter(),
	}

	// Setup core
	core.New()

	// Setup router
	srv.injectRoutes()
	return srv
}

// AddModule to the server
func (srv *Server) AddModule(mod Module) {
	moduleGroup.Add(moduleCount)
	mod(srv, moduleGroup)
	moduleCount++
}

// WaitForModules to complete setup
func (srv *Server) WaitForModules() {
	moduleGroup.Wait()
}

// Start configures default MDroid routes, starts router with optional middleware if configured
func (srv *Server) Start() {
	// Walk routes
	err := srv.Router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		var newroute mdroidRoute

		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			newroute.Path = pathTemplate
		}
		methods, err := route.GetMethods()
		if err == nil {
			newroute.Methods = strings.Join(methods, ",")
		}
		routes = append(routes, newroute)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to walk server routes")
	}

	httpServer := &http.Server{
		Handler:      srv.Router,
		Addr:         fmt.Sprintf("0.0.0.0:5353"),
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  20 * time.Second,
	}

	log.Info().Msg("Starting server...")

	// Start the router in an endless loop
	for {
		err := httpServer.ListenAndServe()
		log.Error().Err(err).Msg("Router failed unexpectedly. Restarting in 10 seconds...")
		time.Sleep(time.Second * 10)
	}
}

func (srv *Server) injectRoutes() {
	//
	// Debug route
	//
	srv.Router.HandleFunc("/debug/level/{level}", handleChangeLogLevel).Methods("GET")

	//
	// Session routes
	//
	srv.Router.HandleFunc("/session", session.GetAll()).Methods("GET")
	srv.Router.HandleFunc("/session/{name}", session.Get()).Methods("GET")
	srv.Router.HandleFunc("/session/{name}", session.Set()).Methods("POST")

	//
	// Settings routes
	//
	srv.Router.HandleFunc("/settings", settings.GetAll()).Methods("GET")
	srv.Router.HandleFunc("/settings/{key}", settings.Get()).Methods("GET")
	srv.Router.HandleFunc("/settings/{key}/{value}", settings.Set()).Methods("POST")

	//
	// Welcome route
	//
	srv.Router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		core.WriteNewResponse(&w, r, core.JSONResponse{Output: routes, OK: true})
	}).Methods("GET")
}

func handleChangeLogLevel(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	level := core.FormatName(params["level"])
	switch level {
	case "INFO":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "DEBUG":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "ERROR":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		core.WriteNewResponse(&w, r, core.JSONResponse{Output: "Invalid log level.", OK: false})
	}
	core.WriteNewResponse(&w, r, core.JSONResponse{Output: level, OK: true})
}
