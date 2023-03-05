package session

import (
	"fmt"
	"geeGorm/log"
	"geeGorm/schema"
	"reflect"
	"strings"
)

func (s *Session) Model(value interface{}) *Session {
	if s.schema == nil || reflect.TypeOf(value) != reflect.TypeOf(s.schema.Model) {
		s.schema = schema.Parse(value, s.dialect)
	}
	return s
}

func (s *Session) RefTable() *schema.Schema {
	// 这里对获取schema进行一层封装,保证schema不为空
	if s.schema == nil {
		log.Error("Model is not set")
	}
	return s.schema
}

func (s *Session) CreateTable() error {
	name := s.RefTable().Name

	var field []string

	for _, f := range s.schema.Fields {
		field = append(field, fmt.Sprintf("%s %s %s", f.Name, f.Type, f.Tag))
	}
	desc := strings.Join(field, ",")

	_, err := s.Raw(fmt.Sprintf("CREATE TABLE %s(%s);", name, desc)).Exec()

	return err
}

func (s *Session) DropTable() error {
	name := s.RefTable().Name

	_, err := s.Raw(fmt.Sprintf("DROP TABLE IF EXISTS %s"), name).Exec()
	return err
}

func (s *Session) HasTable() bool {
	name := s.RefTable().Name

	hasTableSQL, args := s.dialect.TableExistSQL(name)

	row := s.Raw(hasTableSQL, args...).QueryRaw()

	var tmp string
	_ = row.Scan(&tmp)

	return tmp == name
}
