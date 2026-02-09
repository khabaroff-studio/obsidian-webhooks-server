package models

// KeyType represents the type of API key
type KeyType string

const (
	// KeyTypeWebhook identifies webhook keys used by external services
	KeyTypeWebhook KeyType = "webhook"
	// KeyTypeClient identifies client keys used by Obsidian plugin
	KeyTypeClient KeyType = "client"
)

// KeyStatus represents the activation status of an API key
type KeyStatus string

const (
	// KeyStatusActive indicates the key is active and can be used
	KeyStatusActive KeyStatus = "active"
	// KeyStatusInactive indicates the key is deactivated
	KeyStatusInactive KeyStatus = "inactive"
)

// Deprecated table names (backward compatibility views)
const (
	// TableWebhookKeys is the deprecated webhook_keys view name
	TableWebhookKeys = "webhook_keys"
	// TableClientKeys is the deprecated client_keys view name
	TableClientKeys = "client_keys"
)
