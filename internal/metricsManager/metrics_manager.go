package metricsManager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/David-Antunes/gone-proxy/api"
	"github.com/David-Antunes/gone-proxy/internal"
	"github.com/David-Antunes/gone-proxy/xdp"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Magic number to align json
var magicNumber = 34

type MetricsManager struct {
	iface           xdp.Isocket
	mac             net.HardwareAddr
	ip              net.IP
	port            int
	fd              int
	addr            *syscall.SockaddrLinklayer
	receiveLatency  time.Duration
	transmitLatency time.Duration
	tests           []api.RTTRequest
	endpoint        *RttConnection
	metricsSocket   *MetricsSocket
	timeout         time.Duration
	numTests        int
}

var metricsLog = log.New(os.Stdout, "METRICS INFO: ", log.Ltime)

func NewMetricsManager(iface xdp.Isocket, mac net.HardwareAddr, ip net.IP, port int, endpoint *RttConnection, socketPath string, timeout time.Duration, numTests int) *MetricsManager {

	//fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_IP)))
	//if err != nil {
	//	internal.ShutdownAndLog(err)
	//	return nil
	//}
	//
	//ifi, err := net.InterfaceByName("br0")
	//if err != nil {
	//	internal.ShutdownAndLog(err)
	//	return nil
	//}
	//addr := &syscall.SockaddrLinklayer{
	//	Protocol: htons(syscall.ETH_P_IP),
	//	Ifindex:  ifi.Index,
	//}

	metricsSocket, err := NewMetricsSocket(socketPath)
	if err != nil {
		internal.ShutdownAndLog(err)
	}

	go metricsSocket.StartSocket()

	return &MetricsManager{
		iface: iface,
		mac:   mac,
		ip:    ip,
		port:  port,
		//fd:              fd,
		//addr:            addr,
		receiveLatency:  0,
		transmitLatency: 0,
		tests:           make([]api.RTTRequest, 5),
		endpoint:        endpoint,
		metricsSocket:   metricsSocket,
		timeout:         timeout,
		numTests:        numTests,
	}
}

func htons(i uint16) uint16 {
	return (i<<8)&0xff00 | i>>8
}

func (manager *MetricsManager) Close() {
	manager.iface.Close()
	manager.metricsSocket.Close()
	err := syscall.Close(manager.fd)
	if err != nil {
		fmt.Println(err)
	}
	metricsLog.Println("Closed")
}

func (manager *MetricsManager) Start() {

	metricsLog.Println("MAC:", manager.mac)
	metricsLog.Println("IP:", manager.ip)
	metricsLog.Println("PORT:", manager.port)
	metricsLog.Println("RTT:", manager.endpoint.IP, manager.endpoint.Port)
	metricsLog.Println("Metrics Socket:", manager.metricsSocket.socketPath)
	metricsLog.Println("Update Frequency:", manager.timeout)
	metricsLog.Println("Number of Tests:", manager.numTests)

	var frames []*xdp.Frame
	var err error
	time.Sleep(5 * time.Second)
	for {
		manager.tests = make([]api.RTTRequest, 0, manager.numTests)
		obs := time.Duration(math.MaxInt)
		for i := 0; i < manager.numTests; i++ {
			_, err = manager.sendTest()
			//metricsLog.Println("Sending Frame")
			if err != nil {
				metricsLog.Println(err)
				break
			}
			for {
				frames, err = manager.iface.Receive(-1)
				if len(frames) == 0 {
					//metricsLog.Println("No frames received")
					continue
				} else {
					break
				}
			}
			req := &api.RTTRequest{}
			for _, frame := range frames {
				err = receive(frame.FramePointer[:frame.FrameSize], req)
				if err != nil {
					metricsLog.Println(err)
					continue
				}
				req.EndTime = time.Now()
				obs = time.Duration(math.Min(float64(obs), math.Min(float64(req.ReceiveTime.Sub(req.StartTime)), float64(req.EndTime.Sub(req.TransmitTime)))))
			}
		}
		manager.metricsSocket.sendRTT(obs, obs)
		if manager.timeout > 0 {
			time.Sleep(manager.timeout)
		} else {
			return
		}
	}
}

//func (manager *MetricsManager) calculateAvg() {
//	var accReceive time.Duration
//	var accTransmit time.Duration
//
//	for _, test := range manager.tests {
//		accReceive += test.ReceiveTime.Sub(test.StartTime)
//		accTransmit += test.EndTime.Sub(test.TransmitTime)
//	}
//
//	manager.receiveLatency = accReceive / time.Duration(len(manager.tests))
//	manager.transmitLatency = accTransmit / time.Duration(len(manager.tests))
//
//	metricsLog.Println("receiveLatency:", manager.receiveLatency)
//	metricsLog.Println("transmitLatency:", manager.transmitLatency)
//}

func (manager *MetricsManager) sendTest() (api.RTTRequest, error) {
	req, err := json.Marshal(&api.RTTRequest{
		StartTime:    time.Now(),
		ReceiveTime:  time.Time{},
		TransmitTime: time.Time{},
		EndTime:      time.Time{},
	})
	if err != nil {
		fmt.Println(req)
		return api.RTTRequest{}, err
	}
	buf := gopacket.NewSerializeBuffer()
	var layersToSerialize []gopacket.SerializableLayer

	ethLayer := &layers.Ethernet{
		SrcMAC:       manager.mac,
		DstMAC:       manager.endpoint.Mac,
		EthernetType: layers.EthernetTypeIPv4,
	}
	layersToSerialize = append(layersToSerialize, ethLayer)

	ipLayer := &layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    manager.ip,
		DstIP:    manager.endpoint.IP,
		Protocol: layers.IPProtocolUDP,
	}
	layersToSerialize = append(layersToSerialize, ipLayer)
	udpLayer := &layers.UDP{
		SrcPort: layers.UDPPort(manager.port),
		DstPort: layers.UDPPort(manager.endpoint.Port),
	}
	udpLayer.SetNetworkLayerForChecksum(ipLayer)
	layersToSerialize = append(layersToSerialize, udpLayer)

	layersToSerialize = append(layersToSerialize, gopacket.Payload(req))

	if err = gopacket.SerializeLayers(buf, gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}, layersToSerialize...); err != nil {
		internal.ShutdownAndLog(err)
		return api.RTTRequest{}, err
	}
	manager.iface.SendFrame(&xdp.Frame{
		FramePointer:   buf.Bytes(),
		FrameSize:      len(buf.Bytes()),
		Time:           time.Time{},
		MacOrigin:      "",
		MacDestination: "",
	})
	//if err = syscall.Sendto(manager.fd, buf.Bytes(), 0, manager.addr); err != nil {
	//	internal.ShutdownAndLog(err)
	//	return api.RTTRequest{}, err
	//}
	return api.RTTRequest{}, nil
}

func receive(payload []byte, request *api.RTTRequest) error {
	packet := gopacket.NewPacket(payload, layers.LayerTypeUDP, gopacket.Default)

	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp := udpLayer.(*layers.UDP)

		d := json.NewDecoder(bytes.NewReader(udp.Payload[magicNumber:]))
		err := d.Decode(request)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("received tcp packet")
	}
}

func (manager *MetricsManager) Publish() {
	manager.metricsSocket.sendRTT(manager.receiveLatency, manager.transmitLatency)
}
