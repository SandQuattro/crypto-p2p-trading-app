package shared

import (
	"os"
	"strings"
)

const EnvBlockchainDebugMode = "BLOCKCHAIN_DEBUG_MODE"

// IsBlockchainDebugMode checks if blockchain debug mode is enabled via environment variable
func IsBlockchainDebugMode() bool {
	debugMode := os.Getenv(EnvBlockchainDebugMode)
	return strings.ToLower(debugMode) == "true" || strings.ToLower(debugMode) == "1"
}
