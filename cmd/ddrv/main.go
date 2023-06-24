package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/joho/godotenv"

	"github.com/forscht/ddrv/internal/config"
	"github.com/forscht/ddrv/internal/dataprovider"
	"github.com/forscht/ddrv/internal/filesystem"
	"github.com/forscht/ddrv/internal/ftp"
	"github.com/forscht/ddrv/internal/http"
	"github.com/forscht/ddrv/internal/webdav"
	"github.com/forscht/ddrv/pkg/ddrv"
)

func main() {
	// Set the maximum number of operating system threads to use.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Load env file.
	_ = godotenv.Load()

	// Parse command line arguments into config
	kong.Parse(config.New(), kong.Vars{
		"version": fmt.Sprintf("ddrv %s", version),
	})

	// Make sure chunkSize is below 25MB
	if config.ChunkSize() > 25*1024*1024 || config.ChunkSize() < 0 {
		log.Fatalf("ddrv: invalid chunkSize %d", config.ChunkSize())
	}

	// Create a ddrv manager
	mgr, err := ddrv.NewManager(config.ChunkSize(), strings.Split(config.Webhooks(), ","))
	if err != nil {
		log.Fatalf("ddrv: failed to open ddrv mgr :%v", err)
	}

	// Create FS object
	fs := filesystem.New(mgr)

	// New data provider
	dataprovider.New()

	errCh := make(chan error)

	if config.FTPAddr() != "" {
		go func() {
			// Create and start ftp server
			ftpServer := ftp.New(fs)
			log.Printf("ddrv: starting FTP server on : %s", config.FTPAddr())
			errCh <- ftpServer.ListenAndServe()
		}()
	}
	if config.HTTPAddr() != "" {
		go func() {
			httpServer := http.New(mgr)
			log.Printf("ddrv: starting HTTP server on : %s", config.HTTPAddr())
			errCh <- httpServer.Listen(config.HTTPAddr())
		}()
	}

	if config.WDAddr() != "" {
		go func() {
			webdavServer := webdav.New(fs)
			log.Printf("ddrv: starting WEBDAV server on : %s", config.WDAddr())
			errCh <- webdavServer.ListenAndServe()
		}()
	}

	go stats("")

	log.Fatalf("ddrv: ddrv error %v", <-errCh)
}

func stats(msg string) {
	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("%v memory usage: Heap %v MB HeapIdle %v MB HeapReleased %v MB Frag %v MB \n", msg, btm(m.HeapAlloc), btm(m.HeapIdle), btm(m.HeapReleased), btm(m.HeapInuse-m.HeapAlloc))
			time.Sleep(5 * time.Second)
		}
	}()
}

func btm(n uint64) uint64 {
	return n / 1024 / 1024
}
