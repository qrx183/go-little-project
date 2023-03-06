package session

import "geeGorm/log"

func (s *Session) Begin() (err error) {
	log.Info("transaction begin")
	if _, err := s.db.Begin(); err != nil {
		log.Error(err)
		return err
	}

	return
}

func (s *Session) RollBack() (err error) {
	log.Info("transaction rollback")
	if err := s.tx.Rollback(); err != nil {
		log.Error(err)
		return err
	}
	return
}

func (s *Session) Commit() (err error) {
	log.Info("transaction commit")
	if err := s.tx.Commit(); err != nil {
		log.Error(err)
		return err
	}
	return
}
