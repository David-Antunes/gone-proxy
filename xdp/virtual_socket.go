package xdp

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type VirtSock struct {
	sync.Mutex
	uuid           uuid.UUID
	mac            string
	running        bool
	queue          chan *Frame
	endpoints      []string
	ReceivedFrames []*Frame
}

func CreateVirtSocket(mac string) *VirtSock {
	return &VirtSock{sync.Mutex{}, uuid.New(), mac, false, make(chan *Frame, 1), make([]string, 0), make([]*Frame, 0)}
}

// Start This is broken in case of the socket closing, and it will still try to send packets
func (sock *VirtSock) Start() {
	if !sock.running {
		sock.running = true

		go func() {
			for sock.running {
				sock.Lock()
				auxEndpoints := make([]string, 0, len(sock.endpoints))
				copy(sock.endpoints, auxEndpoints)
				sock.Unlock()
				for _, endpoint := range auxEndpoints {
					time.Sleep(time.Second)
					frame := &Frame{nil, 0, time.Now(), sock.mac, endpoint}
					sock.queue <- frame
				}
			}
		}()
	}
}

func (sock *VirtSock) InjectFrame(destMac string) {
	sock.queue <- &Frame{nil, 0, time.Now(), sock.mac, destMac}
}

func (sock *VirtSock) ID() string {
	return sock.uuid.String()
}

func (sock *VirtSock) SendFrame(frame *Frame) {
	sock.Lock()
	sock.ReceivedFrames = append(sock.ReceivedFrames, frame)
	sock.Unlock()
}

func (sock *VirtSock) Send(frames []*Frame) {
	sock.Lock()
	sock.ReceivedFrames = append(sock.ReceivedFrames, frames...)
	sock.Unlock()
}

func (sock *VirtSock) Receive() []*Frame {
	frames := make([]*Frame, 0, 1)
	frames = append(frames, <-sock.queue)
	return frames
}

func (sock *VirtSock) Close() {
	sock.Lock()
	sock.running = false
	sock.Unlock()
}

func (sock *VirtSock) CleanFrameMem(frames []*Frame) {

}
