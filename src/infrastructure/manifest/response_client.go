package manifest

// Client is a client of the manifest: its slug, the name of its Easypanel project, its lifecycle, the role of its secrets, null when empty, and the Easypanel schema of its services.
type Client struct {
	Slug           string  `json:"slug"`
	Lifecycle      string  `json:"lifecycle"`
	SecretsRoleArn *string `json:"secretsRoleArn"`
	Schema         struct {
		Services []Service `json:"services"`
	} `json:"schema"`
}
