package config

import ()

type Provider struct {
	Name                 string `yaml:"name"`
	URL                  string `yaml:"url"`
	ApiUsername          string `yaml:"api_username"`
	ApiPassword          string `yaml:"api_password"`
	MaxPodLimit          int    `yaml:"max_pod_limit"`
	DefaultNetworkID     string `yaml:"default_network_id"`
	CompetitionNetworkID string `yaml:"competition_network_id"`

	VCenter VCenter `yaml:"vcenter"`
}

type VCenter struct {
	CloneRole                  string
	CustomCloneRole            string
	Datacenter                 string
	Datastore                  string
	DefaultWanPortGroup        string
	DestinationFolder          string
	EndingPortGroup            int
	MainDistributedSwitch      string
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
	CompetitionEndPortGroup    int
	CompetitionResourcePool    string
	CompetitionStartPortGroup  int
	CompetitionWanPortGroup    string
}
