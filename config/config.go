package config

import (
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var (
	configErrors = []string{}
)

type Config struct {
	Domain        string
	Fqdn          string
	LogPath       string
	Port          int

	LdapConfig    LdapConfig
	VCenterConfig VCenterConfig
}

type VCenterConfig struct {
	CloneRole                  string
	CustomCloneRole            string
	Datacenter                 string
	Datastore                  string
	DefaultWanPortGroup        string
    DestinationFolder          string
	EndingPortGroup            int
	MainDistributedSwitch      string
	MaxPodLimit                int
	NattedRouterPath           string
	PortGroupSuffix            string
	PresetTemplateResourcePool string
	RouterPassword             string
	RouterPath                 string
	RouterProgram              string
	RouterProgramArgs          string
	RouterUsername             string
	StartingPortGroup          int
	TargetResourcePool         string
	TemplateFolder             string
	VCenterPassword            string
	VCenterURL                 string
	VCenterUsername            string
    CompetitionEndPortGroup    int
    CompetitionNetworkID       string
    CompetitionResourcePool    string
    CompetitionStartPortGroup  int
    CompetitionWanPortGroup    string
    DefaultNetworkID           string
}

type LdapConfig struct {
	BaseDN        string
	BindDN        string
	BindPassword  string
	GroupDN       string
	InsecureTLS   bool
	URL           string
	UserAttribute string
	UsersDN       string
}

/*
Load config settings into given config object
*/
func ReadConfigFromEnv(conf *Config) error {
	startPG, err := strconv.Atoi(os.Getenv("STARTING_PORT_GROUP"))
	if err != nil {
		log.Fatalln("Error converting STARTING_PORT_GROUP to int")
		return err
	}
	endPG, err := strconv.Atoi(os.Getenv("ENDING_PORT_GROUP"))
	if err != nil {
		log.Fatalln("Error converting ENDING_PORT_GROUP to int")
		return err
	}
    competitionStartPG, err := strconv.Atoi(os.Getenv("COMPETITION_STARTING_PORT_GROUP"))
    if err != nil {
        log.Fatalln("Error converting COMPETITION_STARTING_PORT_GROUP to int")
        return err
    }
    competitionEndPG, err := strconv.Atoi(os.Getenv("COMPETITION_ENDING_PORT_GROUP"))
    if err != nil {
        log.Fatalln("Error converting COMPETITION_ENDING_PORT_GROUP to int")
        return err
    }
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		log.Fatalln("Error converting PORT to int")
		return err
	}
	podLimit, err := strconv.Atoi(os.Getenv("MAX_POD_LIMIT"))
	if err != nil {
		log.Fatalln("Error converting MAX_POD_LIMIT to int")
		return err
	}
    _, ipNet, err := net.ParseCIDR(os.Getenv("DEFAULT_NETWORK_ID"))
    if err != nil {
        log.Fatalln("Error converting DEFAULT_NETWORK_ID to net.IPNet")
        return err
    }
    _, ipNetCompetition, err := net.ParseCIDR(os.Getenv("COMPETITION_NETWORK_ID"))
    if err != nil {
        log.Fatalln("Error converting COMPETITION_NETWORK_ID to net.IPNet")
        return err
    }
    ipNetStr := ipNet.String()
    ipNetCompetitionStr := ipNetCompetition.String()


	conf.Port = port
	conf.Domain = os.Getenv("DOMAIN")
	conf.Fqdn = os.Getenv("FQDN")
	conf.LogPath = os.Getenv("LOG_PATH")

	conf.VCenterConfig.VCenterPassword = strings.TrimSpace(os.Getenv("VCENTER_PASSWORD"))
	conf.VCenterConfig.VCenterURL = os.Getenv("VCENTER_URL")
	conf.VCenterConfig.VCenterUsername = strings.TrimSpace(os.Getenv("VCENTER_USERNAME"))

	conf.VCenterConfig.CloneRole = os.Getenv("CLONE_ROLE")
	conf.VCenterConfig.CustomCloneRole = os.Getenv("CUSTOM_CLONE_ROLE")
	conf.VCenterConfig.Datacenter = os.Getenv("DATACENTER")
	conf.VCenterConfig.Datastore = os.Getenv("DATASTORE")
	conf.VCenterConfig.DefaultWanPortGroup = os.Getenv("DEFAULT_WAN_PORT_GROUP")
    conf.VCenterConfig.DestinationFolder = os.Getenv("DESTINATION_FOLDER")
	conf.VCenterConfig.EndingPortGroup = endPG
	conf.VCenterConfig.MainDistributedSwitch = os.Getenv("MAIN_DISTRIBUTED_SWITCH")
	conf.VCenterConfig.MaxPodLimit = podLimit
	conf.VCenterConfig.NattedRouterPath = os.Getenv("NATTED_ROUTER_PATH")
	conf.VCenterConfig.PortGroupSuffix = os.Getenv("PORT_GROUP_SUFFIX")
	conf.VCenterConfig.PresetTemplateResourcePool = os.Getenv("PRESET_TEMPLATE_RESOURCE_POOL")
	conf.VCenterConfig.RouterPassword = os.Getenv("ROUTER_PASSWORD")
	conf.VCenterConfig.RouterPath = os.Getenv("ROUTER_PATH")
	conf.VCenterConfig.RouterProgram = os.Getenv("ROUTER_PROGRAM")
	conf.VCenterConfig.RouterProgramArgs = os.Getenv("ROUTER_PROGRAM_ARGS")
	conf.VCenterConfig.RouterUsername = os.Getenv("ROUTER_USERNAME")
	conf.VCenterConfig.StartingPortGroup = startPG
	conf.VCenterConfig.TargetResourcePool = os.Getenv("TARGET_RESOURCE_POOL")
	conf.VCenterConfig.TemplateFolder = os.Getenv("TEMPLATE_FOLDER")
    conf.VCenterConfig.CompetitionEndPortGroup = competitionEndPG
    conf.VCenterConfig.CompetitionNetworkID = ipNetCompetitionStr
    conf.VCenterConfig.CompetitionResourcePool = os.Getenv("COMPETITION_RESOURCE_POOL")
    conf.VCenterConfig.CompetitionStartPortGroup = competitionStartPG
    conf.VCenterConfig.CompetitionWanPortGroup = os.Getenv("COMPETITION_WAN_PORT_GROUP")
    conf.VCenterConfig.DefaultNetworkID = ipNetStr

	conf.LdapConfig.BaseDN = os.Getenv("LDAP_BASE_DN")
	conf.LdapConfig.BindDN = os.Getenv("LDAP_BIND_DN")
	conf.LdapConfig.BindPassword = os.Getenv("LDAP_BIND_PASSWORD")
	conf.LdapConfig.GroupDN = os.Getenv("LDAP_GROUP_DN")
	conf.LdapConfig.InsecureTLS, err = strconv.ParseBool(os.Getenv("LDAP_INSECURE_TLS"))
	conf.LdapConfig.URL = os.Getenv("LDAP_URL")
	conf.LdapConfig.UserAttribute = os.Getenv("LDAP_USER_ATTRIBUTE")
	conf.LdapConfig.UsersDN = os.Getenv("LDAP_USERS_DN")
	if err != nil {
		log.Println("Error converting LDAP_INSECURE_TLS to bool")
	}

	return nil
}

