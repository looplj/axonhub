package objects

type ErrorResponse struct {
	Error Error `json:"error"`
}

type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
