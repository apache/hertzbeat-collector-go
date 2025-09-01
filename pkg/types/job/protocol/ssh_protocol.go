package protocol

type SshProtocol struct {
	Host                 string
	Port                 string
	Timeout              string
	Username             string
	Password             string
	PrivateKey           string
	PrivateKeyPassphrase string
	ReuseConnection      string
	Script               string
	ParseType            string
	ProxyHost            string
	ProxyPort            string
	ProxyUsername        string
	ProxyPassword        string
	UseProxy             string
	ProxyPrivateKey      string
}

type SshProtocolConfigOptFunc func(option *SshProtocol)

func NewSshProtocol(host, port string, opts ...SshProtocolConfigOptFunc) *SshProtocol {

	option := &SshProtocol{
		Host: host,
		Port: port,
	}

	for _, opt := range opts {
		opt(option)
	}

	return &SshProtocol{
		Host:                 host,
		Port:                 port,
		Timeout:              option.Timeout,
		Username:             option.Username,
		Password:             option.Password,
		PrivateKey:           option.PrivateKey,
		PrivateKeyPassphrase: option.PrivateKeyPassphrase,
		ReuseConnection:      option.ReuseConnection,
		Script:               option.Script,
		ParseType:            option.ParseType,
		ProxyHost:            option.ProxyHost,
		ProxyPort:            option.ProxyPort,
		ProxyUsername:        option.ProxyUsername,
		ProxyPassword:        option.ProxyPassword,
		UseProxy:             option.UseProxy,
		ProxyPrivateKey:      option.ProxyPrivateKey,
	}
}

func (sp *SshProtocol) IsInvalid() error {

	return nil
}
