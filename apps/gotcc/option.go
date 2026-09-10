package gotcc

import "time"

// Options holds the tunables of a TXManager.
type Options struct {
	// Timeout limits the duration of one transaction. A transaction that is
	// still hanging after Timeout is rolled back by the monitor task.
	Timeout time.Duration
	// MonitorTick is the interval between two runs of the monitor task that
	// advances hanging transactions.
	MonitorTick time.Duration
}

// Option mutates Options.
type Option func(*Options)

// WithTimeout sets the transaction timeout. Non-positive values fall back
// to the 5s default.
func WithTimeout(timeout time.Duration) Option {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return func(o *Options) {
		o.Timeout = timeout
	}
}

// WithMonitorTick sets the monitor task interval. Non-positive values fall
// back to the 10s default.
func WithMonitorTick(tick time.Duration) Option {
	if tick <= 0 {
		tick = 10 * time.Second
	}

	return func(o *Options) {
		o.MonitorTick = tick
	}
}

// repair applies the defaults to any option left unset, e.g. when the
// TXManager is created without options.
func repair(o *Options) {
	if o.MonitorTick <= 0 {
		o.MonitorTick = 10 * time.Second
	}

	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Second
	}
}
