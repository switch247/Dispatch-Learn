package worker

import (
	"fmt"
	"time"

	"dispatchlearn/internal/usecase"
	"dispatchlearn/logging"
)

// ExpiryWorker runs a background ticker that expires stale orders and cancels unstarted accepted orders.
type ExpiryWorker struct {
	dispatchUC *usecase.DispatchUseCase
	interval   time.Duration
	stopCh     chan struct{}
}

func NewExpiryWorker(dispatchUC *usecase.DispatchUseCase, interval time.Duration) *ExpiryWorker {
	return &ExpiryWorker{
		dispatchUC: dispatchUC,
		interval:   interval,
		stopCh:     make(chan struct{}),
	}
}

func (w *ExpiryWorker) Start() {
	logging.Info("worker", "expiry", "Starting expiry worker with interval "+w.interval.String())
	go w.run()
}

func (w *ExpiryWorker) Stop() {
	close(w.stopCh)
	logging.Info("worker", "expiry", "Expiry worker stopped")
}

func (w *ExpiryWorker) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run once immediately on startup
	w.process()

	for {
		select {
		case <-ticker.C:
			w.process()
		case <-w.stopCh:
			return
		}
	}
}

func (w *ExpiryWorker) process() {
	expired, cancelled := w.dispatchUC.CancelExpiredOrders()

	if expired > 0 || cancelled > 0 {
		logging.Info("worker", "expiry",
			"Processed: expired="+itoa(expired)+", cancelled="+itoa(cancelled))
	}
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
