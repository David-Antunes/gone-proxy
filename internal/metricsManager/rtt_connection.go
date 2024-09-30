package metricsManager

import (
	"net"
)

type RttConnection struct {
	Mac  net.HardwareAddr `json:"mac"`
	IP   net.IP           `json:"ip"`
	Port int              `json:"port"`
}
