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
    Domain             string `yaml:"domain"`

    VCenter VCenter `yaml:"vcenter"`
}

type VCenter struct {
    CloneRole                  string `yaml:"clone_role"`
    CustomCloneRole            string `yaml:"custom_clone_role"`
    Datacenter                 string `yaml:"datacenter"`
    Datastore                  string `yaml:"datastore"`
    DefaultWanPortGroup        string `yaml:"default_wan_port_group"`
    DestinationFolder          string `yaml:"destination_folder"`
    EndingPortGroup            int    `yaml:"ending_port_group"`
    MainDistributedSwitch      string `yaml:"main_distributed_switch"`
    NattedRouterPath           string `yaml:"natted_router_path"`
    PortGroupSuffix            string `yaml:"port_group_suffix"`
    PresetTemplateResourcePool string `yaml:"preset_template_resource_pool"`
    RouterPassword             string `yaml:"router_password"`
    RouterPath                 string `yaml:"router_path"`
    RouterProgram              string `yaml:"router_program"`
    RouterProgramArgs          string `yaml:"router_program_args"`
    RouterUsername             string `yaml:"router_username"`
    StartingPortGroup          int    `yaml:"starting_port_group"`
    TargetResourcePool         string `yaml:"target_resource_pool"`
    TemplateFolder             string `yaml:"template_folder"`
    CompetitionEndPortGroup    int   `yaml:"competition_end_port_group"`
    CompetitionResourcePool    string `yaml:"competition_resource_pool"`
    CompetitionStartPortGroup  int    `yaml:"competition_start_port_group"`
    CompetitionWanPortGroup    string `yaml:"competition_wan_port_group"`
}
