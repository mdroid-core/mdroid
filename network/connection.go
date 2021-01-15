package network

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/qcasey/mdroid/pkg/core"
	"github.com/rs/zerolog/log"
)

// ConnectLTE brings up LTE from ipconfig
func ConnectLTE() error {
	if !core.Session.Store.GetBool("lte") {
		err := setInterfaceState("usb0", "up")
		if err != nil {
			return err
		}
	}
	return setDefaultRoute("dev", "usb0")
}

// DisconnectLTE brings down LTE from ipconfig
func DisconnectLTE() error {
	if !core.Session.Store.IsSet("lte") || core.Session.Store.GetBool("lte") {
		err := setInterfaceState("usb0", "down")
		if err != nil {
			return err
		}
	}
	return setDefaultRoute("via", "10.0.3.1")
}

// ConnectHardlink brings up hardlink from ipconfig
/*
func ConnectHardlink() error {
	return setInterfaceState("veth-netns0", "up") && setDefaultRoute("veth-netns0")
}

// DisconnectHardlink brings down hardlink from ipconfig
func DisconnectHardlink() error {
	return setInterfaceState("veth-netns0", "down") && setDefaultRoute("usb0")
}*/

func setDefaultRoute(typeName string, interfaceName string) error {
	if core.Session.Store.IsSet("network.default_route") &&
		strings.Contains(core.Session.Store.GetString("network.default_route"), interfaceName) {
		log.Info().Msgf("%s is already the default route, ignoring request to change", interfaceName)
		return nil
	}

	// Connect / disconnect LTE
	cmd := exec.Command("ip", "route", "del", "default")
	cmd.Run()

	cmd = exec.Command("ip", "route", "add", "0.0.0.0/0", typeName, interfaceName, "metric", "100")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to make interface %s default route: %s", interfaceName, err.Error())
	}

	log.Info().Msgf("Set %s as the default route", interfaceName)
	//mqtt.ForceReconnection()
	return nil
}

func setInterfaceState(state string, interfaceName string) error {
	// Connect / disconnect LTE
	cmd := exec.Command("ip", "link", "set", state, interfaceName)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Failed to %s interface %s: %s", state, interfaceName, err.Error())
	}
	log.Info().Msgf("Brought %s %s", interfaceName, state)
	return nil
}

// GetNetworkState by checking `ip a` in `/sys/class/net` for up/down status
func GetNetworkState(fileName string) bool {
	if _, err := os.Stat(fileName); err == nil {
		output, err := ioutil.ReadFile(fileName)
		if err != nil {
			log.Error().Msgf("File reading error: %s", err.Error())
		} else {
			trimmedOutput := strings.TrimSpace(string(output))
			return trimmedOutput == "up" || trimmedOutput == "unknown"
		}
	}
	return false
}

// GetDefaultRoute polls the default route from ip
func GetDefaultRoute() string {
	cmd := exec.Command("ip", "route", "show", "default")
	out, err := cmd.Output()

	if err != nil {
		log.Error().Err(err).Msg("Failed to get default route")
		return "UNKNOWN"
	}
	return string(out)
}

// PollDefaultRoute to prevent unnecessary route changes
func PollDefaultRoute() {
	for {
		core.Session.Publish("network.default_route", GetDefaultRoute())
		time.Sleep(300 * time.Millisecond)
	}
}

// PollNetworkState to check for interface changes
func PollNetworkState(interfaceName string, sessionBooleanValue string) {
	fileName := fmt.Sprintf("/sys/class/net/%s/operstate", interfaceName)
	for {
		core.Session.Publish(sessionBooleanValue, GetNetworkState(fileName))
		time.Sleep(250 * time.Millisecond)
	}
}
