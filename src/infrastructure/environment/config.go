package environment

type Key struct {
	Name    string
	Default string
}

func (k Key) Required() bool {
	return k.Default == ""
}

func Keys() []Key {
	return []Key{
		{Name: "CMS_URL"},
		{Name: "CMS_AGENT_TOKEN"},
		{Name: "CMS_AGENT_HOST_NAME"},
		{Name: "EASYPANEL_URL", Default: "http://127.0.0.1:3000"},
		{Name: "EASYPANEL_TOKEN"},
		{Name: "AWS_ACCESS_KEY_ID"},
		{Name: "AWS_SECRET_ACCESS_KEY"},
		{Name: "AWS_REGION", Default: "us-east-1"},
		{Name: "INTERVAL_RECONCILE", Default: "5m"},
		{Name: "INTERVAL_REPORT", Default: "10m"},
	}
}
