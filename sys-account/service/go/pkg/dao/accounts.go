package dao

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/VictoriaMetrics/metrics"
	"github.com/genjidb/genji/document"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/telemetry"

	utilities "go.amplifyedge.org/sys-share-v2/sys-core/service/config"

	accountRpc "go.amplifyedge.org/sys-share-v2/sys-account/service/go/rpc/v2"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/pass"
	coresvc "go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/coredb"
)

var (
	accountsUniqueIdx      = fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_email ON %s(email)", AccTableName, AccTableName)
	accountAvatarUniqueIdx = fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_avatar_resource_id ON %s(avatar_resource_id)", AccTableName, AccTableName)
)

type LoginAttempt struct {
	OriginIP      string `json:"origin_ip" genji:"origin_ip" coredb:"primary"`
	AccountEmail  string `json:"account_email,omitempty" genji:"account_email"`
	TotalAttempts uint   `json:"total_attempts" genji:"total_attempts"`
	BanPeriod     int64  `json:"ban_period" genji:"ban_period"`
}

type Account struct {
	ID                string `json:"id,omitempty" genji:"id" coredb:"primary"`
	Email             string `json:"email,omitempty" genji:"email"`
	Password          string `json:"password,omitempty" genji:"password"`
	CreatedAt         int64  `json:"created_at" genji:"created_at"`
	UpdatedAt         int64  `json:"updated_at" genji:"updated_at"`
	LastLogin         int64  `json:"last_login" genji:"last_login"`
	Disabled          bool   `json:"disabled" genji:"disabled"`
	Verified          bool   `json:"verified" genji:"verified"`
	VerificationToken string `json:"verification_token,omitempty" genji:"verification_token"`
	AvatarResourceId  string `json:"avatar_resource_id,omitempty" genji:"avatar_resource_id"`
}

func (a *AccountDB) InsertFromRpcAccountRequest(account *accountRpc.AccountNewRequest, verified bool) (*Account, error) {
	accountId := utilities.NewID()
	var roles []*Role
	if account.Roles != nil && len(account.Roles) > 0 {
		a.log.Debugf("Convert and getting roles")
		for _, accountRpcRole := range account.Roles {
			role := a.FromPkgRoleRequest(accountRpcRole, accountId)
			roles = append(roles, role)
		}
	} else if account.NewUserRoles != nil && len(account.NewUserRoles) > 0 {
		a.log.Debugf("Convert and getting new roles")
		for _, accountRpcNewRole := range account.NewUserRoles {
			param := map[string]interface{}{}
			if accountRpcNewRole.ProjectName != "" {
				param["name"] = accountRpcNewRole.ProjectName
			}
			if accountRpcNewRole.GetProjectId() != "" {
				param["id"] = accountRpcNewRole.GetProjectId()
			}
			project, err := a.GetProject(&coresvc.QueryParams{Params: param})
			if err != nil {
				return nil, err
			}
			joinedProjectMetrics := metrics.GetOrCreateCounter(fmt.Sprintf(telemetry.JoinProjectLabel, telemetry.METRICS_JOINED_PROJECT, project.OrgId, project.Id))
			go func() {
				joinedProjectMetrics.Inc()
			}()

			accountRpcNewRole.ProjectId = project.Id
			accountRpcNewRole.OrgId = project.OrgId
			role := a.FromPkgNewRoleRequest(accountRpcNewRole, accountId)
			roles = append(roles, role)
		}
	} else {
		roles = append(roles, &Role{
			ID:        utilities.NewID(),
			AccountId: accountId,
			Role:      int(accountRpc.Roles_GUEST),
			ProjectId: "",
			OrgId:     "",
			CreatedAt: utilities.CurrentTimestamp(),
		})
	}
	for _, daoRole := range roles {
		err := a.InsertRole(daoRole)
		if err != nil {
			return nil, err
		}
	}
	isVerified := false
	if verified {
		isVerified = verified
	}
	acc := &Account{
		ID:               accountId,
		Email:            account.Email,
		Password:         account.Password,
		CreatedAt:        utilities.CurrentTimestamp(),
		UpdatedAt:        utilities.CurrentTimestamp(),
		LastLogin:        utilities.CurrentTimestamp(),
		Disabled:         false,
		Verified:         isVerified,
		AvatarResourceId: account.AvatarFilepath,
	}

	if err := a.InsertAccount(acc); err != nil {
		return nil, err
	}
	return acc, nil
}

