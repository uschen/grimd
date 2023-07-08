package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"cloud.google.com/go/logging"
	drblpeer "github.com/elico/drbl-peer"
	"github.com/looterz/grimd/zapstackdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/api/option"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
)

var (
	configPath      string
	forceUpdate     bool
	grimdActive     bool
	grimdActivation *ActivationHandler
	drblPeers       *drblpeer.DrblPeers
)

func reloadBlockCache(
	config *Config,
	blockCache *MemoryBlockCache,
	questionCache *MemoryQuestionCache,
	drblPeers *drblpeer.DrblPeers,
	apiServer *http.Server,
	server *Server,
	reloadChan chan bool,
) (*MemoryBlockCache, *http.Server, error) {

	logger.Debug("Reloading the blockcache")
	blockCache = PerformUpdate(config, true)
	server.Stop()
	if apiServer != nil {
		if err := apiServer.Shutdown(context.Background()); err != nil {
			logger.Debugf("error shutting down api server: %v", err)
		}
	}
	server.Run(config, blockCache, questionCache)
	apiServer, err := StartAPIServer(config, reloadChan, blockCache, questionCache)
	if err != nil {
		logger.Fatal(err)
		return nil, nil, err
	}

	if config.UseDrbl > 0 {
		drblPeers, _ = drblpeer.NewPeerListFromYamlFile(config.DrblPeersFilename, config.DrblBlockWeight, config.DrblTimeout, config.DrblDebug > 0)
		if config.DrblDebug > 0 {
			log.Println("Drbl Debug is ON")
			drblPeers.Debug = true
			log.Println("Drbl Debug is ON", drblPeers.Debug)
		}
	}

	return blockCache, apiServer, nil
}

func main() {
	flag.Parse()

	config, err := LoadConfig(configPath)
	if err != nil {
		logger.Fatal(err)
	}

	l := zap.NewAtomicLevel()
	if err := l.UnmarshalText([]byte("info")); err != nil {
		log.Fatal(err)
	}

	ops := []option.ClientOption{}
	if config.LogGCPKeyPath != "" {
		ops = append(ops, option.WithServiceAccountFile(config.LogGCPKeyPath))
	}
	clogClient, err := logging.NewClient(context.Background(), config.LogGCPProject, ops...)

	defer clogClient.Close()
	cLogger := clogClient.Logger(config.LogID,
		logging.CommonLabels(map[string]string{"lang": "go"}),
		logging.CommonResource(&mrpb.MonitoredResource{
			Type: "global",
			Labels: map[string]string{
				"project_id": config.LogGCPProject,
			},
		}),
	)
	defer cLogger.Flush()
	core, err := zapstackdriver.New(l, cLogger)
	if err != nil {
		logger.Fatal(err)
	}
	zaplogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	loggingState, err := loggerInit(config.LogConfig)
	if err != nil {
		logger.Fatal(err)
	}

	grimdActive = true
	quitActivation := make(chan bool)
	actChannel := make(chan *ActivationHandler)

	go startActivation(actChannel, quitActivation, config.ReactivationDelay)
	grimdActivation = <-actChannel
	close(actChannel)

	server := &Server{
		host:     config.Bind,
		rTimeout: 5 * time.Second,
		wTimeout: 5 * time.Second,
		logger:   zaplogger,
	}

	// BlockCache contains all blocked domains
	blockCache := &MemoryBlockCache{Backend: make(map[string]bool)}
	// QuestionCache contains all queries to the dns server
	questionCache := makeQuestionCache(config.QuestionCacheCap)

	reloadChan := make(chan bool)

	// The server will start with an empty blockcache soe we can dowload the lists if grimd is the
	// system's dns server.
	server.Run(config, blockCache, questionCache)

	var apiServer *http.Server
	// Load the block cache, restart the server with the new context
	blockCache, apiServer, err = reloadBlockCache(config, blockCache, questionCache, drblPeers, apiServer, server, reloadChan)
	if err != nil {
		logger.Fatalf("Cannot start the API server %s", err)
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, syscall.SIGHUP)

forever:
	for {
		select {
		case s := <-sig:
			switch s {
			case os.Interrupt:
				logger.Error("SIGINT received, stopping\n")
				quitActivation <- true
				break forever
			case syscall.SIGHUP:
				logger.Error("SIGHUP received: rotating logs\n")
				err := loggingState.reopen()
				if err != nil {
					logger.Error(err)
				}
			}
		case <-reloadChan:
			blockCache, apiServer, err = reloadBlockCache(config, blockCache, questionCache, drblPeers, apiServer, server, reloadChan)
			if err != nil {
				logger.Fatalf("Cannot start the API server %s", err)
			}
		}
	}
	// make sure we give the activation goroutine time to exit
	<-quitActivation
	logger.Debugf("Main goroutine exiting")
}

func init() {
	flag.StringVar(&configPath, "config", "grimd.toml", "location of the config file, if not found it will be generated (default grimd.toml)")
	flag.BoolVar(&forceUpdate, "update", false, "force an update of the blocklist database")

	runtime.GOMAXPROCS(runtime.NumCPU())
}
