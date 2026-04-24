package codes

// URN is a string type for error code constants
type URN string

const (
	// ErrorsForbiddenResourceQuotaExceeded indicates the tenant has exceeded their resource quota for the requested operation.
	ErrorsForbiddenResourceQuotaExceeded URN = "err:user:forbidden:resource_quota_exceeded"
)
