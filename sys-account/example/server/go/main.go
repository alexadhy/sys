// package main
// this is only used for local testing
// making sure that the sys-account service works locally before wiring it up to the maintemplate.
package main

import (
	"fmt"
	"github.com/getcouragenow/sys-share/sys-core/service/logging/zaplog"
	"net/http"

	"github.com/getcouragenow/sys-share/sys-core/service/logging"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/spf13/cobra"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"

	grpcMw "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcRecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"

	corebus "github.com/getcouragenow/sys-share/sys-core/service/go/pkg/bus"
	"github.com/getcouragenow/sys/sys-account/service/go/pkg"
)

const (
	errSourcingConfig   = "error while sourcing config for %s: %v"
	errCreateSysService = "error while creating sys-%s service: %v"

	defaultAddr                 = "127.0.0.1"
	defaultPort                 = 8888
	defaultSysCoreConfigPath    = "./bin-all/config/syscore.yml"
	defaultSysAccountConfigPath = "./bin-all/config/sysaccount.yml"
	defaultSysFileConfigPath    = "./bin-all/config/sysfile.yml"
)

var (
	rootCmd        = &cobra.Command{Use: "sys-account-srv"}
	coreCfgPath    string
	accountCfgPath string
	fileCfgPath    string
)

func recoveryHandler(l logging.Logger) func(panic interface{}) error {
	return func(panic interface{}) error {
		l.Warnf("sys-account service recovered, reason: %v",
			panic)
		return nil
	}
}

func main() {
	rootCmd.PersistentFlags().StringVarP(&coreCfgPath, "sys-core-config-path", "c", defaultSysCoreConfigPath, "sys-core config path to use")
	rootCmd.PersistentFlags().StringVarP(&accountCfgPath, "sys-account-config-path", "a", defaultSysAccountConfigPath, "sys-account config path to use")
	rootCmd.PersistentFlags().StringVarP(&fileCfgPath, "sys-file-config-path", "a", defaultSysFileConfigPath, "sys-file config path to use")

	log := zaplog.NewZapLogger("debug", "sys-account", true)
	log.InitLogger(nil)

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		sysAccountConfig, err := accountpkg.NewSysAccountServiceConfig(log, accountCfgPath, corebus.NewCoreBus())
		if err != nil {
			log.Fatalf("error creating config: %v", err)
		}

		svc, err := accountpkg.NewSysAccountService(sysAccountConfig, "127.0.0.1:8080")
		if err != nil {
			log.Fatalf("error creating sys-account service: %v", err)
		}

		recoveryOptions := []grpcRecovery.Option{
			grpcRecovery.WithRecoveryHandler(recoveryHandler(log)),
		}

		unaryItc := []grpc.UnaryServerInterceptor{
			grpcRecovery.UnaryServerInterceptor(recoveryOptions...),
			log.GetServerUnaryInterceptor(),
		}

		streamItc := []grpc.StreamServerInterceptor{
			grpcRecovery.StreamServerInterceptor(recoveryOptions...),
			log.GetServerStreamInterceptor(),
		}

		unaryItc, streamItc = svc.InjectInterceptors(unaryItc, streamItc)
		grpcSrv := grpc.NewServer(
			grpcMw.WithUnaryServerChain(unaryItc...),
			grpcMw.WithStreamServerChain(streamItc...),
		)

		// Register sys-account service
		svc.RegisterServices(grpcSrv)

		grpcWebServer := grpcweb.WrapServer(
			grpcSrv,
			grpcweb.WithCorsForRegisteredEndpointsOnly(false),
			grpcweb.WithWebsocketOriginFunc(func(req *http.Request) bool {
				return true
			}),
			grpcweb.WithWebsockets(true),
		)

		httpServer := &http.Server{
			Handler: h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-User-Agent, X-Grpc-Web")
				log.Infof("Request Endpoint: %s", r.URL)
				grpcWebServer.ServeHTTP(w, r)
			}), &http2.Server{}),
		}
		httpServer.Addr = fmt.Sprintf("%s:%d", defaultAddr, defaultPort)
		log.Infof("service listening at %v\n", httpServer.Addr)
		return httpServer.ListenAndServe()
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}

}
