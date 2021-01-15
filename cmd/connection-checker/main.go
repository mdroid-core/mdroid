// Will read an mdroid config, and post connection up/down status
// for all interfaces found under `ip a`
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/qcasey/mdroid/network"
	"github.com/rs/zerolog/log"
)

var mdroidAPI string

func makeRequest(url string, value bool) {
	//log.Info().Msgf("%s %t", url, value)
	reqBody, err := json.Marshal(map[string]bool{
		"Value": value,
	})
	if err != nil {
		log.Err(err)
		return
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Err(err)
		return
	}
	defer resp.Body.Close()

	if _, err := ioutil.ReadAll(resp.Body); err != nil {
		log.Err(err)
		return
	}
}

func main() {
	mdroidAPI = os.Args[1]
	log.Info().Msgf("Reporting connection status for all interfaces to %s", mdroidAPI)

	interfaceBluetooth := fmt.Sprintf("%s/session/network.bnep0", mdroidAPI)
	interfaceWifi1 := fmt.Sprintf("%s/session/network.wlan0", mdroidAPI)
	interfaceWifi2 := fmt.Sprintf("%s/session/network.wlan1", mdroidAPI)
	interfaceEth0 := fmt.Sprintf("%s/session/network.eth0", mdroidAPI)

	for {
		go makeRequest(interfaceBluetooth, network.GetNetworkState("/sys/class/net/bnep0/operstate"))
		go makeRequest(interfaceWifi1, network.GetNetworkState("/sys/class/net/wlan0/operstate"))
		go makeRequest(interfaceWifi2, network.GetNetworkState("/sys/class/net/wlan1/operstate"))
		go makeRequest(interfaceEth0, network.GetNetworkState("/sys/class/net/eth0/operstate"))

		time.Sleep(250 * time.Millisecond)
	}
}
