package vsphere

import (
	"context"
	"fmt"
	"goclone/internal/auth"
	"goclone/internal/config"
	"log"
	"net/url"

	"github.com/pkg/errors"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

type VSphereClient struct {
	client *vim25.Client
	ctx    context.Context
    conf *config.Provider
    authMgr auth.AuthManager    
}

var (
	authMgr        *auth.AuthManager
	mainConfig    = &config.Config{}
	templateMap   = map[string]Template{}
	vCenterConfig config.VCenter
    tracer trace.Tracer
)

var (
	authManager             *object.AuthorizationManager
	cloneRole               *types.AuthorizationRole
	customCloneRole         *types.AuthorizationRole
	customFieldsManager     *object.CustomFieldsManager
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

func NewVSphereProvider(conf *config.Config, authMgr *auth.AuthManager) *VSphereClient {
	// setup vSphere client
    tracer = conf.Core.Tracer

    fmt.Println("Setting up vSphere Provider")
	u, err := soap.ParseURL(conf.Provider.URL)
	if err != nil {
        fmt.Println("Error parsing vCenter URL")
		log.Fatalln(errors.Wrap(err, "Error parsing vCenter URL"))
	}

	u.User = url.UserPassword(conf.Provider.Username, conf.Provider.Password)
	ctx := context.Background()
	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
        fmt.Println("Error creating vSphere client")
		log.Fatalln(errors.Wrap(err, "Error creating vSphere client"))
	}

	vSphereClient = &VSphereClient{
		client: client.Client,
		ctx:    context.Background(),
        conf:   &conf.Provider,
	}

    vCenterConfig = conf.Provider.VCenter

	InitializeGovmomi()
	err = vSphereLoadTakenPortGroups()
	if err != nil {
		log.Fatalln(errors.Wrap(err, "Error finding taken port groups"))
	}

    eg := errgroup.Group{}
    eg.Go(func() error {
        return LoadTemplates(ctx)  
    })

    if err := eg.Wait(); err != nil {
        fmt.Println("Error loading templates", err)
    }

	go refreshSession()

	return vSphereClient
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

	customFieldsManager = object.NewCustomFieldsManager(vSphereClient.client)

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
}
