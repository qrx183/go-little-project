package clause

import (
	"fmt"
	"strings"
)

type generator func(values ...interface{}) (string, []interface{})

var generators map[Type]generator

func init() {
	generators[INSERT] = _insert
	generators[VALUES] = _values
	generators[SELECT] = _select
	generators[WHERE] = _where
	generators[LIMIT] = _limit
	generators[ORDERBY] = _orderby
	generators[UPDATE] = _update
	generators[DELETE] = _delete
	generators[COUNT] = _count
}

func genBindVars(num int) string {
	var res []string
	for i := 0; i < num; i++ {
		res = append(res, "?")
	}
	// 这里分隔符要有空格
	return strings.Join(res, ", ")
}
func _update(values ...interface{}) (string, []interface{}) {
	// UPDATE tableName SET k1 = ?, k2 = v ?, vars
	tableName := values[0]
	kv := values[1].(map[string]interface{})
	var sql []string
	var vars []interface{}

	for k, v := range kv {
		sql = append(sql, k+" = ? ")
		vars = append(vars, v)
	}

	return fmt.Sprintf("UPDATE %s SEt %s", tableName, strings.Join(sql, ",")), vars

}

func _delete(values ...interface{}) (string, []interface{}) {
	return fmt.Sprintf("DELETE FROM %s", values[0].(string)), []interface{}{}
}

func _count(values ...interface{}) (string, []interface{}) {
	return _select(values[0], []string{"count(*)"})
}

func _insert(values ...interface{}) (string, []interface{}) {
	// INSERT INTO tableNames(Fields)
	tableName := values[0]
	fields := strings.Join(values[1].([]string), ",")

	return fmt.Sprintf("INSERT INTO %s(%v)", tableName, fields), []interface{}{}
}

func _values(values ...interface{}) (string, []interface{}) {
	// VALUES (...), (...)
	var vars []interface{}
	var sql strings.Builder
	var bindStr string
	sql.WriteString("VALUES ")
	for i, value := range values {
		v := value.([]interface{})
		if bindStr == "" {
			bindStr = genBindVars(len(v))
		}

		sql.WriteString(fmt.Sprintf("(%v)", bindStr))

		if i+1 != len(values) {
			sql.WriteString(",")
		}

		vars = append(vars, v)
	}
	return sql.String(), vars
}

func _select(values ...interface{}) (string, []interface{}) {
	// SELECT fields FROM tableName
	tableName := values[0]
	fields := strings.Join(values[1].([]string), ",")

	return fmt.Sprintf("SELECT %v FROM %s", fields, tableName), []interface{}{}
}

func _where(values ...interface{}) (string, []interface{}) {
	// WHERE desc
	desc, vars := values[0], values[1:]
	return fmt.Sprintf("WHERE %s", desc), vars
}

func _limit(values ...interface{}) (string, []interface{}) {
	// LIMIT ?
	return "LIMIT ?", values
}

func _orderby(values ...interface{}) (string, []interface{}) {
	// ORDER BY field
	return fmt.Sprintf("ORDER BY %s", values[0]), []interface{}{}
}
