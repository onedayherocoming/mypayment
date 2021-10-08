package main

import (
	"github.com/asim/go-micro/plugins/registry/consul/v3"
	"github.com/asim/go-micro/plugins/wrapper/monitoring/prometheus/v3"
	ratelimit "github.com/asim/go-micro/plugins/wrapper/ratelimiter/uber/v3"
	opentracing2 "github.com/asim/go-micro/plugins/wrapper/trace/opentracing/v3"
	"github.com/asim/go-micro/v3"
	log "github.com/asim/go-micro/v3/logger"
	"github.com/asim/go-micro/v3/registry"
	"github.com/jinzhu/gorm"
	"github.com/onedayherocoming/mycommon"
	"github.com/onedayherocoming/mypayment/domain/repository"
	service2 "github.com/onedayherocoming/mypayment/domain/service"
	"github.com/onedayherocoming/mypayment/handler"
	"github.com/opentracing/opentracing-go"

	_ "github.com/jinzhu/gorm/dialects/mysql"

	payment "github.com/onedayherocoming/mypayment/proto/payment"
)
func main() {
	// 配置中心
	ip:= "192.168.1.100"
	consulConfig,err := common.GetConsulConfig(ip,8500,"/micro/config")
	if err !=nil {
		common.Error(err)
	}
	//注册中心
	consul := consul.NewRegistry(func(options *registry.Options) {
		options.Addrs = []string{
			ip+":8500",
		}
	})
	//jaeger 链路追踪
	t,io,err:=common.NewTracer("go.micro.service.payment",ip+":6831")
	if err!=nil{
		common.Error(err)
	}
	defer io.Close()
	opentracing.SetGlobalTracer(t)

	//mysql 设置
	mysqlInfo := common.GetMysqlFromConsul(consulConfig,"mysql")
	//初始化数据库
	db, err := gorm.Open("mysql", mysqlInfo.User+":"+mysqlInfo.Pwd+"@tcp("+mysqlInfo.Host+")/"+mysqlInfo.Database+"?charset=utf8&parseTime=True")
	if err !=nil {
		common.Error(err)
	}
	defer db.Close()
	//禁止复数表
	db.SingularTable(true)

	//创建表
	tableInit := repository.NewPaymentRepository(db)
	tableInit.InitTable()

	//监控
	common.PrometheusBoot(9089)


	// New Service
	service := micro.NewService(
		micro.Name("go.micro.service.payment"),
		micro.Version("latest"),
		micro.Address("0.0.0.0:8089"),
		//添加注册中心
		micro.Registry(consul),
		//添加链路追踪
		micro.WrapHandler(opentracing2.NewHandlerWrapper(opentracing.GlobalTracer())),
		//加载限流
		micro.WrapHandler(ratelimit.NewHandlerWrapper(1000)),
		//加载监控
		micro.WrapHandler(prometheus.NewHandlerWrapper()),
	)

	// Initialise service
	service.Init()

	paymentDataService := service2.NewPaymentDataService(repository.NewPaymentRepository(db))

	// Register Handler
	payment.RegisterPaymentHandler(service.Server(), &handler.Payment{PaymentDataService:paymentDataService})

	// Run service
	if err := service.Run(); err != nil {
		log.Fatal(err)
	}
}