/*
Check for config errors and set defaults
*/
func CheckConfig(conf *Config) error {
	if conf.VCenterConfig.VCenterURL == "" {
		return errors.New("illegal config: vCenterURL must be defined")
	}
	if conf.VCenterConfig.VCenterUsername == "" {
		return errors.New("illegal config: vCenterUsername must be defined")
	}
	if conf.VCenterConfig.VCenterPassword == "" {
		return errors.New("illegal config: vCenterPassword must be defined")
	}
	if conf.VCenterConfig.Datacenter == "" {
        return errors.New("illegal config: Datacenter must be defined")
	}
	if conf.VCenterConfig.PresetTemplateResourcePool == "" {
        conf.VCenterConfig.PresetTemplateResourcePool = "07-01_Templates"
	}
	if conf.VCenterConfig.MainDistributedSwitch == "" {
        conf.VCenterConfig.MainDistributedSwitch = "Main_DSW"
	}

	if conf.VCenterConfig.StartingPortGroup == 0 || conf.VCenterConfig.EndingPortGroup == 0 {
        conf.VCenterConfig.StartingPortGroup = 1801
        conf.VCenterConfig.EndingPortGroup = 2000
	}
	if conf.Port == 0 {
		conf.Port = 8080
	}

	if conf.VCenterConfig.MaxPodLimit == 0 {
        conf.VCenterConfig.MaxPodLimit = 5
	}

	if conf.Fqdn == "" {
		return errors.New("illegal config: Must set FQDN")
	}

    if conf.Domain == "" {
        return errors.New("illegal config: Must set Domain")
    }

    if conf.LogPath == "" {
        conf.LogPath = "/var/log/goclone"
    }

    if conf.VCenterConfig.CompetitionStartPortGroup == 0 || conf.VCenterConfig.CompetitionEndPortGroup == 0 {
        conf.VCenterConfig.CompetitionStartPortGroup = 2001
        conf.VCenterConfig.CompetitionEndPortGroup = 2200
    }

    if conf.VCenterConfig.CompetitionNetworkID == "" {
        conf.VCenterConfig.CompetitionNetworkID = "172.26.0.0/16"
    }

    if conf.VCenterConfig.CompetitionResourcePool == "" {
        conf.VCenterConfig.CompetitionResourcePool = "07-03_CompetitionPods"
    }

    if conf.VCenterConfig.CompetitionWanPortGroup == "" {
        conf.VCenterConfig.CompetitionWanPortGroup = "0041_RvB2CoreNetwork"
    }

    if conf.VCenterConfig.DefaultNetworkID == "" {
        conf.VCenterConfig.DefaultNetworkID = "172.16.0.0/16"
    }

    if conf.VCenterConfig.DefaultWanPortGroup == "" {
        conf.VCenterConfig.DefaultWanPortGroup = "0040_RvBCoreNetwork"
    }

    if conf.VCenterConfig.DestinationFolder == "" {
        return errors.New("illegal config: DestinationFolder must be defined")
    }

    if conf.VCenterConfig.Datastore == "" {
        return errors.New("illegal config: Datastore must be defined")
    }

    if conf.VCenterConfig.NattedRouterPath == "" {
        return errors.New("illegal config: NattedRouterPath must be defined")
    }

    if conf.VCenterConfig.RouterPath == "" {
        return errors.New("illegal config: RouterPath must be defined")
    }

    if conf.VCenterConfig.RouterProgram == "" {
        return errors.New("illegal config: RouterProgram must be defined")
    }

    if conf.VCenterConfig.RouterProgramArgs == "" {
        return errors.New("illegal config: RouterProgramArgs must be defined")
    }

    if conf.VCenterConfig.TargetResourcePool == "" {
        return errors.New("illegal config: TargetResourcePool must be defined")
    }

    if conf.VCenterConfig.TemplateFolder == "" {
        return errors.New("illegal config: TemplateFolder must be defined")
    }

    if conf.VCenterConfig.PortGroupSuffix == "" {
        conf.VCenterConfig.PortGroupSuffix = "PodNetwork"
    }

    if conf.VCenterConfig.RouterUsername == "" {
        return errors.New("illegal config: RouterUsername must be defined")
    }

    if conf.VCenterConfig.RouterPassword == "" {
        return errors.New("illegal config: RouterPassword must be defined")
    }

    if conf.VCenterConfig.CloneRole == "" {
        return errors.New("illegal config: CloneRole must be defined")
    }

    if conf.VCenterConfig.CustomCloneRole == "" {
        return errors.New("illegal config: CustomCloneRole must be defined")
    }

    if conf.LdapConfig.BaseDN == "" {
        return errors.New("illegal config: LDAP_BASE_DN must be defined")
    }

    if conf.LdapConfig.BindDN == "" {
        return errors.New("illegal config: LDAP_BIND_DN must be defined")
    }

    if conf.LdapConfig.BindPassword == "" {
        return errors.New("illegal config: LDAP_BIND_PASSWORD must be defined")
    }

    if conf.LdapConfig.GroupDN == "" {
        return errors.New("illegal config: LDAP_GROUP_DN must be defined")
    }

    if conf.LdapConfig.URL == "" {
        return errors.New("illegal config: LDAP_URL must be defined")
    }

    if conf.LdapConfig.UserAttribute == "" {
        return errors.New("illegal config: LDAP_USER_ATTRIBUTE must be defined")
    }

    if conf.LdapConfig.UsersDN == "" {
        return errors.New("illegal config: LDAP_USERS_DN must be defined")
    }

	return nil
}
