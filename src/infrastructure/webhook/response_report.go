package webhook

// ReportResponse holds the reply of the webhook to a report: the host and measuredAt it received, the outcome, and the projects it matched and those it did not.
type ReportResponse struct {
	Data struct {
		Host       string   `json:"host"`
		MeasuredAt string   `json:"measuredAt"`
		Outcome    string   `json:"outcome"`
		Matched    []string `json:"matched"`
		Unmatched  []string `json:"unmatched"`
	} `json:"data"`
}
