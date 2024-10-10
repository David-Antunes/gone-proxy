package proxy

import (
	"encoding/gob"
	"errors"
	"github.com/David-Antunes/gone-proxy/internal"
	"github.com/David-Antunes/gone-proxy/xdp"
	ebpf "github.com/David-Antunes/xdp"
	"github.com/vishvananda/netlink"
	"log"
	"net"
	"os"
	"time"
)

var proxyLog = log.New(os.Stdout, "PROXY INFO: ", log.Ltime)

type ProxySocket struct {
	bpf        ebpf.Program
	mac        string
	enc        *gob.Encoder
	dec        *gob.Decoder
	sock       *ebpf.Socket
	link       netlink.Link
	descs      []ebpf.Desc
	socketPath string
	listener   net.Listener
	conn       net.Conn
	queue      chan *xdp.Frame
}

func NewProxySocket(queue int, iface string) (*ProxySocket, error) {

	link, err := netlink.LinkByName(iface)

	if err != nil {
		return nil, err
	}

	socket, err := ebpf.NewSocket(link.Attrs().Index, queue, &xdp.DefaultSocketOptions)
	if err != nil {
		return nil, err
	}
	socket.Fill(socket.GetDescs(socket.NumFreeFillSlots(), true))
	descs := socket.GetDescs(socket.NumFreeTxSlots(), false)

	program, err := ebpf.NewProgram(queue + 1)

	if err != nil {
		return nil, err
	}

	if err = program.Attach(link.Attrs().Index); err != nil {
		return nil, err
	}

	if err = program.Register(queue, socket.FD()); err != nil {
		panic(err)
	}

	proxyLog.Println("New proxy socket in", iface)
	return &ProxySocket{
		bpf:        *program,
		mac:        "",
		enc:        nil,
		dec:        nil,
		sock:       socket,
		link:       link,
		descs:      descs,
		socketPath: "",
		listener:   nil,
		conn:       nil,
		queue:      make(chan *xdp.Frame, internal.QueueSize),
	}, nil
}

func (p *ProxySocket) getLinkName() string {
	return p.link.Attrs().Name
}

func (p *ProxySocket) Bootstrap() {
	p.FindMac()
	p.CreateSocket()
	for {
		conn, err := p.listener.Accept()
		p.conn = conn

		if err != nil {
			proxyLog.Println(err)
			return
		}
		proxyLog.Println(p.link.Attrs().Name+":", "connection accepted")
		p.enc = gob.NewEncoder(conn)
		p.dec = gob.NewDecoder(conn)
		go p.Transmit()
		p.Receive(-1)
	}
}

func (p *ProxySocket) CreateSocket() {
	if p.mac == "" {
		internal.ShutdownAndLog(errors.New("mac is nil"))
		return
	}
	p.socketPath = "/tmp/" + p.mac + ".sock"
	os.Remove(p.socketPath)

	var err error

	proxyLog.Println(p.link.Attrs().Name+":", p.socketPath)

	p.listener, err = net.Listen("unix", p.socketPath)
	if err != nil {
		internal.ShutdownAndLog(err)
		return
	}
}

func (p *ProxySocket) FindMac() string {

	var mac string
	numRx, _, err := p.sock.Poll(-1)

	if err != nil {
		p.Close()
		return ""
	}

	rxDescs := p.sock.Receive(numRx)
	if len(rxDescs) > 0 {
		for i := 0; i < len(rxDescs); i++ {
			framePointer := p.sock.GetFrame(rxDescs[i])
			macOrig := string(framePointer[6:12])
			mac = macOrig
		}
		p.sock.Fill(rxDescs)
	}
	p.mac = net.HardwareAddr(mac).String()
	proxyLog.Println(p.link.Attrs().Name+":", p.mac)
	return mac
}

func (p *ProxySocket) Receive(timeout int) {
	for {

		numRx, _, err := p.sock.Poll(timeout)

		if err != nil {
			p.Close()
		}

		if numRx == 0 {
			continue
		}

		rxDescs := p.sock.Receive(numRx)
		if len(rxDescs) > 0 {
			for i := 0; i < len(rxDescs); i++ {
				framePointer := p.sock.GetFrame(rxDescs[i])
				macDest := string(framePointer[0:6])
				macOrig := string(framePointer[6:12])
				frame := &xdp.Frame{FramePointer: framePointer[:int(rxDescs[i].Len)], FrameSize: int(rxDescs[i].Len), Time: time.Now(), MacOrigin: macOrig, MacDestination: macDest}
				err = p.enc.Encode(&frame)

				if err != nil {
					proxyLog.Println(p.link.Attrs().Name, err)
					return
				}
			}
			p.sock.Fill(rxDescs)
		}
	}
}

func (p *ProxySocket) Transmit() {
	go func() {
		for {
			var f *xdp.Frame
			err := p.dec.Decode(&f)
			if err != nil {
				proxyLog.Println(p.link.Attrs().Name, err)
				return
			}
			p.queue <- f
		}
	}()

	for {
		select {
		case frame := <-p.queue:
			batchSize := len(p.queue)
			frames := make([]*xdp.Frame, 0, batchSize+1)

			frames = append(frames, frame)

			for i := 1; i < batchSize; i++ {
				frames = append(frames, <-p.queue)
			}
			err := p.send(frames)
			if err != nil {
				p.Close()
				return
			}
		}
	}
}

func (p *ProxySocket) SendFrame(frame *xdp.Frame) {

	txDescs := p.getTransmitDescs(1)

	if len(txDescs) > 0 {
		outFrame := p.sock.GetFrame(txDescs[0])
		txDescs[0].Len = uint32(copy(outFrame, frame.FramePointer[:frame.FrameSize]))
		_, err := p.sock.Transmit(txDescs)
		if err != nil {
			proxyLog.Println(err)
			return
		}
	}
}

func (p *ProxySocket) send(frames []*xdp.Frame) error {

	txDescs := p.getTransmitDescs(len(frames))

	for i := 0; i < len(txDescs); i++ {
		outFrame := p.sock.GetFrame(txDescs[i])
		txDescs[i].Len = uint32(copy(outFrame, frames[i].FramePointer))
	}
	_, err := p.sock.Transmit(txDescs)
	if err != nil {
		proxyLog.Println(err)
		return err
	}
	return nil
}

func (p *ProxySocket) getTransmitDescs(number int) []ebpf.Desc {
	for len(p.descs) < number {
		p.descs = p.sock.GetDescs(p.sock.NumFreeTxSlots(), false)

		if len(p.descs) < number {
			_, _, err := p.sock.Poll(1)
			if err != nil {
				proxyLog.Println(p.link.Attrs().Name, err)
				p.Close()
				return p.descs
			}
		} else {
			continue
		}
	}
	descs := p.descs[:number]
	p.descs = p.descs[number:]
	return descs
}

func (p *ProxySocket) Close() {
	err := p.bpf.Detach(p.link.Attrs().Index)
	if err != nil {
		proxyLog.Println(p.link.Attrs().Name, err)
	}
	err = p.sock.Close()
	if err != nil {
		proxyLog.Println(p.link.Attrs().Name, err)
	}
	err = p.conn.Close()
	if err != nil {
		proxyLog.Println(p.link.Attrs().Name, err)
	}
	err = p.listener.Close()
	if err != nil {
		proxyLog.Println(p.link.Attrs().Name, err)
	}
	err = os.Remove(p.socketPath)
	if err != nil {
		proxyLog.Println(p.link.Attrs().Name, err)
	}
}
