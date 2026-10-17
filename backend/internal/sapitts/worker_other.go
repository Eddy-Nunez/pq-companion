//go:build !windows

package sapitts

import "context"

type worker struct{}

func startWorker() (*worker, error) { return nil, ErrUnsupported }

func (w *worker) dead() bool { return true }

func (w *worker) stop() {}

func (w *worker) synthesize(context.Context, request) error { return ErrUnsupported }
