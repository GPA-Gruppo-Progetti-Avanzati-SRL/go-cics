package cicsservice

type ConnectionConfig struct {
	Hostname             string `mapstructure:"hostname"`
	Port                 int    `mapstructure:"port"`
	Timeout              int    `mapstructure:"timeout"`
	UserName             string `mapstructure:"username"`
	Password             string `mapstructure:"password"`
	ServerName           string `mapstructure:"servername"`
	ProxyPort            int    `mapstructure:"proxyport"`
	InsecureSkipVerify   bool   `mapstructure:"skv"`
	UseProxy             bool   `mapstructure:"useproxy"`
	MaxTotal             int    `mapstructure:"connectionnumber"`
	MaxIdle              int    `mapstructure:"maxidle"`
	MinIdle              int    `mapstructure:"minidle"`
	MaxIdleLifeTime      int    `mapstructure:"maxidlelifetime"`
	SSLRootCaCertificate string `mapstructure:"sslrootcacertificate"`
	SSLClientKey         string `mapstructure:"sslclientkey"`
	SSLClientCertificate string `mapstructure:"sslclientcertificate"`
}

type RoutineConfig struct {
	Name            string `mapstructure:"name"`
	ChannelName     string `mapstructure:"channelname"`
	ProgramName     string `mapstructure:"programname"`
	CicsGatewayName string `mapstructure:"cicsgatewayname"`
	TransId         string `mapstructure:"transid"`
}
