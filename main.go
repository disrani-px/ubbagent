package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	httplib "net/http"
	"os"
	"os/signal"
	"ubbagent/app"
	"ubbagent/config"
	"ubbagent/http"
	"ubbagent/persistence"
)

var configPath = flag.String("config", "", "configuration file")
var stateDir = flag.String("state-dir", "", "persistent state directory")
var noState = flag.Bool("no-state", false, "do not store persistent state")
var localPort = flag.Int("local-port", 0, "local HTTP daemon port")

// main is the entry point to the standalone agent. It constructs a new app.App with the config file
// specified using the --config flag, and it starts the http interface. SIGINT will initiate a
// graceful shutdown.
func main() {
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "configuration file must be specified")
		flag.Usage()
		os.Exit(2)
	}

	if *stateDir == "" && !*noState {
		fmt.Fprintln(os.Stderr, "state directory must be specified (or use --no-state)")
		flag.Usage()
		os.Exit(2)
	}

	if *localPort == 0 {
		fmt.Fprintln(os.Stderr, "local-port must be > 0")
		flag.Usage()
		os.Exit(2)
	}

	cfg := loadConfig(*configPath)
	var p persistence.Persistence
	if *noState {
		p = persistence.NewMemoryPersistence()
	} else {
		var err error
		p, err = persistence.NewDiskPersistence(*stateDir)
		if err != nil {
			exitf("startup: %+v", err)
		}
	}

	a, err := app.NewApp(cfg, p)
	if err != nil {
		exitf("startup: %+v", err)
	}

	rest := http.NewHttpInterface(a.Aggregator, *localPort)
	if err := rest.Start(func(err error) {
		// Process async http errors (which may be an immediate port in use error).
		if err != httplib.ErrServerClosed {
			exitf("http: %+v", err)
		}
	}); err != nil {
		exitf("startup: %+v", err)
	}

	infof("Listening locally on port %v", *localPort)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	infof("Shutting down...")
	rest.Shutdown()
	a.Shutdown()
	glog.Flush()
}

// infof prints a message to stdout and also logs it to the INFO log.
func infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	glog.Info(msg)
}

// exitf prints a message to stderr, logs it to the FATAL log, and exits.
func exitf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, msg)
	glog.Exit(msg)
}

func loadConfig(path string) *config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		exitf("invalid configuration file: %+v", err)
	}
	if err := cfg.Validate(); err != nil {
		exitf("invalid configuration file: %+v", err)
	}
	return cfg
}