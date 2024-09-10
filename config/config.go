package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var (
	configErrors = []string{}
)

type Config struct {
	Port    int
	Domain  string
	LogPath string
	Fqdn    string

	VCenterConfig VCenterConfig
	LdapConfig    LdapConfig
}

type VCenterConfig struct {
	VCenterURL                 string
	VCenterUsername            string
	VCenterPassword            string
	Datacenter                 string
	Datastore                  string
	PresetTemplateResourcePool string
	StartingPortGroup          int
	EndingPortGroup            int
	TargetResourcePool         string
	WanPortGroup               string
	MaxPodLimit                int
	MainDistributedSwitch      string
	TemplateFolder             string
	PortGroupSuffix            string
	CloneRole                  string
	CustomCloneRole            string
	NattedRouterPath           string
	RouterPath                 string
	RouterUsername             string
	RouterPassword             string
	RouterProgram              string
	RouterProgramArgs          string
}

type LdapConfig struct {
	BindDN        string
	BindPassword  string
	URL           string
	BaseDN        string
    UsersDN       string
	InsecureTLS   bool
	UserAttribute string
	GroupDN       string
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

	conf.Port = port
	conf.Domain = os.Getenv("DOMAIN")
	conf.Fqdn = os.Getenv("FQDN")
	conf.LogPath = os.Getenv("LOG_PATH")

	conf.VCenterConfig.VCenterURL = os.Getenv("VCENTER_URL")
	conf.VCenterConfig.VCenterUsername = strings.TrimSpace(os.Getenv("VCENTER_USERNAME"))
	conf.VCenterConfig.VCenterPassword = strings.TrimSpace(os.Getenv("VCENTER_PASSWORD"))

	conf.VCenterConfig.Datacenter = os.Getenv("DATACENTER")
	conf.VCenterConfig.Datastore = os.Getenv("DATASTORE")
	conf.VCenterConfig.PresetTemplateResourcePool = os.Getenv("PRESET_TEMPLATE_RESOURCE_POOL")
	conf.VCenterConfig.StartingPortGroup = startPG
	conf.VCenterConfig.EndingPortGroup = endPG
	conf.VCenterConfig.TargetResourcePool = os.Getenv("TARGET_RESOURCE_POOL")
	conf.VCenterConfig.WanPortGroup = os.Getenv("WAN_PORT_GROUP")
	conf.VCenterConfig.MaxPodLimit = podLimit
	conf.VCenterConfig.MainDistributedSwitch = os.Getenv("MAIN_DISTRIBUTED_SWITCH")
	conf.VCenterConfig.TemplateFolder = os.Getenv("TEMPLATE_FOLDER")
	conf.VCenterConfig.PortGroupSuffix = os.Getenv("PORT_GROUP_SUFFIX")
	conf.VCenterConfig.CloneRole = os.Getenv("CLONE_ROLE")
	conf.VCenterConfig.CustomCloneRole = os.Getenv("CUSTOM_CLONE_ROLE")
	conf.VCenterConfig.NattedRouterPath = os.Getenv("NATTED_ROUTER_PATH")
	conf.VCenterConfig.RouterPath = os.Getenv("ROUTER_PATH")
	conf.VCenterConfig.RouterUsername = os.Getenv("ROUTER_USERNAME")
	conf.VCenterConfig.RouterPassword = os.Getenv("ROUTER_PASSWORD")
	conf.VCenterConfig.RouterProgram = os.Getenv("ROUTER_PROGRAM")
	conf.VCenterConfig.RouterProgramArgs = os.Getenv("ROUTER_PROGRAM_ARGS")

	conf.LdapConfig.BindDN = os.Getenv("LDAP_BIND_DN")
	conf.LdapConfig.BindPassword = os.Getenv("LDAP_BIND_PASSWORD")
	conf.LdapConfig.URL = os.Getenv("LDAP_URL")
	conf.LdapConfig.BaseDN = os.Getenv("LDAP_BASE_DN")
    conf.LdapConfig.UsersDN = os.Getenv("LDAP_USERS_DN")
	conf.LdapConfig.UserAttribute = os.Getenv("LDAP_USER_ATTRIBUTE")
	conf.LdapConfig.GroupDN = os.Getenv("LDAP_GROUP_DN")
	conf.LdapConfig.InsecureTLS, err = strconv.ParseBool(os.Getenv("LDAP_INSECURE_TLS"))
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
		return errors.New("illegal config: PresetTemplateResourcePool must be defined")
	}
	if conf.VCenterConfig.MainDistributedSwitch == "" {
		return errors.New("illegal config: MainDistributedSwitch must be defined")
	}

	if conf.VCenterConfig.StartingPortGroup == 0 || conf.VCenterConfig.EndingPortGroup == 0 {
		return errors.New("illegal config: StartingPortGroup and EndingPortGroup must be defined")
	}
	if conf.Port == 0 {
		conf.Port = 80
	}

	if conf.VCenterConfig.MaxPodLimit == 0 {
		return errors.New("illegal config: MaxPodLimit must be more than 0")
	}

	if conf.Fqdn == "" {
		return errors.New("illegal config: Must set FQDN")
	}

	return nil
}
