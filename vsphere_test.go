package main

import (
	"net/http"
	"os"
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

func TestViewPresetTemplates(t *testing.T) {
	router := gin.Default()
	private := router.Group("/api/v1")
	addPrivateRoutes(private)
	public := router.Group("/api/v1")
	addPublicRoutes(public)
	initCookies(router)

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
		WithHeaders(map[string]string{
			"Content-Type": "application/json",
		}).
		Expect().
		Status(http.StatusOK)

	e.GET("/api/v1/view/templates/preset").
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("templates")
}

func TestViewCustomTemplates(t *testing.T) {
	router := gin.Default()
	private := router.Group("/api/v1")
	addPrivateRoutes(private)
	public := router.Group("/api/v1")
	addPublicRoutes(public)
	initCookies(router)
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
		WithHeaders(map[string]string{
			"Content-Type": "application/json",
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
	public := router.Group("/api/v1")
	addPublicRoutes(public)
	initCookies(router)

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