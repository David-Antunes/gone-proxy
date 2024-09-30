package xdp

import (
	"fmt"
	"github.com/David-Antunes/xdp"
)

//var DefaultXdpFlags = int(unix.XDP_FLAGS_SKB_MODE)

// XdpBpfSock XDP socket with ebpf program
type XdpBpfSock struct {
	xdp *XdpSock
	bpf xdp.Program
}

func CreateXdpBpfSock(queue int, ifname string) (*XdpBpfSock, error) {

	socket, err := CreateXdpSock(queue, ifname)
	if err != nil {
		return nil, err
	}
	program, err := xdp.NewProgram(queue + 1)

	if err != nil {
		return nil, err
	}

	if err = program.Attach(socket.link.Attrs().Index); err != nil {
		return nil, err
	}

	if err = program.Register(queue, socket.sock.FD()); err != nil {
		panic(err)
	}
	socket.sock.Fill(socket.sock.GetDescs(socket.sock.NumFreeFillSlots(), true))
	return &XdpBpfSock{socket, *program}, nil

}

func (sock *XdpBpfSock) ID() string {
	return sock.xdp.ID()
}

func (sock *XdpBpfSock) SendFrame(frame *Frame) {
	sock.xdp.SendFrame(frame)
}

func (sock *XdpBpfSock) Send(frames []*Frame) {
	sock.xdp.Send(frames)
}

func (sock *XdpBpfSock) Receive(timeout int) ([]*Frame, error) {
	return sock.xdp.Receive(timeout)
}

func (sock *XdpBpfSock) Close() {
	err := sock.bpf.Detach(sock.xdp.link.Attrs().Index)
	if err != nil {
		fmt.Println(err)
	}
	err = sock.xdp.sock.Close()
	if err != nil {
		fmt.Println(err)
	}
}
