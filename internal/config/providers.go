package config

type Provider struct {
	Name                 string `mapstructure:"name"`
	URL                  string `mapstructure:"url"`
	Username          string `mapstructure:"username"`
	Password          string `mapstructure:"password"`
	MaxPodLimit          int    `mapstructure:"max_pod_limit"`
	DefaultNetworkID     string `mapstructure:"default_network_id"`
	CompetitionNetworkID string `mapstructure:"competition_network_id"`
    Domain             string `mapstructure:"domain"`

	VCenter VCenter `mapstructure:"vcenter"`
}

type VCenter struct {
    CloneRole                  string `mapstructure:"clone_role"`
    CustomCloneRole            string `mapstructure:"custom_clone_role"`
    Datacenter                 string `mapstructure:"datacenter"`
    Datastore                  string `mapstructure:"datastore"`
    DefaultWanPortGroup        string `mapstructure:"default_wan_port_group"`
    DestinationFolder          string `mapstructure:"destination_folder"`
    EndingPortGroup            int    `mapstructure:"ending_port_group"`
    MainDistributedSwitch      string `mapstructure:"main_distributed_switch"`
    NattedRouterPath           string `mapstructure:"natted_router_path"`
    PortGroupSuffix            string `mapstructure:"port_group_suffix"`
    PresetTemplateResourcePool string `mapstructure:"preset_template_resource_pool"`
    RouterPassword             string `mapstructure:"router_password"`
    RouterPath                 string `mapstructure:"router_path"`
    RouterProgram              string `mapstructure:"router_program"`
    RouterProgramArgs          string `mapstructure:"router_program_args"`
    RouterUsername             string `mapstructure:"router_username"`
    StartingPortGroup          int    `mapstructure:"starting_port_group"`
    TargetResourcePool         string `mapstructure:"target_resource_pool"`
    TemplateFolder             string `mapstructure:"template_folder"`
    CompetitionEndPortGroup    int   `mapstructure:"competition_end_port_group"`
    CompetitionResourcePool    string `mapstructure:"competition_resource_pool"`
    CompetitionStartPortGroup  int    `mapstructure:"competition_start_port_group"`
    CompetitionWanPortGroup    string `mapstructure:"competition_wan_port_group"`
}
