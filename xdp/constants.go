package xdp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/David-Antunes/xdp"
)

const xdpFrameSize = 2048

var DefaultSocketOptions = xdp.SocketOptions{
	NumFrames:              16384,
	FrameSize:              xdpFrameSize,
	FillRingNumDescs:       8192,
	CompletionRingNumDescs: 8192,
	RxRingNumDescs:         8192,
	TxRingNumDescs:         8192,
}

func ConvertMacStringToBytes(macAddr string) []byte {
	parts := strings.Split(macAddr, ":")
	var macBytes []byte

	for _, part := range parts {
		b, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			fmt.Printf("Error parsing %s: %v\n", part, err)
			panic(err)
		}
		macBytes = append(macBytes, byte(b))

	}
	return macBytes
}
