package geeGorm

import (
	"database/sql"
	"geeGorm/dialect"
	"geeGorm/log"
	"geeGorm/session"
)

type Engine struct {
	db      *sql.DB
	dialect dialect.Dialect
}

func NewEngine(driveName string, dataSourceName string) (e *Engine, err error) {
	db, err := sql.Open(driveName, dataSourceName)

	if err != nil {
		log.Error(err)
		return
	}
	if err = db.Ping(); err != nil {
		log.Error(err)
		return
	}
	dial, ok := dialect.GetDialect(driveName)

	if !ok {
		log.Errorf("dialect %s not found", driveName)
		return
	}
	e = &Engine{db, dial}
	log.Info("Connect database success")
	return
}

func (e *Engine) Close() {
	if err := e.db.Close(); err != nil {
		log.Error("Failed to close database")
		return
	}

	log.Info("Close database success")
}

func (e *Engine) NewSession() *session.Session {
	return session.NewSession(e.db, e.dialect)
}
