package xdp

import (
	"fmt"
	"time"

	"github.com/David-Antunes/xdp"
	"github.com/google/uuid"
	"github.com/vishvananda/netlink"
)

type XdpSock struct {
	id    uuid.UUID
	sock  xdp.Socket
	link  netlink.Link
	descs []xdp.Desc
}

func (socket *XdpSock) ID() string {
	return socket.id.String()
}
func CreateCustomXdpSock(queue int, ifname string, opts *xdp.SocketOptions) (*XdpSock, error) {

	link, err := netlink.LinkByName(ifname)

	if err != nil {
		return nil, err
	}

	xsk, err := xdp.NewSocket(link.Attrs().Index, queue, opts)
	if err != nil {
		return nil, err
	}
	xsk.Fill(xsk.GetDescs(xsk.NumFreeFillSlots(), true))
	descs := xsk.GetDescs(xsk.NumFreeTxSlots(), false)

	return &XdpSock{uuid.New(), *xsk, link, descs}, nil
}

func CreateXdpSock(queue int, ifname string) (*XdpSock, error) {
	return CreateCustomXdpSock(queue, ifname, &xdp.DefaultSocketOptions)
}

func (socket *XdpSock) Receive(timeout int) ([]*Frame, error) {
	numRx, _, err := socket.sock.Poll(timeout)

	if err != nil {
		return nil, err
	}

	if numRx == 0 {
		return []*Frame{}, nil
	}

	rxDescs := socket.sock.Receive(numRx)
	if len(rxDescs) > 0 {
		frames := make([]*Frame, 0, len(rxDescs))
		for i := 0; i < len(rxDescs); i++ {
			framePointer := socket.sock.GetFrame(rxDescs[i])
			macDest := string(framePointer[0:6])
			macOrig := string(framePointer[6:12])
			buf := make([]byte, int(rxDescs[i].Len))
			copy(buf, framePointer)
			frame := &Frame{buf, int(rxDescs[i].Len), time.Now(), macOrig, macDest}
			frames = append(frames, frame)
		}
		socket.sock.Fill(rxDescs)
		return frames, nil
	}
	return []*Frame{}, nil
}

func (socket *XdpSock) SendFrame(frame *Frame) {

	txDescs := socket.getTransmitDescs(1)

	if len(txDescs) > 0 {
		outFrame := socket.sock.GetFrame(txDescs[0])
		txDescs[0].Len = uint32(copy(outFrame, frame.FramePointer[:frame.FrameSize]))
		_, err := socket.sock.Transmit(txDescs)
		if err != nil {
			fmt.Println(err)
			return
		}

	}
}

func (socket *XdpSock) Send(frames []*Frame) {

	txDescs := socket.getTransmitDescs(len(frames))

	for i := 0; i < len(txDescs); i++ {
		outFrame := socket.sock.GetFrame(txDescs[i])
		txDescs[i].Len = uint32(copy(outFrame, frames[i].FramePointer))
	}
	_, err := socket.sock.Transmit(txDescs)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func (socket *XdpSock) getTransmitDescs(number int) []xdp.Desc {
	for len(socket.descs) < number {
		socket.descs = socket.sock.GetDescs(socket.sock.NumFreeTxSlots(), false)

		if len(socket.descs) < number {
			_, _, err := socket.sock.Poll(1)
			if err != nil {
				socket.Close()
				return socket.descs
			}
		} else {
			continue
		}
	}
	descs := socket.descs[:number]
	socket.descs = socket.descs[number:]
	return descs

}

func (socket *XdpSock) Close() {
	err := socket.sock.Close()
	if err != nil {
		fmt.Println(err)
	}
}
