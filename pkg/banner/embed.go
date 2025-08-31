package banner

import (
	"embed"
	"os"
	"strconv"
	"text/template"

	"hertzbeat.apache.org/hertzbeat-collector-go/pkg/collector/internel"
)

//go:embed banner.txt
var EmbedLogo embed.FS

type Banner struct {
	server *internel.CollectorServer
}

func New(server *internel.CollectorServer) *Banner {
	return &Banner{server: server}
}

type bannerVars struct {
	CollectorName string
	ServerPort    string
	Pid           string
}

func (b *Banner) PrintBanner(appName, port string) error {
	data, err := EmbedLogo.ReadFile("banner.txt")
	if err != nil {
		b.server.Logger.Error(err, "read banner file failed")
		return err
	}

	tmpl, err := template.New("banner").Parse(string(data))
	if err != nil {
		b.server.Logger.Error(err, "parse banner template failed")
		return err
	}

	vars := bannerVars{
		CollectorName: appName,
		ServerPort:    port,
		Pid:           strconv.Itoa(os.Getpid()),
	}

	err = tmpl.Execute(os.Stdout, vars)
	if err != nil {
		return err
	}

	return nil
}
