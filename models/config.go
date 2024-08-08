package models

import (
	"log"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"
)

var (
	configErrors = []string{}
)

type Config struct {
	Https      bool   `toml:"Https"`
	Port       int    `toml:"port"`
	Cert       string `toml:"Cert"`
	Key        string `toml:"Key"`
	Domain     string `toml:"Domain"`
	LogPath    string `toml:"LogPath"`
	DomainName string `toml:"DomainName"`

	VCenterConfig VCenterConfig `toml:"VCenterConfig"`
	LdapConfig    LdapConfig    `toml:"LdapConfig"`
}

type VCenterConfig struct {
	VCenterURL                 string `toml:"VCenterURL"`
	VCenterUsername            string `toml:"VCenterUsername"`
	VCenterPassword            string `toml:"VCenterPassword"`
	Datacenter                 string `toml:"Datacenter"`
	Datastore                  string `toml:"Datastore"`
	PresetTemplateResourcePool string `toml:"PresetTemplateResourcePool"`
	StartingPortGroup          int    `toml:"StartingPortGroup"`
	EndingPortGroup            int    `toml:"EndingPortGroup"`
	TargetResourcePool         string `toml:"TargetResourcePool"`
	WanPortGroup               string `toml:"WanPortGroup"`
	MaxPodLimit                int    `toml:"MaxPodLimit"`
	MainDistributedSwitch      string `toml:"MainDistributedSwitch"`
	TemplateFolder             string `toml:"TemplateFolder"`
	PortGroupSuffix            string `toml:"PortGroupSuffix"`
	NattedRouterPath           string `toml:"NattedRouterPath"`
	RouterPath                 string `toml:"RouterPath"`
	RouterUsername             string `toml:"RouterUsername"`
	RouterPassword             string `toml:"RouterPassword"`
	RouterProgram              string `toml:"RouterProgram"`
	RouterProgramArgs          string `toml:"RouterProgramArgs"`
}

type LdapConfig struct {
	LdapAdminDN       string `toml:"LdapAdminDN"`
	LdapAdminPassword string `toml:"LdapAdminPassword"`
	LdapUri           string `toml:"LdapUri"`
}

/*
Load config settings into given config object
*/
func ReadConfig(conf *Config, configPath string) {
	fileContent, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalln("Configuration file ("+configPath+") not found:", err)
	}
	if md, err := toml.Decode(string(fileContent), &conf); err != nil {
		log.Fatalln(err)
	} else {
		for _, undecoded := range md.Undecoded() {
			errMsg := "[WARN] Undecoded scoring configuration key \"" + undecoded.String() + "\" will not be used."
			configErrors = append(configErrors, errMsg)
			log.Println(errMsg)
		}
	}
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
		if conf.Https {
			conf.Port = 443
		} else {
			conf.Port = 80
		}
	}

	if conf.Https {
		if conf.Cert == "" || conf.Key == "" {
			return errors.New("illegal config: https requires a cert and key pair")
		}
	}

	if conf.VCenterConfig.MaxPodLimit == 0 {
		return errors.New("illegal config: MaxPodLimit must be more than 0")
	}

	if conf.DomainName == "" {
		return errors.New("illegal config: Must set DomainName")
	}

	return nil
}
