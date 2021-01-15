// Package artwork provides a simple REST API for locally stored album artwork
package artwork

import (
	"net/http"

	"github.com/qcasey/mdroid/pkg/core"
	"github.com/qcasey/mdroid/pkg/server"
	"github.com/rs/zerolog/log"
)

// Start configures the extracted artwork fileserver
func Start(srv *server.Server) {
	// Check if enabled
	if !core.Settings.Store.GetBool("artwork.enabled") {
		log.Info().Msg("Started artwork without enabling in the config. Skipping module...")
		return
	}

	artDir := core.Settings.Store.GetString("artwork.directory")
	srv.Router.PathPrefix("/artwork/").Handler(http.StripPrefix("/artwork/", http.FileServer(http.Dir(artDir))))
	log.Info().Msgf("Added artwork directory %s", artDir)
}
