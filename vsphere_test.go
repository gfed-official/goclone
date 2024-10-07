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
	c      *httpexpect.Cookie
	pods   *httpexpect.Object
)

func init() {
	gin.SetMode(gin.TestMode)
	router = gin.Default()
	router.MaxMultipartMemory = 8 << 20

	session := sessions.Sessions("kamino", cookie.NewStore([]byte("kamino")))
	router.Use(session)

	public := router.Group("/api/v1")
	addPublicRoutes(public)

	private := router.Group("/api/v1")
	private.Use(authRequired)
	addPrivateRoutes(private)

	admin := router.Group("/api/v1/admin")
	admin.Use(adminRequired)
	addAdminRoutes(admin)
}

func TestAPI(t *testing.T) {
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

	testFuncs := []func(*httpexpect.Expect){
		HealthEndpoint,
		LoginEndpoint,
		ViewPresetTemplatesEndpoint,
		ViewCustomTemplatesEndpoint,
		TemplateCloneEndpoint,
		ViewPodsEndpoint,
		AdminGetPodsEndpoint,
		DeletePodEndpoint,
	}

	for _, testFunc := range testFuncs {
		testFunc(e)
	}

}

func HealthEndpoint(e *httpexpect.Expect) {
	e.GET("/api/v1/health").
		Expect().
		Status(http.StatusOK).
		JSON().Object().HasValue("status", "ok")
}

func LoginEndpoint(e *httpexpect.Expect) {
	userName := os.Getenv("VCENTER_USERNAME")
	password := os.Getenv("VCENTER_PASSWORD")

	c = e.POST("/api/v1/login").
		WithJSON(map[string]interface{}{
			"username": userName,
			"password": password,
		}).
		Expect().
		Status(http.StatusOK).
		Cookie("kamino")

}

func ViewPresetTemplatesEndpoint(e *httpexpect.Expect) {
	e.GET("/api/v1/view/templates/preset").
		WithCookie(c.Raw().Name, c.Raw().Value).
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("templates")
}

func ViewCustomTemplatesEndpoint(e *httpexpect.Expect) {
	e.GET("/api/v1/view/templates/custom").
		WithCookie(c.Raw().Name, c.Raw().Value).
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("templates")
}

func TemplateCloneEndpoint(e *httpexpect.Expect) {
	e.POST("/api/v1/pod/clone/template").
		WithCookie(c.Raw().Name, c.Raw().Value).
		WithJSON(map[string]interface{}{
			"template": "CPTC-Web",
		}).
		Expect().
		Status(http.StatusOK)
}

func ViewPodsEndpoint(e *httpexpect.Expect) {
	pods = e.GET("/api/v1/view/pods").
		WithCookie(c.Raw().Name, c.Raw().Value).
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("pods")
}

func AdminGetPodsEndpoint(e *httpexpect.Expect) {
	pod := pods.Value("pods").Array().Value(0).Object()
	podName := pod.Value("Name").String().Raw()

	e.GET("/api/v1/admin/view/pods").
		WithCookie(c.Raw().Name, c.Raw().Value).
		Expect().
		Status(http.StatusOK).
		JSON().Array().Value(0).Object().ContainsKey("Name").HasValue("Name", podName)
}

func DeletePodEndpoint(e *httpexpect.Expect) {
	pod := pods.Value("pods").Array().Value(0).Object()
	podName := pod.Value("Name").String().Raw()

	e.DELETE("/api/v1/pod/delete/"+podName).
		WithCookie(c.Raw().Name, c.Raw().Value).
		Expect().
		Status(http.StatusOK)
}
