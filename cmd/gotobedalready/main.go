package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cadmium228/GoToBedAlready/internal/config"
	"github.com/cadmium228/GoToBedAlready/internal/desktop"
	"github.com/cadmium228/GoToBedAlready/internal/energy"
	"github.com/cadmium228/GoToBedAlready/internal/httpui"
	"github.com/cadmium228/GoToBedAlready/internal/monitor"
	"github.com/cadmium228/GoToBedAlready/internal/platform"
	"github.com/cadmium228/GoToBedAlready/internal/storage"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		desktop.ReportError(err.Error())
		os.Exit(1)
	}
}

func run() error {
	settings, err := config.Parse(os.Args[1:], os.Stdout)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" && !settings.Demo {
		return errors.New("Windows 10/11 required; use -demo for a safe preview")
	}
	windows := platform.WindowsObserver{Script: filepath.Join(settings.Assets, "scripts", "observe.ps1")}
	var observer monitor.Observer = windows
	var power monitor.PowerController = platform.WindowsPower{}
	if settings.Demo {
		demo := &platform.Demo{}
		observer, power = demo, platform.DisabledPower{}
		settings.Data += ".demo"
	}
	service, err := monitor.New(observer, power, storage.JSON{Path: settings.Data}, 3*time.Second)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", settings.Listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	token := hex.EncodeToString(secret)
	address := listener.Addr().String()
	url := "http://" + address + "/#" + token
	var device energy.Device
	if !settings.Demo {
		device = energy.NewDevice()
	}
	energyManager := energy.New(device, service)
	handler := httpui.New(service, httpui.Options{Assets: settings.Assets, Token: token, Host: address, Demo: settings.Demo, Version: version, Energy: energyManager, PowerSettings: service})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		service.Run(ctx)
	}()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()
	fmt.Printf("GoToBedAlready %s\n%s\nCtrl+C: exit. Closing the desktop window cancels monitoring.\n", version, url)
	if settings.Demo {
		fmt.Println("DEMO: shutdown is disabled; progress is simulated.")
	}

	energyDone := make(chan struct{})
	go func() { defer close(energyDone); energyManager.Run(ctx) }()
	windowErrors := make(chan error, 1)
	if settings.OpenBrowser && runtime.GOOS == "windows" {
		go func() { windowErrors <- desktop.Run(ctx, settings.Assets, url) }()
	}

	select {
	case <-ctx.Done():
	case err = <-serverErrors:
	case err = <-windowErrors:
	}
	stop()
	service.Cancel()
	<-monitorDone
	<-energyDone
	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownContext)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return shutdownErr
}
