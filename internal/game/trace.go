package game

// TraceTimelineStep represents an event in the request's journey.
type TraceTimelineStep struct {
	Step     string  `json:"step"`
	TimeMs   float64 `json:"time_ms"`
	Title    string  `json:"title"`
	Desc     string  `json:"desc"`
	Category string  `json:"category"` // "ingress", "balancer", "mutation", "upstream", "egress"
	Details  any     `json:"details,omitempty"`
}

// RequestTrace contains the microscopic forensic log of a proxied request.
type RequestTrace struct {
	ID               string              `json:"id"`
	Method           string              `json:"method"`
	Path             string              `json:"path"`
	ClientIP         string              `json:"client_ip"`
	SelectedBackend  string              `json:"selected_backend"`
	SelectedMockID   string              `json:"selected_mock_id,omitempty"`
	SelectedURL      string              `json:"selected_url"`
	BalancerStrategy string              `json:"balancer_strategy"`
	StatusCode       int                 `json:"status_code"`
	TotalDurationMs  float64             `json:"total_duration_ms"`
	Timeline         []TraceTimelineStep `json:"timeline"`
	HeadersSent      map[string][]string `json:"headers_sent"`
	HeadersReceived  map[string][]string `json:"headers_received"`
	HeadersAdded     map[string]string   `json:"headers_added"`
	HeadersStripped  []string            `json:"headers_stripped"`
	ResponseBody     any                 `json:"response_body,omitempty"`
	ErrorMessage     string              `json:"error_message,omitempty"`
}
