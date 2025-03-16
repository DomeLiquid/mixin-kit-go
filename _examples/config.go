package _examples

type Config struct {
	// AppID is equivalent to the ClientID
	AppID string `json:"app_id"`

	SessionID string `json:"session_id"`
	// ServerPublicKey is equivalent to the PinToken in hex format
	ServerPublicKey string `json:"server_public_key"`
	// SessionPrivateKey is equivalent to the PrivateKey in hex format
	SessionPrivateKey string `json:"session_private_key"`

	SpendKey string `json:"spend_key"`
}
