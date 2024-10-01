package spam

import (
	"time"

	"github.com/ava-labs/hypersdk/consts"
)

const (
	plotSamples    = 5000
	plotIdentities = 20
	plotBarWidth   = 10
	plotOverhead   = 20
	plotHeight     = 30

	pendingTargetMultiplier        = 10
	pendingExpiryBuffer            = 15 * consts.MillisecondsPerSecond
	successfulRunsToIncreaseTarget = 10
	failedRunsToDecreaseTarget     = 5

	issuerShutdownTimeout = 60 * time.Second
)