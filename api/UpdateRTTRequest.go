package api

import "time"

type UpdateRTTRequest struct {
	ReceiveLatency  time.Duration
	TransmitLatency time.Duration
}