func (a *AccountDB) FromRpcAccount(account *accountRpc.Account) (*Account, error) {
	return &Account{
		ID:               account.Id,
		Email:            account.Email,
		Password:         account.Password,
		CreatedAt:        utilities.TsToUnixUTC(account.CreatedAt),
		UpdatedAt:        utilities.TsToUnixUTC(account.UpdatedAt),
		LastLogin:        utilities.TsToUnixUTC(account.LastLogin),
		Disabled:         account.Disabled,
		Verified:         account.Verified,
		AvatarResourceId: account.AvatarResourceId,
	}, nil
}

func (a *Account) ToRpcAccount(roles []*accountRpc.UserRoles, avatar []byte) (*accountRpc.Account, error) {
	avt := avatar
	if avt == nil {
		avt = []byte{}
	}
	c := utilities.UnixToUtcTS(a.CreatedAt)
	u := utilities.UnixToUtcTS(a.UpdatedAt)
	l := utilities.UnixToUtcTS(a.LastLogin)
	return &accountRpc.Account{
		Id:               a.ID,
		Email:            a.Email,
		Password:         a.Password,
		Roles:            roles,
		CreatedAt:        c,
		UpdatedAt:        u,
		LastLogin:        l,
		Disabled:         a.Disabled,
		Verified:         a.Verified,
		AvatarResourceId: a.AvatarResourceId,
		Avatar:           avt,
	}, nil
}

func accountToQueryParams(acc *Account) (res coresvc.QueryParams, err error) {
	return coresvc.AnyToQueryParam(acc, true)
}

// CreateSQL will only be called once by sys-core see sys-core API.
func (a Account) CreateSQL() []string {
	fields := coresvc.GetStructTags(a)
	tbl := coresvc.NewTable(AccTableName, fields, []string{accountsUniqueIdx})
	return tbl.CreateTable()
}

func (a *AccountDB) GetAccount(filterParams *coresvc.QueryParams) (*Account, error) {
	var acc Account
	selectStmt, args, err := coresvc.BaseQueryBuilder(
		filterParams.Params,
		AccTableName,
		a.accountColumns,
		"eq",
	).ToSql()
	if err != nil {
		return nil, err
	}
	doc, err := a.db.QueryOne(selectStmt, args...)
	if err != nil {
		return nil, err
	}
	err = doc.StructScan(&acc)
	return &acc, err
}

func (a *AccountDB) ListAccount(filterParams *coresvc.QueryParams, orderBy string, limit, cursor int64, sqlMatcher string) ([]*Account, int64, error) {
	var accs []*Account
	if sqlMatcher == "" {
		sqlMatcher = "like"
	}
	baseStmt := coresvc.BaseQueryBuilder(filterParams.Params, AccTableName, a.accountColumns, sqlMatcher)
	selectStmt, args, err := coresvc.ListSelectStatement(baseStmt, orderBy, limit, &cursor, DefaultCursor)
	if err != nil {
		return nil, 0, err
	}
	res, err := a.db.Query(selectStmt, args...)
	if err != nil {
		return nil, 0, err
	}
	err = res.Iterate(func(d document.Document) error {
		var acc Account
		if err = document.StructScan(d, &acc); err != nil {
			return err
		}
		accs = append(accs, &acc)
		return nil
	})
	res.Close()
	if err != nil {
		return nil, 0, err
	}
	return accs, accs[len(accs)-1].CreatedAt, nil
}

