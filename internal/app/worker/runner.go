// Package worker owns the lifecycle of Memora background runners.
package worker

import "context"

// Runner is a long-lived background task that stops when its context is cancelled.
type Runner interface {
	Run(context.Context) error
}
