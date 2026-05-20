package common

const (
	StealthDnsIp        = "127.0.0.1"
	DnsUdpPort          = 53
	NhpDomainNameSuffix = ".nhp"
	DefaultUpstreamDNS  = "8.8.8.8"
)

const (
	Type_A    = 1  // DNS A record type (RFC 1035)
	Type_AAAA = 28 // DNS AAAA record type (RFC 3596)
)

const (
	AgentInit          = "nhp_agent_init"
	AgentClose         = "nhp_agent_close"
	AgentFreeCString   = "nhp_free_cstring"
	AgentKnockResource = "nhp_agent_knock_resource"
)
