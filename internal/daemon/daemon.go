package daemon

import (
	"errors"
	"fmt"
	"github.com/David-Antunes/gone-proxy/internal/proxy"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/David-Antunes/gone-proxy/internal"
)

var serverLog = log.New(os.Stdout, "SERVER INFO: ", log.Ltime)

type Daemon struct {
	sync.Mutex
	unixPath     string
	httpServer   *http.Server
	socket       net.Listener
	interfaces   map[string]struct{}
	proxySockets map[string]*proxy.ProxySocket
}

func NewDaemon(unixPath string) *Daemon {

	s, err := net.Listen("unix", unixPath)
	if err != nil {
		internal.ShutdownAndLog(err)
		return nil
	}

	interfaces := make(map[string]struct{})

	// Ignore Interfaces
	interfaces["veth0"] = struct{}{}
	interfaces["veth1"] = struct{}{}
	interfaces["vxlan0"] = struct{}{}
	interfaces["br0"] = struct{}{}
	interfaces["lo"] = struct{}{}

	d := &Daemon{
		Mutex:        sync.Mutex{},
		unixPath:     unixPath,
		httpServer:   nil,
		socket:       nil,
		interfaces:   interfaces,
		proxySockets: make(map[string]*proxy.ProxySocket),
	}

	m := http.NewServeMux()
	m.HandleFunc("/refresh", d.SearchInterfaces)

	httpServer := http.Server{
		Handler: m,
	}

	d.httpServer = &httpServer
	d.socket = s
	return d

}
func (d *Daemon) Serve() {

	serverLog.Println("socketPath:", d.unixPath)
	serverLog.Println("Serving...")
	if err := d.httpServer.Serve(d.socket); err != nil {
		internal.ShutdownAndLog(err)
	}
}

func (d *Daemon) SearchInterfaces(w http.ResponseWriter, r *http.Request) {
	serverLog.Println("Searching for network interfaces")
	d.Lock()
	ifaces, err := net.Interfaces()

	if err != nil {
		internal.ShutdownAndLog(err)
		d.Unlock()
		return
	}

	if len(ifaces) == 1 {
		internal.ShutdownAndLog(errors.New("something went wrong with the network"))
		d.Unlock()
		return
	}

	newIfaces := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if _, ok := d.interfaces[iface.Name]; !ok {
			newIfaces = append(newIfaces, iface.Name)
			d.interfaces[iface.Name] = struct{}{}
		}
	}

	for _, iface := range newIfaces {
		serverLog.Println("Found interface:", iface)
		s, err := proxy.NewProxySocket(0, iface)
		if err != nil {
			internal.ShutdownAndLog(err)
			d.Unlock()
			return
		}
		d.proxySockets[iface] = s
		go s.Bootstrap()
	}
	d.Unlock()
}

func (d *Daemon) Close() {
	fmt.Println("yooo")
	for _, s := range d.proxySockets {
		s.Close()
	}
	err := d.socket.Close()
	if err != nil {
		serverLog.Println(err)

	}
	err = d.httpServer.Close()
	if err != nil {
		serverLog.Println(err)
	}
	err = os.Remove(d.unixPath)
	if err != nil {
		serverLog.Println(err)
	}
	serverLog.Println("Closed")
}
