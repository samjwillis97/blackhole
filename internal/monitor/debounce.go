package monitor

import (
	"sync"
	"time"
)

type DebounceEvent int

const (
	CreateOrWrite DebounceEvent = iota
	Unknown
)

type debounceEntry struct {
	timers map[DebounceEvent]*time.Timer
	mu     sync.Mutex
}

var debounceTimers sync.Map // A concurrent map to track timers for each file

func Debounce(key string, event DebounceEvent, fn func()) {
	const debounceDuration = 5 * time.Second
	entry, _ := debounceTimers.LoadOrStore(key, &debounceEntry{
		timers: make(map[DebounceEvent]*time.Timer),
	})

	debounce := entry.(*debounceEntry)
	debounce.mu.Lock()
	defer debounce.mu.Unlock()

	// Reset all the timers for this file
	for _, timer := range debounce.timers {
		// Stop and reset the timer
		if !timer.Stop() {
			<-timer.C // Drain channel to prevent leaks
		}
		timer.Reset(debounceDuration)
	}

	// eventType := sonarrEventFromFileEvent(e)

	// Create a new timer for this event type, if doesn't exist
	if _, exists := debounce.timers[event]; !exists {
		timer := time.AfterFunc(debounceDuration, func() {
			// handleEvent(eventType, e.Name)
			fn()

			// Clean up the timer after execution
			debounce.mu.Lock()
			delete(debounce.timers, event)
			if len(debounce.timers) == 0 {
				// If no timers remain, remove the file entry entirely
				debounceTimers.Delete(key)
			}
			debounce.mu.Unlock()
		})
		debounce.timers[event] = timer
	}
}
