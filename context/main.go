package main

import (
	gee "gee/context/mygee"
	"net/http"
)

func main() {
	r := gee.New()

	r.GET("/", func(c *gee.Context) {
		c.HTML(http.StatusOK, "<h1>Hello Gee </h1>")
	})

	r.GET("/hello", func(c *gee.Context) {
		name := c.Query("name")
		c.String(http.StatusOK, "hello %s", name)
	})

	r.POST("/login", func(c *gee.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")
		c.JSON(http.StatusOK, gee.H{
			"username": username,
			"password": password,
		})
	})

	r.Run(":9090")

}
