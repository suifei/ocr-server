package server

import (
	"log"
	"sync/atomic"
)

func (s *Server) GetStats() map[string]interface{} {
	s.poolLock.Lock()
	defer s.poolLock.Unlock()

	activeCount := 0
	totalUsage := int64(0)
	for _, p := range s.activeProcessors {
		if p.inUse {
			activeCount++
		}
		totalUsage += atomic.LoadInt64(&p.usageCount)
	}

	stats := map[string]interface{}{
		"active_processors": len(s.activeProcessors),
		"in_use_processors": activeCount,
		"idle_processors":   len(s.idleProcessors),
		"queue_length":      len(s.taskQueue),
		"total_usage":       totalUsage,
	}

	log.Printf("Server stats: %+v", stats)
	return stats
}