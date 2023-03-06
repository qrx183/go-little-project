package clause

import "strings"

type Type int

const (
	INSERT Type = iota
	VALUES
	SELECT
	LIMIT
	WHERE
	ORDERBY
	UPDATE
	DELETE
	COUNT
)

type Clause struct {
	sql     map[Type]string
	sqlVars map[Type][]interface{}
}

func (c *Clause) Set(name Type, values ...interface{}) {
	if c.sql == nil {
		c.sqlVars = make(map[Type][]interface{})
		c.sql = make(map[Type]string)
	}

	sql, vars := generators[name](values...)

	c.sql[name] = sql
	c.sqlVars[name] = vars
}

func (c *Clause) Build(orders ...Type) (string, []interface{}) {
	// 按照传入的顺序进行sql语句的构建
	var sqls []string
	var sqlVars []interface{}

	for _, order := range orders {
		if _, ok := c.sql[order]; ok {
			sqls = append(sqls, c.sql[order])
			sqlVars = append(sqlVars, c.sqlVars[order])
		}
	}

	return strings.Join(sqls, " "), sqlVars
}
