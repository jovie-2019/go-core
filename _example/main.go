package main

import (
	"fmt"
	go_application "github.com/pefish/go-application"
	go_config "github.com/pefish/go-config"
	go_core "github.com/pefish/go-core"
	api_strategy "github.com/pefish/go-core/api-strategy"
	external_service "github.com/pefish/go-core/driver/external-service"
	global_api_strategy2 "github.com/pefish/go-core/driver/global-api-strategy"
	"github.com/pefish/go-core/driver/logger"
	"github.com/pefish/go-core/global-api-strategy"
	go_logger "github.com/pefish/go-logger"
	"log"
	"os"
	"runtime/debug"
	external_service2 "test/external-service"
	"test/route"
	"time"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			fmt.Println(string(debug.Stack()))
			os.Exit(1)
		}
		os.Exit(0)
	}()

	go_application.Application.SetEnv(`local`)
	go_config.Config.MustLoadYamlConfig(go_config.Configuration{
		ConfigEnvName: `GO_CONFIG`,
		SecretEnvName: `GO_SECRET`,
	})

	go_core.Service.SetName(`测试服务api`)
	global_api_strategy2.GlobalApiStrategyDriver.Register(global_api_strategy2.GlobalStrategyData{
		Strategy: &global_api_strategy.OpenCensusStrategy,
		Disable:  go_application.Application.Env == `local`,
		Param: global_api_strategy.OpenCensusStrategyParam{
			StackDriverOption: nil,
			EnableTrace:       true,
			EnableStats:       false,
		},
	})

	global_api_strategy.GlobalRateLimitStrategy.SetErrorCode(10000)
	global_api_strategy2.GlobalApiStrategyDriver.Register(global_api_strategy2.GlobalStrategyData{
		Strategy: &global_api_strategy.GlobalRateLimitStrategy,
		Param:    global_api_strategy.GlobalRateLimitStrategyParam{
			FillInterval: 1000 * time.Millisecond,
		},
		Disable:  false,
	})

	go_logger.Logger = go_logger.NewLogger(go_logger.WithIsDebug(go_application.Application.Debug))
	logger.LoggerDriver.Register(go_logger.Logger)

	external_service.ExternalServiceDriver.Register(`deposit_address`, &external_service2.DepositAddressService)

	//go_mysql.MysqlHelper.ConnectWithMap(go_config.Config.MustGetMap(`mysql`))

	go_core.Service.SetPath(`/api/test`)
	api_strategy.RateLimitApiStrategy.SetErrorCode(2006)
	global_api_strategy.ParamValidateStrategy.SetErrorCode(2005)
	api_strategy.IpFilterStrategy.SetErrorCode(2007)
	go_core.Service.SetRoutes(route.TestRoute)
	go_core.Service.SetPort(go_config.Config.GetUint64(`port`))

	go_core.Service.Run()
}
