package main

import (
	"database/sql"
	"fmt"
	"log"
	"reflect"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	name string
	age  int
}

func main() {
	u := &User{}
	destSlice := (reflect.ValueOf(u))
	fmt.Println(destSlice)
	fmt.Println(destSlice.Type())
	destType := destSlice.Type().Elem()
	fmt.Println(destType)

	//t := reflect.Indirect(reflect.ValueOf(u))
	//fmt.Println("type:", t)
	//
	//fmt.Println(t.Type().Elem())
}

func main1() {
	db, _ := sql.Open("sqlite3", "gee.db")
	defer func() { _ = db.Close() }()

	_, _ = db.Exec("DROP TABLE IF EXISTS User;")

	_, _ = db.Exec("CREATE TABLE User(Name text);")
	result, err := db.Exec("INSERT INTO User VALUES (?), (?)", "Tom", "Jack")

	if err == nil {
		affected, _ := result.RowsAffected()
		log.Println("affected:", affected)
	}

	row := db.QueryRow("SELECT Name FROM User LIMIT 1")

	var name string

	if err := row.Scan(&name); err == nil {
		log.Println("name:", name)
	}

}
