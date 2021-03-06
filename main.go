package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/huskar-t/blm_demo/config"
	"github.com/huskar-t/blm_demo/log"
	"github.com/huskar-t/blm_demo/plugin"
	_ "github.com/huskar-t/blm_demo/plugin/influxdb"
	_ "github.com/huskar-t/blm_demo/plugin/opentsdb"
	"github.com/huskar-t/blm_demo/rest"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var logger = log.GetLogger("main")

func createRouter(debug bool, corsConf *config.CorsConfig, enableGzip bool) *gin.Engine {
	if debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(log.GinLog())
	router.Use(log.GinRecoverLog())
	if debug {
		pprof.Register(router)
	}
	if enableGzip {
		router.Use(gzip.Gzip(gzip.DefaultCompression))
	}
	router.Use(cors.New(corsConf.GetConfig()))
	return router
}

func main() {
	config.Init()
	log.ConfigLog()
	logger.Info("start server:", log.ServerID)
	router := createRouter(config.Conf.Debug, &config.Conf.Cors, false)
	router.POST("logModel", func(c *gin.Context) {
		body, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		err = log.SetLevel(string(body))
		if err != nil {
			c.JSON(http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusOK, body)
	})
	r := rest.Restful{}
	_ = r.Init(router)
	plugin.RegisterGenerateAuth(router)
	plugin.Init(router)
	plugin.Start()
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(config.Conf.Port),
		Handler:           router,
		ReadHeaderTimeout: 20 * time.Second,
		ReadTimeout:       200 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	logger.Println("server on :", config.Conf.Port)
	if config.Conf.SSl.Enable {
		go func() {
			if err := server.ListenAndServeTLS(config.Conf.SSl.CertFile, config.Conf.SSl.KeyFile); err != nil && err != http.ErrServerClosed {
				logger.Fatalf("listen: %s\n", err)
			}
		}()
	} else {
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatalf("listen: %s\n", err)
			}
		}()
	}
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	<-quit
	logger.Println("Shutdown WebServer ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		if err := server.Shutdown(ctx); err != nil {
			logger.Println("WebServer Shutdown error:", err)
		}
	}()
	logger.Println("Stop Plugins ...")
	ticker := time.NewTicker(time.Second * 5)
	done := make(chan struct{})
	go func() {
		r.Close()
		plugin.Stop()
		close(done)
	}()
	select {
	case <-done:
		break
	case <-ticker.C:
		break
	}
	logger.Println("Server exiting")
}
