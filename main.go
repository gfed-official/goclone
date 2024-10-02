package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"

	"goclone/config"
	"os"
)

type VSphereClient struct {
	client *vim25.Client
	ctx    context.Context
}

var (
	ldapConfig    config.LdapConfig
	mainConfig    = &config.Config{}
	templateMap   = map[string]Template{}
	vCenterConfig config.VCenterConfig
)

var (
	authManager             *object.AuthorizationManager
	cloneRole               *types.AuthorizationRole
	customCloneRole         *types.AuthorizationRole
	datastore               *object.Datastore
	destinationFolder       *object.Folder
	dvsMo                   mo.DistributedVirtualSwitch
	finder                  *find.Finder
	noAccessRole            *types.AuthorizationRole
	targetResourcePool      *object.ResourcePool
	templateFolder          *object.Folder
	vSphereClient           *VSphereClient
	wanPG                   *object.DistributedVirtualPortgroup
	competitionPG           *object.DistributedVirtualPortgroup
	competitionResourcePool *object.ResourcePool
)

func init() {
	// setup config

	config.ReadConfigFromEnv(mainConfig)
	err := config.CheckConfig(mainConfig)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "illegal config"))
	}

	vCenterConfig = mainConfig.VCenterConfig
	ldapConfig = mainConfig.LdapConfig

	// setup vSphere client
	u, err := soap.ParseURL(vCenterConfig.VCenterURL)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error parsing vCenter URL"))
	}

	u.User = url.UserPassword(vCenterConfig.VCenterUsername, vCenterConfig.VCenterPassword)
	ctx := context.Background()
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error creating vSphere client"))
	}

	client.Login(ctx, u.User)

	vSphereClient = &VSphereClient{
		client: client.Client,
		ctx:    context.Background(),
	}

	InitializeGovmomi()
	err = vSphereLoadTakenPortGroups()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding taken port groups"))
	}
}

func main() {
	go refreshSession()

	//setup logging
	gin.SetMode(gin.ReleaseMode)

	f, err := os.OpenFile(mainConfig.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
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

	public := router.Group("/api/v1")
	addPublicRoutes(public)

	private := router.Group("/api/v1")
	private.Use(authRequired)
	addPrivateRoutes(private)

	admin := router.Group("/api/v1/admin")
	admin.Use(authRequired, adminRequired)
	addAdminRoutes(admin)

	log.Fatalln(router.Run(":" + fmt.Sprint(mainConfig.Port)))
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/json")
		c.Writer.Header().Set("Access-Control-Allow-Origin", mainConfig.Fqdn)
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

	dc, err := finder.Datacenter(vSphereClient.ctx, vCenterConfig.Datacenter)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding datacenter"))
	}

	finder.SetDatacenter(dc)

	datastore, err = finder.Datastore(vSphereClient.ctx, vCenterConfig.Datastore)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding datastore"))
	}

	dswitch, err := finder.Network(vSphereClient.ctx, vCenterConfig.MainDistributedSwitch)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding distributed switch"))
	}

	dvs := object.NewDistributedVirtualSwitch(vSphereClient.client, dswitch.Reference())
	err = dvs.Properties(vSphereClient.ctx, dvs.Reference(), []string{"uuid"}, &dvsMo)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error getting distributed switch properties"))
	}

	templateFolder, err = finder.Folder(vSphereClient.ctx, vCenterConfig.TemplateFolder)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding template folder"))
	}

	destinationFolder, err = finder.Folder(vSphereClient.ctx, vCenterConfig.DestinationFolder)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding destination folder"))
	}

	targetResourcePool, err = finder.ResourcePool(vSphereClient.ctx, vCenterConfig.TargetResourcePool)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding target resource pool"))
	}

	competitionResourcePool, err = finder.ResourcePool(vSphereClient.ctx, vCenterConfig.CompetitionResourcePool)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding competition resource pool"))
	}

	pg, err := finder.Network(vSphereClient.ctx, vCenterConfig.DefaultWanPortGroup)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding WAN port group"))
	}
	wanPG = object.NewDistributedVirtualPortgroup(vSphereClient.client, pg.Reference())

	compPG, err := finder.Network(vSphereClient.ctx, vCenterConfig.CompetitionWanPortGroup)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding competition WAN port group"))
	}
	competitionPG = object.NewDistributedVirtualPortgroup(vSphereClient.client, compPG.Reference())

	authManager = object.NewAuthorizationManager(vSphereClient.client)
	roles, err := authManager.RoleList(vSphereClient.ctx)
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error listing roles"))
	}

	for _, role := range roles {
		if role.Name == vCenterConfig.CloneRole {
			cloneRole = &role
		}
		if role.Name == vCenterConfig.CustomCloneRole {
			customCloneRole = &role
		}
		if role.Name == "NoAccess" {
			noAccessRole = &role
		}
	}

	err = LoadTemplates()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error loading templates"))
	}

	fmt.Fprintln(os.Stdout, []any{"Initialized"}...)
}
