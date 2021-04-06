package repo

import (
	rpc "go.amplifyedge.org/sys-share-v2/sys-account/service/go/rpc/v2"
	"go.amplifyedge.org/sys-share-v2/sys-core/service/logging"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/superusers"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/telemetry"

	sharedAuth "go.amplifyedge.org/sys-share-v2/sys-account/service/go/pkg/shared"
	corebus "go.amplifyedge.org/sys-share-v2/sys-core/service/go/pkg/bus"
	"go.amplifyedge.org/sys-v2/sys-account/service/go"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/dao"
	"go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/coredb"
	corefile "go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/filesvc/repo"
	coremail "go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/mailer"
)

type (
	// SysAccountRepo is the repository layer of the authn & authz && accounts
	SysAccountRepo struct {
		store    *dao.AccountDB
		log      logging.Logger
		tokenCfg *sharedAuth.TokenConfig
		// the auth interceptor would not intercept tokens on these routes
		// (format is: /ProtoServiceName/ProtoServiceMethod, example: /proto.AuthService/Login).
		unauthenticatedRoutes []string
		bus                   *corebus.CoreBus
		mail                  *coremail.MailSvc
		frepo                 *corefile.SysFileRepo
		domain                string
		//initialSuperusersMail []string
		bizmetrics *telemetry.SysAccountMetrics
		superDao   *superusers.SuperUserIO
		*rpc.UnimplementedAccountServiceServer
		*rpc.UnimplementedAuthServiceServer
		*rpc.UnimplementedOrgProjServiceServer
	}
)

func NewAuthRepo(l logging.Logger, db *coredb.CoreDB, cfg *service.SysAccountConfig, bus *corebus.CoreBus, mail *coremail.MailSvc, frepo *corefile.SysFileRepo, domain string, superUserFilePath string, bizmetrics *telemetry.SysAccountMetrics) (*SysAccountRepo, error) {
	accdb, err := dao.NewAccountDB(db, l)
	if err != nil {
		l.Errorf("Error while initializing DAO: %v", err)
		return nil, err
	}
	tokenCfg := sharedAuth.NewTokenConfig([]byte(cfg.JWTConfig.Access.Secret), []byte(cfg.JWTConfig.Refresh.Secret))
	//var initialSuperMails []string
	//for _, sureq := range supes {
	//	initialSuperMails = append(initialSuperMails, sureq.Email)
	//}
	superDao := superusers.NewSuperUserDAO(superUserFilePath, l)
	repo := &SysAccountRepo{
		store:                 accdb,
		log:                   l,
		tokenCfg:              tokenCfg,
		unauthenticatedRoutes: cfg.UnauthenticatedRoutes,
		bus:                   bus,
		mail:                  mail,
		frepo:                 frepo,
		domain:                domain,
		//initialSuperusersMail: initialSuperMails,
		bizmetrics: bizmetrics,
		superDao:   superDao,
	}
	// Register Bus Dispatchers
	bus.RegisterAction("onDeleteOrg", repo.onDeleteOrg)
	bus.RegisterAction("onDeleteAccount", repo.onDeleteAccount)
	bus.RegisterAction("onDeleteProject", repo.onDeleteProject)
	bus.RegisterAction("onCheckProjectExists", repo.onCheckProjectExists)
	bus.RegisterAction("onCheckAccountExists", repo.onCheckAccountExists)
	bus.RegisterAction("onResetAllSysAccount", repo.onResetAllSysAccount)
	bus.RegisterAction("onGetAccountEmail", repo.onGetAccountEmail)
	bus.RegisterAction("onCheckAllowProject", repo.onCheckAllowProject)
	bus.RegisterAction("onCheckAllowSurveyUser", repo.onCheckAllowSurveyUser)
	bus.RegisterAction("onJoinNewProject", repo.onJoinNewProject)
	return repo, nil
}
