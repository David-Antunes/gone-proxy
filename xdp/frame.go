package xdp

import "time"

type Frame struct {
	FramePointer   []byte    `json:"framePointer"`
	FrameSize      int       `json:"frameSize"`
	Time           time.Time `json:"time"`
	MacOrigin      string    `json:"macOrigin"`
	MacDestination string    `json:"macDestination"`
}

func NewFrame(framePointer []byte, frameSize int, time time.Time, macOrigin, macDestination string) *Frame {
	return &Frame{
		FramePointer:   framePointer,
		FrameSize:      frameSize,
		Time:           time,
		MacOrigin:      macOrigin,
		MacDestination: macDestination,
	}
}

func (frame *Frame) GetTime() time.Time {
	return frame.Time
}

func (frame *Frame) Frame() []byte {
	return frame.FramePointer
}

func (frame *Frame) GetFrameSize() int {
	return frame.FrameSize
}

func (frame *Frame) GetMacOrigin() string {
	return frame.MacOrigin
}
func (frame *Frame) GetMacDestination() string {
	return frame.MacDestination
}
