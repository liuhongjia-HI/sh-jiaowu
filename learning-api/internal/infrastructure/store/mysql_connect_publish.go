package store

import "errors"

// connectPrepared performs open/ping/schema/bootstrap/load without holding the
// request mutex. Only the final state publication uses a short critical section.
// Reconnection is intentionally rejected: Connect* is a startup lifecycle API.
func (s *MemoryStore) connectPrepared(dsn string, schedulingOnly bool) error {
	s.mu.Lock()
	if s.db != nil {
		s.mu.Unlock()
		return errors.New("database is already connected")
	}
	work := s.cloneForMutation()
	s.mu.Unlock()

	var err error
	if schedulingOnly {
		err = work.connectSchedulingDBUnlocked(dsn)
	} else {
		err = work.connectDatabaseUnlocked(dsn)
	}
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		_ = work.db.Close()
		return errors.New("database was connected concurrently")
	}
	s.publishMutation(work)
	s.db = work.db
	return nil
}
