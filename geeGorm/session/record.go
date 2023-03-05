package session

import (
	"geeGorm/clause"
	"reflect"
)

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
