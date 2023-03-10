package mygee

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
)

func trace(message string) string {
	var pcs [32]uintptr

	n := runtime.Callers(3, pcs[:])

	var res strings.Builder

	res.WriteString(message + "\nTraceback:")

	for _, pc := range pcs[:n] {
		fn := runtime.FuncForPC(pc)

		file, line := fn.FileLine(pc)
		res.WriteString(fmt.Sprintf("\n\t%s:%d", file, line))
	}
	return res.String()
}

func Recovery() HandlerFunc {
	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil {
				message := fmt.Sprintf("%s", err)
				log.Printf("%s\n\n", trace(message))
				c.String(http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		c.Next()
	}
}
