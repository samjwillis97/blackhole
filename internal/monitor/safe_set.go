package monitor

import (
	"github.com/samjwillis97/sams-blackhole/internal/arr"
	"log"
	"sync"
	"time"
)

type PathMeta struct {
	OriginalFileName string
	Expiration       time.Time
	CompletedDir     string
	Service          arr.ArrService
}

type PathSet map[string]PathMeta

type Monitors struct {
	mu  sync.Mutex
	set PathSet
}

var (
	instance *Monitors
	once     sync.Once
)

func GetMonitoredFile(name string) PathMeta {
	pathSet := getInstance()
	return pathSet.get(name)
}

// GetInstance ensures only one instance of SafeSet exists
func getInstance() *Monitors {
	once.Do(func() {
		instance = &Monitors{
			set: make(PathSet),
		}
	})

	instance.cleanupExpiredItems()

	return instance
}

// Add inserts an element into the set
func (s *Monitors) add(item string, meta PathMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set[item] = meta
}

// Exists checks if an element is in the set
func (s *Monitors) exists(item string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.set[item]
	return exists
}

// Gets item from set, no need to lock
func (s *Monitors) get(item string) PathMeta {
	return s.set[item]
}

// Remove deletes an element from the set
func (s *Monitors) remove(item string) PathMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	toReturn := s.get(item)
	delete(s.set, item)
	return toReturn
}

// Items returns all elements in the set
func (s *Monitors) items() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]string, 0, len(s.set))
	for k := range s.set {
		items = append(items, k)
	}
	return items
}

func (s *Monitors) cleanupExpiredItems() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for k, meta := range s.set {
		if now.After(meta.Expiration) {
			log.Printf("Removing %s from monitoring\n", k)
			delete(s.set, k)
		}
	}
}
