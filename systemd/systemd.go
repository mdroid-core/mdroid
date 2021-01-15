package systemd

import "os/exec"

func doService(service string, action string) error {
	cmd := exec.Command("systemctl", action, service)
	return cmd.Run()
}

// StartService will request a start to the given .service
func StartService(service string) error {
	return doService(service, "start")
}

// RestartService will request a restart to the given .service
func RestartService(service string) error {
	return doService(service, "restart")
}

// StopService will request a stop to the given .service
func StopService(service string) error {
	return doService(service, "stop")
}
