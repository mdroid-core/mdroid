package logfile

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	zerolog "github.com/rs/zerolog/log"
)

// NewLogFile creates a new file with a temporary name, discontinuing the old one
func NewLogFile(directory string) *log.Logger {
	// Retire any old log files
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		zerolog.Error().Err(err).Msgf("Failed to read directory %s", directory)
		return nil
	}
	for _, f := range files {
		if strings.Contains(f.Name(), ".log.tmp") {
			os.Rename(
				fmt.Sprintf("%s%s", directory, f.Name()),
				fmt.Sprintf("%s%s", directory, strings.ReplaceAll(f.Name(), ".log.tmp", ".log")),
			)
		}
	}

	// Create new log file
	f, err := os.OpenFile(fmt.Sprintf("%s%s.log.tmp", directory, time.Now().Format(time.RFC3339)),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		zerolog.Error().Err(err).Msgf("Failed to create log file at %s", directory)
	}
	return log.New(f, "", log.LstdFlags)
}
