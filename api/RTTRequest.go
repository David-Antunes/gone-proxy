package api

import "time"

type RTTRequest struct {
	StartTime    time.Time `json:"StartTime"`
	ReceiveTime  time.Time `json:"ReceiveTime"`
	TransmitTime time.Time `json:"TransmitTime"`
	EndTime      time.Time `json:"EndTime"`
}
