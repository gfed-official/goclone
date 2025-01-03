package routes

import (
	"goclone/internal/auth"
	"goclone/internal/config"
	"net/http"
	"os"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var (
	router        *gin.Engine
	adminCookie   *httpexpect.Cookie
	noAdminCookie *httpexpect.Cookie
	pods          *httpexpect.Object
	templates     *httpexpect.Object
	e             *httpexpect.Expect
)

func init() {
    conf, err := config.GetConfig()
    if err != nil {
        panic(err)
    }

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
	e = httpexpect.WithConfig(httpexpect.Config{
		Client: &http.Client{
			Transport: httpexpect.NewBinder(router),
			Jar:       httpexpect.NewCookieJar(),
		},
		Reporter: httpexpect.NewAssertReporter(t),
		Printers: []httpexpect.Printer{
			httpexpect.NewDebugPrinter(t, true),
		},
	})

	testFuncs := []struct {
		Name string
		Test func(t *testing.T)
	}{
		{
			Name: "HealthEndpoint",
			Test: HealthEndpoint,
		},
		{
			Name: "RegisterEndpoint",
			Test: RegisterEndpoint,
		},
		{
			Name: "LoginEndpoint",
			Test: LoginEndpoint,
		},
		{
			Name: "ViewPresetTemplatesEndpoint",
			Test: ViewPresetTemplatesEndpoint,
		},
		{
			Name: "ViewCustomTemplatesEndpoint",
			Test: ViewCustomTemplatesEndpoint,
		},
		{
			Name: "TemplateCloneEndpoint",
			Test: TemplateCloneEndpoint,
		},
		{
			Name: "ViewPodsEndpoint",
			Test: ViewPodsEndpoint,
		},
		{
			Name: "AdminGetPodsEndpoint",
			Test: AdminGetPodsEndpoint,
		},
		{
			Name: "DeletePodEndpoint",
			Test: DeletePodEndpoint,
		},
		{
			Name: "DeleteUserEndpoint",
			Test: DeleteUserEndpoint,
		},
	}

	for _, testFunc := range testFuncs {
		t.Run(testFunc.Name, testFunc.Test)
	}
}

func HealthEndpoint(t *testing.T) {
	e.GET("/api/v1/health").
		Expect().
		Status(http.StatusOK).
		JSON().Object().HasValue("status", "ok")
}

func RegisterEndpoint(t *testing.T) {
	e.POST("/api/v1/register").
		WithJSON(map[string]interface{}{
			"username": "goclone_test",
			"password": os.Getenv("VCENTER_PASSWORD"),
		}).
		Expect().
		Status(http.StatusOK)
}

func LoginEndpoint(t *testing.T) {
	type testCase struct {
		Username       string
		Password       string
		ExpectedStatus int
	}

	testCases := []testCase{
		{
			Username:       "adsfjasdkljfaalkajdsfhasjhdfdshj",
			Password:       "adskjfalkdjfalksdjlfajdflajd",
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Username:       os.Getenv("VCENTER_USERNAME"),
			Password:       os.Getenv("VCENTER_PASSWORD"),
			ExpectedStatus: http.StatusOK,
		},
		{
			Username:       "goclone_test",
			Password:       os.Getenv("VCENTER_PASSWORD"),
			ExpectedStatus: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		resp := e.POST("/api/v1/login").
			WithJSON(map[string]interface{}{
				"username": tc.Username,
				"password": tc.Password,
			}).
			Expect().
			Status(tc.ExpectedStatus)
		if tc.ExpectedStatus == http.StatusOK && tc.Username == os.Getenv("VCENTER_USERNAME") {
			adminCookie = resp.Cookie("kamino")
		}
		if tc.ExpectedStatus == http.StatusOK && tc.Username == "goclone_test" {
			noAdminCookie = resp.Cookie("kamino")
		}
	}
}

func ViewPresetTemplatesEndpoint(t *testing.T) {
	type testCase struct {
		Cookie         *httpexpect.Cookie
		ExpectedLength int
	}

	testCases := []testCase{
		{
			Cookie:         adminCookie,
			ExpectedLength: 2,
		},
		{
			Cookie:         noAdminCookie,
			ExpectedLength: 1,
		},
	}

	for _, tc := range testCases {
		obj := e.GET("/api/v1/view/templates/preset").
			WithCookie(tc.Cookie.Raw().Name, tc.Cookie.Raw().Value).
			Expect().
			Status(http.StatusOK).
			JSON().Object()

		if tc.Cookie == adminCookie {
			templates = obj.ContainsKey("templates")
		}

		obj.Value("templates").Array().Length().IsEqual(tc.ExpectedLength)
	}
}

func ViewCustomTemplatesEndpoint(t *testing.T) {
	e.GET("/api/v1/view/templates/custom").
		WithCookie(adminCookie.Raw().Name, adminCookie.Raw().Value).
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("templates")
}

func TemplateCloneEndpoint(t *testing.T) {
	templateName := templates.Value("templates").Array().Value(0).String().Raw()

	e.POST("/api/v1/pod/clone/template").
		WithCookie(adminCookie.Raw().Name, adminCookie.Raw().Value).
		WithJSON(map[string]interface{}{
			"template": templateName,
		}).
		Expect().
		Status(http.StatusOK)
}

func ViewPodsEndpoint(t *testing.T) {
	pods = e.GET("/api/v1/view/pods").
		WithCookie(adminCookie.Raw().Name, adminCookie.Raw().Value).
		Expect().
		Status(http.StatusOK).
		JSON().Object().ContainsKey("pods")
}

func AdminGetPodsEndpoint(t *testing.T) {
	type testCase struct {
		Cookie         *httpexpect.Cookie
		ExpectedStatus int
	}

	testCases := []testCase{
		{
			Cookie:         adminCookie,
			ExpectedStatus: http.StatusOK,
		},
		{
			Cookie:         noAdminCookie,
			ExpectedStatus: http.StatusUnauthorized,
		},
	}

	pod := pods.Value("pods").Array().Value(0).Object()
	podName := pod.Value("Name").String().Raw()

	for _, tc := range testCases {
		obj := e.GET("/api/v1/admin/view/pods").
			WithCookie(tc.Cookie.Raw().Name, tc.Cookie.Raw().Value).
			Expect().
			Status(tc.ExpectedStatus)

		if tc.ExpectedStatus == http.StatusOK {
			obj.JSON().Array().Value(0).Object().HasValue("Name", podName)
		}
	}
}

func DeletePodEndpoint(t *testing.T) {
	pod := pods.Value("pods").Array().Value(0).Object()
	podName := pod.Value("Name").String().Raw()

	e.DELETE("/api/v1/pod/delete/"+podName).
		WithCookie(adminCookie.Raw().Name, adminCookie.Raw().Value).
		Expect().
		Status(http.StatusOK)
}

func DeleteUserEndpoint(t *testing.T) {
	e.DELETE("/api/v1/admin/user/delete/goclone_test").
		WithCookie(adminCookie.Raw().Name, adminCookie.Raw().Value).
		Expect().
		Status(http.StatusOK)
}
