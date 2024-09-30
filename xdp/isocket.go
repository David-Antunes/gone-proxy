package xdp

type Isocket interface {
	ID() string
	SendFrame(*Frame)
	Send([]*Frame)
	Receive(int) ([]*Frame, error)
	Close()
}
