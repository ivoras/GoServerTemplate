package main

import (
	"flag"
	"io"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

const (
	eventQuit = iota
)

type sysEventMessage struct {
	event int
	idata int
}

var sysEventChannel = make(chan sysEventMessage, 5)
var logOutput io.Writer
var startTime time.Time

var logFileName = flag.String("log", "/tmp/ch_registry_server.log", "Log file ('-' for only stderr)")

func main() {
	os.Setenv("TZ", "UTC")
	startTime = time.Now()
	rand.Seed(startTime.UnixNano())

	if runtime.GOOS == "windows" {
		*logFileName = "c:\\temp\\ch_registry_server.log"
	}
	flag.Parse()

	if *logFileName != "-" {
		f, err := os.OpenFile(*logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664)
		if err != nil {
			log.Panic("Cannot open log file " + *logFileName)
		}
		defer f.Close()
		logOutput = io.MultiWriter(os.Stderr, f)
	} else {
		logOutput = os.Stderr
	}
	log.SetOutput(logOutput)

	slog.Info("Starting up...")

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT)

	err := initDb()
	if err != nil {
		slog.Error("Cannot open database", "err", err)
		return
	}

	//go webServer()
	//go infraWebServer()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	oldAlloc := int64(m.Alloc)
	printMemStats(&m)

	for {
		select {
		case msg := <-sysEventChannel:
			switch msg.event {
			case eventQuit:
				slog.Info("Exiting")
				os.Exit(msg.idata)
			}
		case sig := <-sigChannel:
			switch sig {
			case syscall.SIGINT:
				sysEventChannel <- sysEventMessage{event: eventQuit, idata: 0}
				log.Println("^C detected")
			}
		case <-time.After(60 * time.Second):

			runtime.ReadMemStats(&m)
			if abs(int64(m.Alloc)-oldAlloc) > 1024*1024 {
				printMemStats(&m)
				oldAlloc = int64(m.Alloc)
			}
		case <-time.After(15 * time.Minute):
			//cleanupDb()
		}
	}
}

func printMemStats(m *runtime.MemStats) {
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	slog.Info("Stats:", "alloc", bToMB(m.Alloc), "total_alloc", bToMB(m.TotalAlloc), "sys", bToMB(m.Sys), "num_gc", m.NumGC, "uptime_hrs", time.Since(startTime).Hours())
}
