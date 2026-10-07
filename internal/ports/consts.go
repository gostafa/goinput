package ports

import "time"

// DiscoveryBudget bounds the entire discovery phase, including metadata retries.
// Capture startup and active stream lifetimes use their caller/owner contexts.
const DiscoveryBudget = 500 * time.Millisecond
