// Package reload provides hot reload functionality for PicoClaw.
package reload

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sipeed/picoclaw/pkg/logger"
)

// SignalHandler handles OS signals for triggering reloads.
type SignalHandler struct {
	sigChan  chan os.Signal
	reloadFn func()
	once     sync.Once
	done     chan struct{}
	wg       sync.WaitGroup
}

// SetupSignalHandler sets up a signal handler for SIGHUP.
// The reloadFn function will be called when SIGHUP is received.
func SetupSignalHandler(reloadFn func()) *SignalHandler {
	sh := &SignalHandler{
		sigChan:  make(chan os.Signal, 1),
		reloadFn: reloadFn,
		done:     make(chan struct{}),
	}

	// Notify for SIGHUP
	signal.Notify(sh.sigChan, syscall.SIGHUP)

	sh.wg.Add(1)
	go sh.handleSignals()

	logger.InfoC("reload", "Signal handler registered for SIGHUP")

	return sh
}

// handleSignals waits for signals and triggers reload.
func (sh *SignalHandler) handleSignals() {
	defer sh.wg.Done()

	for {
		select {
		case sig := <-sh.sigChan:
			logger.InfoC("reload", fmt.Sprintf("Received signal: %v", sig))
			if sig == syscall.SIGHUP {
				logger.InfoC("reload", "SIGHUP received, triggering reload")
				// Call reload function in a goroutine to avoid blocking signal handling
				go func() {
					defer func() {
						if r := recover(); r != nil {
							logger.ErrorC("reload", fmt.Sprintf("Panic in reload function: %v", r))
						}
					}()
					sh.reloadFn()
				}()
			}

		case <-sh.done:
			return
		}
	}
}

// Stop stops the signal handler.
func (sh *SignalHandler) Stop() {
	logger.InfoC("reload", "Stopping signal handler")

	// Unregister signal handler
	signal.Stop(sh.sigChan)

	// Close done channel to signal goroutine to stop
	close(sh.done)

	// Wait for goroutine to finish
	sh.wg.Wait()

	// Close signal channel
	close(sh.sigChan)
}
