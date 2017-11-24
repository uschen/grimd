package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"time"

	"cloud.google.com/go/logging"
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
	grimdActivation ActivationHandler

	// BlockCache contains all blocked domains
	BlockCache = &MemoryBlockCache{Backend: make(map[string]bool)}

	// QuestionCache contains all queries to the dns server
	QuestionCache = &MemoryQuestionCache{Backend: make([]QuestionCacheEntry, 0), Maxcount: 1000}
)

func main() {

	flag.Parse()

	if err := LoadConfig(configPath); err != nil {
		log.Fatal(err)
	}

	QuestionCache.Maxcount = Config.QuestionCacheCap

	logFile, err := LoggerInit(Config.Log)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	l := zap.NewAtomicLevel()
	if err := l.UnmarshalText([]byte("info")); err != nil {
		log.Fatal(err)
	}

	ops := []option.ClientOption{}
	if Config.LogGCPKeyPath != "" {
		ops = append(ops, option.WithServiceAccountFile(Config.LogGCPKeyPath))
	}
	clogClient, err := logging.NewClient(context.Background(), Config.LogGCPProject, ops...)
	if err != nil {
		log.Fatal(err)
	}
	defer clogClient.Close()
	cLogger := clogClient.Logger(Config.LogID,
		logging.CommonLabels(map[string]string{"lang": "go"}),
		logging.CommonResource(&mrpb.MonitoredResource{
			Type: "global",
			Labels: map[string]string{
				"project_id": Config.LogGCPProject,
			},
		}),
	)
	defer cLogger.Flush()
	core, err := zapstackdriver.New(l, cLogger)
	if err != nil {
		log.Fatal(err)
	}
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	// delay updating the blocklists, cache until the server starts and can serve requests as the local resolver
	timer := time.NewTimer(time.Second * 1)
	go func() {
		<-timer.C
		if _, err := os.Stat("lists"); os.IsNotExist(err) || forceUpdate {
			if err := Update(); err != nil {
				log.Fatal(err)
			}
		}
		if err := UpdateBlockCache(); err != nil {
			log.Fatal(err)
		}
	}()

	grimdActive = true
	quitActivation := make(chan bool)
	go grimdActivation.loop(quitActivation)

	server := &Server{
		host:     Config.Bind,
		rTimeout: 5 * time.Second,
		wTimeout: 5 * time.Second,
		logger:   logger,
	}

	server.Run()

	if err := StartAPIServer(); err != nil {
		log.Fatal(err)
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt)

forever:
	for {
		select {
		case <-sig:
			log.Printf("signal received, stopping\n")
			quitActivation <- true
			break forever
		}
	}
}

func init() {
	flag.StringVar(&configPath, "config", "grimd.toml", "location of the config file, if not found it will be generated (default grimd.toml)")
	flag.BoolVar(&forceUpdate, "update", false, "force an update of the blocklist database")

	runtime.GOMAXPROCS(runtime.NumCPU())
}
