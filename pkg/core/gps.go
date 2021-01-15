package core

import (
	"strconv"

	geo "github.com/kellydunn/golang-geo"
	"github.com/rs/zerolog/log"
)

func (ds *Datastore) gpsPointsAreSignificantlyDifferent(topic string, m interface{}) (bool, error) {
	oldLat, err := strconv.ParseFloat(ds.Store.GetString("gps.lat"), 64)
	if err != nil {
		log.Error().Err(err).Msg("Failed to convert gps.lat into a float64")
		return false, err
	}

	oldLng, err := strconv.ParseFloat(ds.Store.GetString("gps.lng"), 64)
	if err != nil {
		return false, err
	}

	var newPoint *geo.Point
	oldPoint := geo.NewPoint(oldLat, oldLng)
	switch topic {
	case "gps.lng":
		newLng, err := strconv.ParseFloat(m.(string), 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert new lng into a float64")
			return false, err
		}
		newPoint = geo.NewPoint(oldLat, newLng)

	case "gps.lat":
		newLat, err := strconv.ParseFloat(m.(string), 64)
		if err != nil {
			log.Error().Err(err).Msg("Failed to convert new lat into a float64")
			return false, err
		}
		newPoint = geo.NewPoint(newLat, oldLng)
	}

	// Significantly different if the distance is greater than 0.2 km
	return oldPoint.GreatCircleDistance(newPoint) > 0.1, nil
}
