package easypanel

// Domain is an item of the answer of listDomains, decoded as JSON values: its host, whether it is served over HTTPS, its path, certificate resolver, middlewares and wildcard, and the port and protocol of the service it routes to.
type Domain struct {
	Host                any `json:"host"`
	HTTPS               any `json:"https"`
	Path                any `json:"path"`
	CertificateResolver any `json:"certificateResolver"`
	Middlewares         any `json:"middlewares"`
	Wildcard            any `json:"wildcard"`
	ServiceDestination  struct {
		Port     any `json:"port"`
		Protocol any `json:"protocol"`
	} `json:"serviceDestination"`
}
