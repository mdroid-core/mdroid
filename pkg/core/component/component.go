package component

import (
	"fmt"
	"strings"
	"time"

	"github.com/qcasey/mdroid/mserial"
	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

// Component to handle
type Component struct {
	Name             string
	LastChangedState time.Time
	Hook             chan core.Message

	ShouldBeOnWhen func() (bool, string)
	TurnOn         func()
	TurnOff        func()
}

// NewWithDefaults creates a new default component
func NewWithDefaults(name string, shouldBeOnWhen func() (bool, string)) *Component {
	return &Component{
		Name: name,
		Hook: make(chan core.Message, 1),

		ShouldBeOnWhen: shouldBeOnWhen,
		TurnOn: func() {
			log.Info().Msgf(strings.ToLower(name))
			mserial.Await(fmt.Sprintf("powerOn:%s", strings.ToLower(name)))
		},
		TurnOff: func() {
			mserial.Await(fmt.Sprintf("powerOff:%s", strings.ToLower(name)))
		},
	}
}

// New creates a new default component with custom on/off functions
func New(name string, shouldBeOnWhen func() (bool, string), on func(), off func()) *Component {
	return &Component{
		Name: name,
		Hook: make(chan core.Message, 1),

		ShouldBeOnWhen: shouldBeOnWhen,
		TurnOn:         on,
		TurnOff:        off,
	}
}

// Evaluate if a component should be on or off, then take that action if it doesn't match the state
func (comp *Component) Evaluate() {
	componentIsOn := core.Session.Store.GetBool(comp.Name)
	componentSetting := strings.ToUpper(core.Settings.Store.GetString(fmt.Sprintf("components.%s", comp.Name)))
	componentShouldBeOn, reason := comp.ShouldBeOnWhen()

	log.Debug().Msgf("%v %s", componentIsOn, componentSetting)

	var toggleStatus string
	var willToggleOn bool

	switch componentSetting {
	case "ON":
		willToggleOn = true
	case "OFF":
		willToggleOn = false
	case "AUTO":
		willToggleOn = componentShouldBeOn
	}

	// If the state doesn't need to change, exit
	if willToggleOn == componentIsOn {
		log.Debug().Msgf("No need to change component %s state. %v == %v", comp.Name, componentShouldBeOn, componentIsOn)
		return
	}

	if willToggleOn {
		toggleStatus = "on"
		go comp.TurnOn()
	} else {
		toggleStatus = "off"
		go comp.TurnOff()
	}

	// Log and set next time threshold
	if componentSetting != "AUTO" {
		reason = fmt.Sprintf("target is %s", componentSetting)
	}
	log.Info().Msgf("Powering %s %s, because %s", toggleStatus, comp.Name, reason)
	comp.LastChangedState = time.Now()
}
