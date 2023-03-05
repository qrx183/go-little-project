package schema

import (
	"geeGorm/dialect"
	"go/ast"
	"reflect"
)

type Field struct {
	Name string
	Type string
	Tag  string
}

type Schema struct {
	Model      interface{}
	Name       string
	Fields     []*Field
	FieldsName []string
	FieldsMap  map[string]*Field
}

func (s *Schema) GetField(name string) *Field {
	return s.FieldsMap[name]
}

func (s *Schema) RecordValues(values ...interface{}) []interface{} {
	destValue := reflect.Indirect(reflect.ValueOf(values))
	var fieldValues []interface{}

	for _, field := range s.Fields {
		fieldValues = append(fieldValues, destValue.FieldByName(field.Name).Interface())
	}

	return fieldValues
}

func Parse(model interface{}, dialect dialect.Dialect) *Schema {
	// 将go中的对象转换成Schema类型
	modelType := reflect.Indirect(reflect.ValueOf(model)).Type()

	schema := &Schema{
		Model:      model,
		Name:       modelType.Name(),
		Fields:     []*Field{},
		FieldsName: []string{},
		FieldsMap:  make(map[string]*Field),
	}

	for i := 0; i < modelType.NumField(); i++ {
		f := modelType.Field(i)
		if !f.Anonymous && ast.IsExported(f.Name) {
			field := &Field{
				Name: f.Name,
				Type: dialect.DataTypeOf(reflect.Indirect(reflect.New(f.Type))),
			}

			if v, ok := f.Tag.Lookup("geeorm"); ok {
				field.Tag = v
			}
			schema.Fields = append(schema.Fields, field)
			schema.FieldsName = append(schema.FieldsName, field.Name)
			schema.FieldsMap[field.Name] = field
		}
	}
	return schema
}
