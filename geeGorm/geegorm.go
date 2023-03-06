package geeGorm

import (
	"database/sql"
	"fmt"
	"geeGorm/dialect"
	"geeGorm/log"
	"geeGorm/session"
	"strings"
)

type Engine struct {
	db      *sql.DB
	dialect dialect.Dialect
}
type TsFunc func(*session.Session) (interface{}, error)

func (e *Engine) Transaction(f TsFunc) (result interface{}, err error) {
	s := e.NewSession()

	if err := s.Begin(); err != nil {
		return nil, err
	}

	defer func() {
		if err := recover(); err != nil {
			_ = s.RollBack()
			panic(err)
		} else if err != nil {
			// 这里的err不为空,因此不能保留err最外层的值
			_ = s.RollBack()
		} else {
			// 这里的err为空,因此需要将commit的结果返回
			err = s.Commit()
		}
	}()

	return f(s)
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

func (e *Engine) Migrate(value interface{}) error {
	_, err := e.Transaction(func(s *session.Session) (result interface{}, err error) {
		if !s.Model(value).HasTable() {
			log.Infof("table %s doesn't exist", s.RefTable().Name)
			return nil, s.CreateTable()
		}

		table := s.RefTable()
		// 因为之前已经用value(新对象)Model过了,但这里只是改了schema中的值,真正数据库中的字段没有改
		newFields := table.FieldsName
		rows, _ := s.Raw(fmt.Sprintf("SELECT * FROM %s LIMIT 1", table.Name)).QueryRows()

		oldFields, _ := rows.Columns()
		addFields := difference(oldFields, newFields)
		delFields := difference(newFields, oldFields)

		for _, col := range addFields {
			field := table.GetField(col)
			_, err = s.Raw(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table.Name, field.Name, field.Type)).Exec()
			if err != nil {
				return
			}
		}
		if len(delFields) == 0 {
			return
		}

		tmp := "tmp_" + table.Name
		fieldStr := strings.Join(oldFields, ", ")
		// sqlite3没有专门为删除字段的sql,所以需要新创建
		// 先把新对象的字段存储到一个新表中,删除原来的表,把新表的名字改成原来表的名字
		s.Raw(fmt.Sprintf("CREATE TABLE %s SELECT AS %s FROM %s", tmp, fieldStr, table.Name))
		s.Raw(fmt.Sprintf("DROP TABLE %s", table.Name))
		s.Raw(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tmp, table.Name))

		_, err = s.Exec()

		return
	})
	return err
}

func difference(oldFields []string, newFields []string) []string {
	diffFields := make([]string, 0)

	diffMap := make(map[string]bool)

	for _, field := range oldFields {
		diffMap[field] = true
	}

	for _, val := range newFields {
		if _, ok := diffMap[val]; !ok {
			diffFields = append(diffFields, val)
		}
	}
	return diffFields
}