func (a *AccountDB) InsertAccount(acc *Account) error {
	passwd, err := pass.GenHash(acc.Password)
	if err != nil {
		return err
	}
	acc.Password = passwd
	filterParams, err := accountToQueryParams(acc)
	if err != nil {
		return err
	}
	columns, values := filterParams.ColumnsAndValues()
	stmt, args, err := sq.Insert(AccTableName).
		Columns(columns...).
		Values(values...).
		ToSql()
	a.log.Debugf("insert to accounts table, stmt: %v, args: %v", columns, values)
	if err != nil {
		return err
	}
	return a.db.Exec(stmt, args...)
}

func (a *AccountDB) UpdateAccount(acc *Account) error {
	filterParams, err := accountToQueryParams(acc)
	if err != nil {
		return err
	}
	delete(filterParams.Params, "id")
	stmt, args, err := sq.Update(AccTableName).SetMap(filterParams.Params).
		Where(sq.Eq{"id": acc.ID}).ToSql()
	a.log.Debugf(
		"update accounts statement: %v, args: %v", stmt,
		args,
	)
	if err != nil {
		return err
	}
	return a.db.Exec(stmt, args...)
}

func (a *AccountDB) DeleteAccount(id string) error {
	stmt, args, err := sq.Delete(AccTableName).Where("id = ?", id).ToSql()
	if err != nil {
		return err
	}
	rstmt, rargs, err := sq.Delete(RolesTableName).Where("account_id = ?", id).ToSql()
	if err != nil {
		return err
	}
	return a.db.BulkExec(map[string][]interface{}{
		stmt:  args,
		rstmt: rargs,
	})
}

func (a *AccountDB) UpsertLoginAttempt(originIp string, accountEmail string, attempt uint, banPeriod int64) (*LoginAttempt, error) {
	newLoginAttempt := &LoginAttempt{
		OriginIP:      originIp,
		AccountEmail:  accountEmail,
		TotalAttempts: attempt,
		BanPeriod:     banPeriod,
	}
	queryParam, err := coresvc.AnyToQueryParam(newLoginAttempt, true)
	if err != nil {
		return nil, err
	}
	columns, values := queryParam.ColumnsAndValues()

	_, err = a.GetLoginAttempt(originIp)
	if err != nil {
		return a.insertLoginAttempt(originIp, columns, values)
	}
	return a.updateLoginAttempt(originIp, queryParam.Params)
}

func (a *AccountDB) GetLoginAttempt(originIp string) (*LoginAttempt, error) {
	var la LoginAttempt
	selectStmt, args, err := coresvc.BaseQueryBuilder(map[string]interface{}{"origin_ip": originIp}, LoginAttemptsTableName, a.loginAttemptColumns, "eq").ToSql()
	if err != nil {
		return nil, err
	}
	res, err := a.db.QueryOne(selectStmt, args...)
	if err != nil {
		return nil, err
	}
	if err = res.StructScan(&la); err != nil {
		return nil, err
	}
	return &la, nil
}

func (a *AccountDB) insertLoginAttempt(originIp string, columns []string, values []interface{}) (*LoginAttempt, error) {
	stmt, args, err := sq.Insert(LoginAttemptsTableName).
		Columns(columns...).
		Values(values...).
		ToSql()
	if err != nil {
		return nil, err
	}
	if err = a.db.Exec(stmt, args...); err != nil {
		return nil, err
	}
	return a.GetLoginAttempt(originIp)
}

func (a *AccountDB) updateLoginAttempt(originIp string, requestMap map[string]interface{}) (*LoginAttempt, error) {
	email := requestMap["account_email"]
	delete(requestMap, "account_email")
	stmt, args, err := sq.Update(LoginAttemptsTableName).SetMap(requestMap).Where(sq.Eq{"account_email": email}).ToSql()
	if err != nil {
		return nil, err
	}
	if err = a.db.Exec(stmt, args...); err != nil {
		return nil, err
	}
	return a.GetLoginAttempt(originIp)
}

// CreateSQL will only be called once by sys-core see sys-core API.
func (l LoginAttempt) CreateSQL() []string {
	fields := coresvc.GetStructTags(l)
	tbl := coresvc.NewTable(LoginAttemptsTableName, fields, []string{})
	return tbl.CreateTable()
}
