package main

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25/soap"

	"goclone/models"
)

type VSphereClient struct {
    client *govmomi.Client
    restClient *rest.Client
    ctx context.Context
}

var (
    vSphereClient *VSphereClient
	tomlConf      = &models.Config{}
	configPath    = "./config.conf"
	finder        = &find.Finder{}
)

func init() {
	// setup config
	models.ReadConfig(tomlConf, configPath)
	err := models.CheckConfig(tomlConf)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	// setup vSphere client
	u, err := soap.ParseURL(tomlConf.VCenterURL)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error parsing vCenter URL"))
	}

	u.User = url.UserPassword(tomlConf.VCenterUsername, tomlConf.VCenterPassword)
    ctx := context.Background()
    client, err := govmomi.NewClient(ctx, u, true)
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error creating vSphere client"))
    }

    vSphereClient = &VSphereClient{
        client: client,
        restClient: rest.NewClient(client.Client),
        ctx: context.Background(),
    }
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error creating vSphere client"))
	}
}

func main() {
	finder = find.NewFinder(vSphereClient.client.Client, true)

	dc, err := finder.Datacenter(vSphereClient.ctx, tomlConf.Datacenter)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding datacenter"))
	}

	finder.SetDatacenter(dc)

    ds, err := finder.DatastoreList(vSphereClient.ctx, "*")
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error finding default datastore"))
    }
    fmt.Println("Default datastore", ds)

    fmt.Println("Logged in to vSphere REST API", vSphereClient.restClient)

    tag, err := CreateTag("test_tag")
    if err != nil {
        log.Fatalln(errors.Wrap(err, "Error creating tag"))
    }
    CreatePortGroup("test_port", 69)
    AssignTagToPortGroup(tag, "test_port")

    /**

	// call before go routine to ensure it finishes before starting router
	err = vSphereLoadTakenPortGroups()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding taken port groups"))
	}

	go refreshSession()

	//setup logging
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()

	f, err := os.OpenFile(tomlConf.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to open log file"))
	}
	defer f.Close()

	log.SetOutput(f)
	gin.DefaultWriter = io.MultiWriter(f)

	// setup router
	router := gin.Default()
	router.Use(CORSMiddleware())
	router.MaxMultipartMemory = 8 << 20 // 8Mib
	initCookies(router)

	public := router.Group("/")
	addPublicRoutes(public)

	private := router.Group("/")
	private.Use(authRequired)
	addPrivateRoutes(private)

	if tomlConf.Https {
		log.Fatalln(router.RunTLS(":"+fmt.Sprint(tomlConf.Port), tomlConf.Cert, tomlConf.Key))
	} else {
		log.Fatalln(router.Run(":" + fmt.Sprint(tomlConf.Port)))
	}
    */
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("Access-Control-Allow-Origin", tomlConf.DomainName)
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Origin")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
		}

		c.Next()
	}
}
