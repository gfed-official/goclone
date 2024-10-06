package main

import (
	"net/http"
	"os"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var (
	router *gin.Engine
)

func init() {
	gin.SetMode(gin.TestMode)
	router = gin.Default()
	router.MaxMultipartMemory = 8 << 20

	session := sessions.Sessions("kamino", cookie.NewStore([]byte("kamino")))
	router.Use(session)

	private := router.Group("/api/v1")
	addPrivateRoutes(private)

	public := router.Group("/api/v1")
	addPublicRoutes(public)

}

func TestHealth(t *testing.T) {
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

func TestViewPresetTemplates(t *testing.T) {
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

	userName := os.Getenv("VCENTER_USERNAME")
	password := os.Getenv("VCENTER_PASSWORD")

	e.POST("/api/v1/login").
		WithJSON(map[string]interface{}{
			"username": userName,
			"password": password,
		}).
		Expect().
		Status(http.StatusOK).
		JSON().Object().HasValue("message", "Successfully logged in!")

	e.GET("/api/v1/view/templates/preset").
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("templates")
}

func TestViewCustomTemplates(t *testing.T) {
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

	userName := os.Getenv("VCENTER_USERNAME")
	password := os.Getenv("VCENTER_PASSWORD")

	e.POST("/api/v1/login").
		WithJSON(map[string]interface{}{
			"username": userName,
			"password": password,
		}).
		Expect().
		Status(http.StatusOK)

	e.GET("/api/v1/view/templates/custom").
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("templates")
}

func TestTemplateClone(t *testing.T) {
	router := gin.Default()
	private := router.Group("/api/v1")
	addPrivateRoutes(private)

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

	e.POST("/api/v1/pod/clone/template").
		WithJSON(map[string]interface{}{
			"template": "CPTC-Web",
		}).
		Expect().
		Status(http.StatusOK)
}
