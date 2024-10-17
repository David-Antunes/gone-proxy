package main

import (
	"github.com/David-Antunes/gone-proxy/internal"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/David-Antunes/gone-proxy/internal/daemon"
	"github.com/David-Antunes/gone-proxy/internal/metricsManager"
	"github.com/David-Antunes/gone-proxy/xdp"
	"github.com/spf13/viper"
)

var proxyLog = log.New(os.Stdout, "PROXY INFO: ", log.Ltime)

func cleanup(d *daemon.Daemon, m *metricsManager.MetricsManager) {
	go func() {
		<-internal.Stop
		d.Close()
		m.Close()
		os.Exit(1)
	}()
}
func main() {
	viper.SetConfigFile(".env")
	viper.ReadInConfig()
	viper.SetDefault("PROXY_SERVER", "/tmp/proxy-server.sock")
	viper.SetDefault("PROXY_RTT_SOCKET", "/tmp/proxy-rtt.sock")
	viper.SetDefault("TIMEOUT", 60000)
	viper.SetDefault("NUM_TESTS", 100)
	viper.SetConfigType("env")
	viper.WriteConfigAs(".env")
	viper.AutomaticEnv()

	for id, value := range viper.AllSettings() {
		proxyLog.Println(id, value)
	}

	os.Remove(viper.GetString("PROXY_SERVER"))
	os.Remove(viper.GetString("PROXY_RTT_SOCKET"))

	server := daemon.NewDaemon(viper.GetString("PROXY_SERVER"))

	metricsIp, metricsMac, broadcastIP := GetIfaceInformation()

	rtt, err := xdp.CreateXdpBpfSock(0, "veth1")
	if err != nil {
		panic(err)
	}

	rttConn := &metricsManager.RttConnection{
		Mac:  []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		IP:   broadcastIP,
		Port: 8000,
	}
	metrics := metricsManager.NewMetricsManager(rtt, metricsMac, metricsIp, 8000, rttConn, viper.GetString("PROXY_RTT_SOCKET"), time.Duration(viper.GetInt("TIMEOUT"))*time.Millisecond, viper.GetInt("NUM_TESTS"))
	cleanup(server, metrics)
	go metrics.Start()
	server.Serve()

}

func GetIfaceInformation() (net.IP, net.HardwareAddr, net.IP) {
	ief, err := net.InterfaceByName("br0")
	if err != nil {
		panic(err)
	}
	addrs, err := ief.Addrs()
	if err != nil {
		panic(err)
	}
	ip := strings.Split(addrs[0].String(), "/")
	splitAddr := strings.Split(ip[0], ".")
	if len(splitAddr) != 4 {
		panic("something went wrong with Ip address")
	}

	broadcastIp := splitAddr[0] + "." + splitAddr[1] + "." + splitAddr[2] + ".255"
	return net.ParseIP(ip[0]), ief.HardwareAddr, net.ParseIP(broadcastIp)

}
