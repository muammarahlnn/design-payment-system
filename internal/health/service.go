package health

import (
	"context"
	"log"
)

const (
	statusOK    = "ok"
	statusError = "error"
)

// Report is the outcome of a health check. Database and Kafka stay empty for a
// service that does not depend on them.
type Report struct {
	Status   string
	Database string
	Kafka    string
}

// Healthy is true only when every dependency the service actually uses is
// reachable. A dependency that was never wired in is not a failure.
func (r Report) Healthy() bool {
	return r.Database != statusError && r.Kafka != statusError
}

// Service holds the health-check business logic.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Check pings every wired dependency and reports the result. A failed
// dependency is a normal, reportable outcome, so this never returns an error.
func (s *Service) Check(ctx context.Context) Report {
	report := Report{Status: statusOK}

	if s.repo.HasDatabase() {
		report.Database = statusOK
		if err := s.repo.PingDatabase(ctx); err != nil {
			report.Database = statusError
			log.Printf("health: database ping failed: %v", err)
		}
	}

	if s.repo.HasKafka() {
		report.Kafka = statusOK
		if err := s.repo.PingKafka(ctx); err != nil {
			report.Kafka = statusError
			log.Printf("health: kafka ping failed: %v", err)
		}
	}

	return report
}
