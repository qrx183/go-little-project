package session

import (
	"errors"
	"geeGorm/clause"
	"reflect"
)

func (s *Session) COUNT(values ...interface{}) (int64, error) {

	// COUNT只需要返回行数,不需要用反射去进行值的复制
	s.clause.Set(clause.COUNT, s, s.RefTable().Name, values)

	sql, vars := s.clause.Build(clause.COUNT, clause.WHERE)

	row := s.Raw(sql, vars).QueryRaw()
	var tmp int64
	if err := row.Scan(&tmp); err != nil {
		return 0, err
	}
	return tmp, nil
}

func (s *Session) DELETE(values ...interface{}) (int64, error) {
	s.clause.Set(clause.DELETE, s.RefTable().Name, values)

	sql, vars := s.clause.Build(clause.DELETE, clause.WHERE)

	result, err := s.Raw(sql, vars).Exec()

	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Session) UPDATE(values ...interface{}) (int64, error) {
	m, ok := values[0].(map[string]interface{})

	if !ok {
		m = make(map[string]interface{})
		for i := 0; i < len(values); i += 2 {
			m[values[i].(string)] = values[i+1]
		}
	}

	s.clause.Set(clause.UPDATE, s.RefTable().Name, m)

	sql, vars := s.clause.Build(clause.UPDATE, clause.WHERE)

	result, err := s.Raw(sql, vars).Exec()

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (s *Session) Insert(values ...interface{}) (int64, error) {

	recordValues := make([]interface{}, 0)

	for _, value := range values {
		table := s.Model(value).RefTable()

		s.clause.Set(clause.INSERT, table.Name, table.FieldsName)

		recordValues = append(recordValues, table.RecordValues(value))
	}

	s.clause.Set(clause.VALUES, recordValues...)

	sql, vars := s.clause.Build(clause.INSERT, clause.VALUES)

	result, err := s.Raw(sql, vars).Exec()

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()

}

func (s *Session) Find(values interface{}) error {
	destValue := reflect.Indirect(reflect.ValueOf(values))
	destType := destValue.Type().Elem()

	table := s.Model(values).RefTable()
	s.clause.Set(clause.SELECT, table.Name, table.FieldsName)

	// 这里其实只执行select
	sql, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.LIMIT, clause.ORDERBY)

	rows, err := s.Raw(sql, vars).QueryRows()
	if err != nil {
		return err
	}

	for rows.Next() {
		dest := reflect.New(destType).Elem()
		// value其实只是一个傀儡,是一个承载了go对象每个字段的指针的载体
		var values []interface{}
		for _, name := range table.FieldsName {
			// 将dest的每个字段的指针传给values
			values = append(values, dest.FieldByName(name).Addr().Interface())
		}
		// 通过对values进行赋值,进而给dest每个字段进行了赋值
		if err := rows.Scan(values...); err != nil {
			return err
		}
		destValue.Set(reflect.Append(destValue, dest))
	}
	return rows.Close()
}

func (s *Session) LIMIT(values ...interface{}) *Session {
	s.clause.Set(clause.LIMIT, values...)
	return s
}

func (s *Session) WHERE(values ...interface{}) *Session {
	s.clause.Set(clause.WHERE, values...)
	return s
}

func (s *Session) ORDERBY(values ...interface{}) *Session {
	s.clause.Set(clause.ORDERBY, values...)
	return s
}

func (s *Session) FIRST(values ...interface{}) error {
	dest := reflect.Indirect(reflect.ValueOf(values))

	destSlice := reflect.New(reflect.SliceOf(dest.Type())).Elem()

	if err := s.LIMIT(1).Find(destSlice.Addr().Interface()); err != nil {
		return err
	}

	if destSlice.Len() == 0 {
		return errors.New("NOT FOUND")
	}

	dest.Set(destSlice.Index(0))
	return nil
}
