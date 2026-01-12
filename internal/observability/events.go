package observability

type EventEnvelope struct {
	EventType string      `json:"event_type"`
	EventName string      `json:"event_name"`
	Payload   interface{} `json:"payload"`
}

func BuildHeaders(requestID, traceID string) map[string]string {
	headers := map[string]string{}
	if requestID != "" {
		headers["x-request-id"] = requestID
	}
	if traceID != "" {
		headers["trace_id"] = traceID
	}
	return headers
}
