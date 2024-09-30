package metricsManager

import (
	"encoding/gob"
	"fmt"
	"time"

	"github.com/David-Antunes/gone-proxy/api"

	"net"
	"os"
)

type MetricsSocket struct {
	socketPath string
	sock       net.Listener
	conn       []net.Conn
	encs       []*gob.Encoder
	closed     bool
}

func NewMetricsSocket(socketPath string) (*MetricsSocket, error) {
	listen, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &MetricsSocket{
		socketPath: socketPath,
		sock:       listen,
		conn:       make([]net.Conn, 0),
		encs:       make([]*gob.Encoder, 0),
		closed:     false,
	}, nil
}

func (s *MetricsSocket) StartSocket() error {

	for {
		conn, err := s.sock.Accept()
		if err != nil {
			fmt.Println(err)
			return nil
		}
		s.conn = append(s.conn, conn)
		s.encs = append(s.encs, gob.NewEncoder(conn))
	}
}

func (s *MetricsSocket) sendRTT(receiveLatency time.Duration, transmitLatency time.Duration) {
	currentConnections := make([]net.Conn, 0)
	currEncs := make([]*gob.Encoder, 0)
	for i, conn := range s.conn {

		err := s.encs[i].Encode(&api.UpdateRTTRequest{
			ReceiveLatency:  receiveLatency,
			TransmitLatency: transmitLatency,
		})

		if err == nil {
			currentConnections = append(currentConnections, conn)
			currEncs = append(currEncs, s.encs[i])
		} else {
			conn.Close()
			fmt.Println(err)
		}
	}
	s.conn = currentConnections
	s.encs = currEncs
	metricsLog.Println("Published results")
}

func (s *MetricsSocket) Close() {

	for _, conn := range s.conn {
		err := conn.Close()
		if err != nil {
			continue
		}
	}
	err := os.Remove(s.socketPath)
	if err != nil {
		return
	}
}
