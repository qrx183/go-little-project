package main

import (
	"gee/context/mygee"
	"net/http"
)

func main() {
	r := mygee.Default()
	r.GET("/", func(c *mygee.Context) {
		c.String(http.StatusOK, "Hello Geektutu\n")
	})
	// index out of range for testing Recovery()
	r.GET("/panic", func(c *mygee.Context) {
		names := []string{"geektutu"}
		c.String(http.StatusOK, names[100])
	})

	r.Run(":9090")
}
