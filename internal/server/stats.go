package server

import (
	"log"
	"sync/atomic"
	"time"
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

	totalRequests := atomic.LoadInt64(&s.stats.TotalRequests)
	successfulRequests := atomic.LoadInt64(&s.stats.SuccessfulRequests)
	failedRequests := atomic.LoadInt64(&s.stats.FailedRequests)
	averageProcessingTime := s.stats.AverageProcessingTime.Load().(time.Duration)

	errorRate := float64(0)
	if totalRequests > 0 {
		errorRate = float64(failedRequests) / float64(totalRequests) * 100
	}

	stats := map[string]interface{}{
		"total_requests":          totalRequests,
		"successful_requests":     successfulRequests,
		"failed_requests":         failedRequests,
		"error_rate":              errorRate,
		"average_processing_time": averageProcessingTime.Seconds(),
		"active_processors":       len(s.activeProcessors),
		"in_use_processors":       activeCount,
		"idle_processors":         len(s.idleProcessors),
		"queue_length":            len(s.taskQueue),
		"total_usage":             totalUsage,
	}

	log.Printf("服务器统计: %+v", stats)
	return stats
}
