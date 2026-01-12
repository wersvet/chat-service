package ws

import "time"

type ConnInfo struct {
	ConnID      string
	UserID      int
	DeviceID    string
	IP          string
	RequestID   string
	TraceID     string
	ConnectedAt time.Time
}
