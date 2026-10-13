package easypanel

// User is the answer of getUser: the user of the token, of which talon reads only the email; Easypanel answers null for a token it refuses.
type User struct {
	Email string `json:"email"`
}
