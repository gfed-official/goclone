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
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"

	"goclone/models"
	"os"
)

type VSphereClient struct {
	client     *vim25.Client
	restClient *rest.Client
	ctx        context.Context
}

var (
	vSphereClient      *VSphereClient
	tomlConf           = &models.Config{}
	configPath         = "./config.conf"
	finder             *find.Finder
	datastore          *object.Datastore
	dvsMo              mo.DistributedVirtualSwitch
	templateFolder     *object.Folder
	tagManager         *tags.Manager
	targetResourcePool *object.ResourcePool
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

	rc := rest.NewClient(client.Client)
	err = rc.Login(ctx, u.User)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error creating REST client"))
	}

	vSphereClient = &VSphereClient{
		client:     client.Client,
		restClient: rc,
		ctx:        context.Background(),
	}

	InitializeGovmomi()
}

func main() {

	WebClone("Evan-Test", "0040_RvBCoreNetwork", "edeters", 69)
	//DestroyResources("69_Evan-Test_edeters")

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

func InitializeGovmomi() {
	finder = find.NewFinder(vSphereClient.client, true)

	dc, err := finder.Datacenter(vSphereClient.ctx, tomlConf.Datacenter)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding datacenter"))
	}

	finder.SetDatacenter(dc)

	datastore, err = finder.Datastore(vSphereClient.ctx, tomlConf.Datastore)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding datastore"))
	}

	dswitch, err := finder.Network(vSphereClient.ctx, tomlConf.MainDistributedSwitch)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding distributed switch"))
	}

	dvs := object.NewDistributedVirtualSwitch(vSphereClient.client, dswitch.Reference())
	err = dvs.Properties(vSphereClient.ctx, dvs.Reference(), []string{"uuid"}, &dvsMo)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error getting distributed switch properties"))
	}

	templateFolder, err = finder.Folder(vSphereClient.ctx, tomlConf.TemplateFolder)

	tagManager = tags.NewManager(vSphereClient.restClient)

	targetResourcePool, err = finder.ResourcePool(vSphereClient.ctx, tomlConf.TargetResourcePool)
	fmt.Fprintln(os.Stdout, []any{"Initialized"}...)
}
