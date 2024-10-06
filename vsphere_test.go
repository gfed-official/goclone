package main

import (
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealth(t *testing.T) {
	router := gin.Default()
	public := router.Group("/api/v1")
	public.GET("/health", health)

	e := httpexpect.WithConfig(httpexpect.Config{
		Client: &http.Client{
			Transport: httpexpect.NewBinder(router),
			Jar:       httpexpect.NewCookieJar(),
		},
		Reporter: httpexpect.NewAssertReporter(t),
		Printers: []httpexpect.Printer{
			httpexpect.NewDebugPrinter(t, true),
		},
	})

	e.GET("/api/v1/health").
		Expect().
		Status(http.StatusOK).
		JSON().Object().HasValue("status", "ok")
}
