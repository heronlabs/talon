package manifest

// Manifest is the answer of the manifest endpoint: the host and the clients it runs, sorted by slug.
type Manifest struct {
	Data struct {
		Host    string   `json:"host"`
		Clients []Client `json:"clients"`
	} `json:"data"`
}
