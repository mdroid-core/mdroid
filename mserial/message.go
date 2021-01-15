package mserial

import (
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type writeQueueItem struct {
	id          uuid.UUID
	message     string
	isConfirmed *chan bool
}

func (d *Device) writeItem(wq *writeQueueItem) {
	_, err := d.port.Write([]byte(wq.message + "\n"))
	if err != nil {
		log.Error().Err(err).Msgf("Failed to write mserial queue item %s", wq.message)
	}
	select {
	case <-*wq.isConfirmed:
		log.Info().Msgf("Successfully wrote message %s (%s)", wq.message, wq.id.String())
	case <-time.After(200 * time.Millisecond):
		log.Info().Msgf("Message %s (%s) timed out, rewriting (%d in queue)...", wq.message, wq.id.String(), len(d.writeQueue))
		d.writeItem(wq)
		/*
			d.writeQueueLock.Lock()
			delete(d.writeQueue, wq.id)
			d.writeQueueLock.Unlock()
			return d.write(msg)*/
	}
}
