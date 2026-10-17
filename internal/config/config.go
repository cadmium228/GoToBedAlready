package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

type Config struct {
	Listen      string
	Assets      string
	Data        string
	Demo        bool
	OpenBrowser bool
}

func Parse(arguments []string, output io.Writer) (Config, error) {
	executable, err := os.Executable()
	if err != nil {
		return Config{}, err
	}
	data, err := os.UserConfigDir()
	if err != nil {
		return Config{}, err
	}
	var config Config
	flags := flag.NewFlagSet("GoToBedAlready", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&config.Listen, "listen", "127.0.0.1:0", "HTTP address; non-loopback is allowed only in demo mode")
	flags.StringVar(&config.Assets, "assets", filepath.Dir(executable), "directory containing web/ and scripts/")
	flags.StringVar(&config.Data, "data", filepath.Join(data, "GoToBedAlready", "settings.json"), "settings file")
	flags.BoolVar(&config.Demo, "demo", false, "simulate progress; never shut down this computer")
	flags.BoolVar(&config.OpenBrowser, "open", true, "open the interface in the default browser")
	if err := flags.Parse(arguments); err != nil {
		return Config{}, err
	}
	if flags.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected positional arguments")
	}
	host, _, err := net.SplitHostPort(config.Listen)
	if err != nil {
		return Config{}, err
	}
	ip := net.ParseIP(host)
	if !config.Demo && (ip == nil || !ip.IsLoopback()) {
		return Config{}, fmt.Errorf("real shutdown mode requires a loopback IP address")
	}
	for _, path := range []string{"web/index.html", "web/app.js", "web/style.css", "scripts/observe.ps1"} {
		if _, err := os.Stat(filepath.Join(config.Assets, path)); err != nil {
			return Config{}, fmt.Errorf("missing asset %s: %w", path, err)
		}
	}
	return config, nil
}
