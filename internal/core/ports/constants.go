package ports

import "time"

const (
	BlockchainSubscriptionRetryDelay = 10 * time.Second // Delay before retrying subscription
	MaxConcurrentChecks              = 100              // Максимальное количество одновременных проверок подтверждений
)
