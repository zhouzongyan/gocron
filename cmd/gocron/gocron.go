// Command goscheduler
//go:generate statik -src=../../web/public -dest=../../internal -f

package main

import (
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	macaron "gopkg.in/macaron.v1"

	"chn.gg/zhouzongyan/gocron/internal/models"
	"chn.gg/zhouzongyan/gocron/internal/modules/app"
	"chn.gg/zhouzongyan/gocron/internal/modules/logger"
	"chn.gg/zhouzongyan/gocron/internal/modules/rpc/auth"
	"chn.gg/zhouzongyan/gocron/internal/modules/rpc/server"
	"chn.gg/zhouzongyan/gocron/internal/modules/setting"
	"chn.gg/zhouzongyan/gocron/internal/modules/utils"
	"chn.gg/zhouzongyan/gocron/internal/routers"
	"chn.gg/zhouzongyan/gocron/internal/service"
	"chn.gg/zhouzongyan/gocron/internal/util"
	"github.com/urfave/cli"
)

var (
	AppVersion           = "1.5"
	BuildDate, GitCommit string
)

// web服务器默认端口
const DefaultPort = 5920

func main() {
	cliApp := cli.NewApp()
	cliApp.Name = "gocron"
	cliApp.Usage = "gocron service"
	cliApp.Version, _ = util.FormatAppVersion(AppVersion, GitCommit, BuildDate)
	cliApp.Commands = getCommands()
	cliApp.Flags = append(cliApp.Flags, []cli.Flag{}...)
	err := cliApp.Run(os.Args)
	if err != nil {
		logger.Fatal(err)
	}
}

// getCommands
func getCommands() []cli.Command {
	command := cli.Command{
		Name:   "web",
		Usage:  "run web server",
		Action: runWeb,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "host",
				Value: "0.0.0.0",
				Usage: "bind host",
			},
			cli.IntFlag{
				Name:  "port,p",
				Value: DefaultPort,
				Usage: "bind port",
			},
			cli.StringFlag{
				Name:  "env,e",
				Value: "prod",
				Usage: "runtime environment, dev|test|prod",
			},
		},
	}

	node := cli.Command{
		Name:   "node",
		Usage:  "run node server",
		Action: runNode,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "root",
				Usage: "run server as root",
			},
			cli.StringFlag{
				Name:  "s",
				Value: "0.0.0.0:5921",
				Usage: "bind port ip:port",
			},
			cli.BoolFlag{
				Name:  "tls",
				Usage: "enable-tls",
			},
			cli.StringFlag{
				Name:  "ca",
				Value: "",
				Usage: "tls ca file",
			},
			cli.StringFlag{
				Name:  "cert",
				Value: "",
				Usage: "tls cert file",
			},
			cli.StringFlag{
				Name:  "key",
				Value: "",
				Usage: "tls key file",
			},
			cli.StringFlag{
				Name:  "logLevel",
				Value: "info",
				Usage: "log lever",
			},
		},
	}

	return []cli.Command{command, node}
}
func runNode(ctx *cli.Context) {
	logLevel := "info"
	if ctx.IsSet("logLevel") {
		logLevel = ctx.String("logLevel")
	}
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(level)

	// if version {
	// 	util.PrintAppVersion(AppVersion, GitCommit, BuildDate)
	// 	return
	// }
	enableTLS := false
	if ctx.IsSet("tls") {
		enableTLS = ctx.Bool("tls")
	}
	CAFile := ctx.String("ca")
	certFile := ctx.String("cert")
	keyFile := ctx.String("key")
	if enableTLS {

		if !utils.FileExist(CAFile) {
			log.Fatalf("failed to read ca cert file: %s", CAFile)
		}
		if !utils.FileExist(certFile) {
			log.Fatalf("failed to read server cert file: %s", certFile)
			return
		}
		if !utils.FileExist(keyFile) {
			log.Fatalf("failed to read server key file: %s", keyFile)
			return
		}
	}

	certificate := auth.Certificate{
		CAFile:   strings.TrimSpace(CAFile),
		CertFile: strings.TrimSpace(certFile),
		KeyFile:  strings.TrimSpace(keyFile),
	}
	allowRoot := ctx.Bool("root")
	if runtime.GOOS != "windows" && os.Getuid() == 0 && !allowRoot {
		log.Fatal("Do not run goscheduler-node as root user")
		return
	}
	serverAddr := ctx.String("s")
	server.Start(serverAddr, enableTLS, certificate)
}

func runWeb(ctx *cli.Context) {
	// 设置运行环境
	setEnvironment(ctx)
	// 初始化应用
	app.InitEnv(AppVersion)
	// 初始化模块 DB、定时任务等
	initModule()
	// 捕捉信号,配置热更新等
	go catchSignal()
	m := macaron.Classic()
	// 注册路由
	routers.Register(m)
	// 注册中间件.
	routers.RegisterMiddleware(m)
	host := parseHost(ctx)
	port := parsePort(ctx)
	m.Run(host, port)
}

func initModule() {
	if !app.Installed {
		return
	}

	config, err := setting.Read(app.AppConfig)
	if err != nil {
		logger.Fatal("读取应用配置失败", err)
	}
	app.Setting = config

	// 初始化DB
	models.Db = models.CreateDb()

	// 版本升级
	upgradeIfNeed()

	// 初始化定时任务
	service.ServiceTask.Initialize()
}

// 解析端口
func parsePort(ctx *cli.Context) int {
	port := DefaultPort
	if ctx.IsSet("port") {
		port = ctx.Int("port")
	}
	if port <= 0 || port >= 65535 {
		port = DefaultPort
	}

	return port
}

func parseHost(ctx *cli.Context) string {
	if ctx.IsSet("host") {
		return ctx.String("host")
	}

	return "0.0.0.0"
}

func setEnvironment(ctx *cli.Context) {
	env := "prod"
	if ctx.IsSet("env") {
		env = ctx.String("env")
	}

	switch env {
	case "test":
		macaron.Env = macaron.TEST
	case "dev":
		macaron.Env = macaron.DEV
	default:
		macaron.Env = macaron.PROD
	}
}

// 捕捉信号
func catchSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	for {
		s := <-c
		logger.Info("收到信号 -- ", s)
		switch s {
		case syscall.SIGHUP:
			logger.Info("收到终端断开信号, 忽略")
		case syscall.SIGINT, syscall.SIGTERM:
			shutdown()
		}
	}
}

// 应用退出
func shutdown() {
	defer func() {
		logger.Info("已退出")
		os.Exit(0)
	}()

	if !app.Installed {
		return
	}
	logger.Info("应用准备退出")
	// 停止所有任务调度
	logger.Info("停止定时任务调度")
	service.ServiceTask.WaitAndExit()
}

// 判断应用是否需要升级, 当存在版本号文件且版本小于app.VersionId时升级
func upgradeIfNeed() {
	currentVersionId := app.GetCurrentVersionId()
	// 没有版本号文件
	if currentVersionId == 0 {
		return
	}
	if currentVersionId >= app.VersionId {
		return
	}

	migration := new(models.Migration)
	logger.Infof("版本升级开始, 当前版本号%d", currentVersionId)

	migration.Upgrade(currentVersionId)
	app.UpdateVersionFile()

	logger.Infof("已升级到最新版本%d", app.VersionId)
}
