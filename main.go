package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sgin/controller"
	"sgin/model"
	"sgin/pkg/app"
	"sgin/pkg/config"
	"sgin/routers"
	"sgin/service"
	"syscall"
	"time"

	redis "github.com/go-redis/redis/v8"
)

// @title           Swagger Example API
// @version         1.0
// @description     This is a sample server celler server.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.basic  BasicAuth

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	if err := config.InitConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		// 建议：将错误写入标准错误日志或集中式日志系统
		os.Exit(1)
	}

	serverApp, err := app.NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize app: %v\n", err)
		os.Exit(1)
	}
	// 退出时同步日志
	defer func() { _ = serverApp.Logger.Sync() }()
	// 启动自检（非阻断式提示）
	serverApp.RunSelfCheck()
	model.MigrateDbTable(serverApp.DB)
	serverApp.Use(app.RecoveryWithWriter(serverApp.Logger))
	serverApp.Use(app.SecurityHeaders())
	serverApp.Use(app.Cors())
	if serverApp.Config.LogConfig.Level == "debug" {
		serverApp.Use(app.RequestLogger())
		serverApp.Use(app.ResponseLogger())
	}

	routers.InitRouter(serverApp)

	serverApp.GET("/ping", func(ctx *app.Context) {
		ctx.JSONSuccess("pong")
	})
	serverApp.GET("/healthz", func(ctx *app.Context) {
		ctx.JSONSuccess("ok")
	})
	serverApp.GET("/readyz", func(ctx *app.Context) {
		if serverApp.DB != nil {
			sqlDB, err := serverApp.DB.DB()
			if err != nil {
				ctx.JSONError(http.StatusServiceUnavailable, "db not ready")
				return
			}
			if err := sqlDB.PingContext(ctx.Ctx); err != nil {
				ctx.JSONError(http.StatusServiceUnavailable, "db not ready")
				return
			}
		}
		if serverApp.Redis != nil {
			if _, err := serverApp.Redis.Get(ctx.Ctx, "__readyz"); err != nil && err != redis.Nil {
				ctx.JSONError(http.StatusServiceUnavailable, "redis not ready")
				return
			}
		}
		ctx.JSONSuccess("ready")
	})

	// 确保上传目录存在
	if serverApp.Config.Upload.Dir != "" {
		_ = os.MkdirAll(serverApp.Config.Upload.Dir, 0755)
		serverApp.Router.Static("/public", serverApp.Config.Upload.Dir)
	}

	// 启动库存预占释放后台任务（每分钟检查）
	// 构造一个轻量的 app.Context 以供后台任务使用（无 gin.Context）
	reservationCtx := &app.Context{
		DB:     serverApp.DB,
		Logger: serverApp.Logger,
		Config: serverApp.Config,
		Ctx:    context.Background(),
	}
	service.NewReservationService().StartReservationReleaser(reservationCtx, 1*time.Minute)

	serverApp.NoRoute(app.NoRouterHandler())

	v1 := serverApp.Group("/api/v1")
	userController := &controller.UserController{Service: &service.UserService{}}
	{
		v1.POST("/users", userController.CreateUser)
	}

	srv := &http.Server{
		Addr:         ":" + serverApp.Config.ServerPort,
		Handler:      serverApp.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverApp.Logger.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	serverApp.Logger.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		serverApp.Logger.Fatal("Server forced to shutdown: ", err)
	}

	serverApp.Logger.Info("Server exiting")
}
