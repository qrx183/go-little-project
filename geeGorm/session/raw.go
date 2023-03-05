package session

import (
	"database/sql"
	"geeGorm/clause"
	"geeGorm/dialect"
	"geeGorm/log"
	"geeGorm/schema"
	"strings"
)

type Session struct {
	db *sql.DB
	// 这里使用strings.Builder的原因:Session是需要在会话中复用的,而string类型是一个可读不可变的,因此每次复用都会申请一个新的内存空间
	// strings.Builder的底层是[]byte实现的,堆内存更友好一些
	sql    strings.Builder
	sqlVal []interface{}

	dialect dialect.Dialect // 数据库类型  获取对应数据库的数据类型(辅助将go对象转换为对应数据库的表结构)   判断表是否存在
	schema  *schema.Schema  // 转换go对象为表结构的对象

	clause clause.Clause
}

func NewSession(db *sql.DB, dialect dialect.Dialect) *Session {
	return &Session{
		db:      db,
		dialect: dialect,
	}
}

func (s *Session) DB() *sql.DB {
	return s.db
}
func (s *Session) Clear() {
	s.sql.Reset()
	s.sqlVal = nil
}

func (s *Session) Raw(sql string, value ...interface{}) *Session {
	s.sql.WriteString(sql)
	s.sql.WriteString(" ")

	s.sqlVal = append(s.sqlVal, value...)
	return s
}

func (s *Session) Exec() (result sql.Result, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVal)

	if result, err = s.DB().Exec(s.sql.String(), s.sqlVal...); err != nil {
		log.Error(err)
	}
	return
}

func (s *Session) QueryRaw() *sql.Row {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVal)

	return s.DB().QueryRow(s.sql.String(), s.sqlVal...)
}

func (s *Session) QueryRows() (rows *sql.Rows, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVal)

	if rows, err = s.DB().Query(s.sql.String(), s.sqlVal...); err != nil {
		log.Error(err)
	}
	return
}
