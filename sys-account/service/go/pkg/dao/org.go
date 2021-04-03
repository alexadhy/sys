package dao

import (
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/genjidb/genji/document"
	rpc "go.amplifyedge.org/sys-share-v2/sys-account/service/go/rpc/v2"
	utilities "go.amplifyedge.org/sys-share-v2/sys-core/service/config"
	coresvc "go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/coredb"
)

type Org struct {
	Id             string `genji:"id" json:"id,omitempty" coredb:"primary"`
	Name           string `genji:"name" json:"name,omitempty"`
	LogoResourceId string `genji:"logo_resource_id" json:"logo_resource_id,omitempty"`
	Contact        string `genji:"contact" json:"contact,omitempty"`
	CreatedAt      int64  `genji:"created_at" json:"created_at"`
	AccountId      string `genji:"account_id" json:"account_id"`
}

var (
	orgUniqueIndex     = fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_name ON %s(name)", OrgTableName, OrgTableName)
	orgLogoUniqueIndex = fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_logo_resource_id ON %s(logo_resource_id)", OrgTableName, OrgTableName)
)

func (a *AccountDB) FromrpcOrgRequest(org *rpc.OrgRequest, id string) (*Org, error) {
	orgId := id
	if orgId == "" {
		orgId = utilities.NewID()
	}
	return &Org{
		Id:             orgId,
		Name:           org.Name,
		LogoResourceId: org.LogoFilepath,
		Contact:        org.Contact,
		CreatedAt:      utilities.CurrentTimestamp(),
		AccountId:      org.CreatorId,
	}, nil
}

func (a *AccountDB) FromRpcOrgRequestToQueryFilter(org *rpc.OrgRequest) (*coresvc.QueryParams, error) {
	qf, err := coresvc.AnyToQueryParam(org, false)
	if err != nil {
		return nil, err
	}
	delete(qf.Params, "logo_upload_bytes")
	return &qf, nil
}

func (o *Org) ToRpcOrg(projects []*rpc.Project, logo []byte) (*rpc.Org, error) {
	return &rpc.Org{
		Id:             o.Id,
		Name:           o.Name,
		LogoResourceId: o.LogoResourceId,
		Logo:           logo,
		Contact:        o.Contact,
		CreatedAt:      utilities.UnixToUtcTS(o.CreatedAt),
		CreatorId:      o.AccountId,
		Projects:       projects,
	}, nil
}

func (o Org) CreateSQL() []string {
	fields := coresvc.GetStructTags(o)
	// tbl := coresvc.NewTable(OrgTableName, fields, []string{orgUniqueIndex})
	tbl := coresvc.NewTable(OrgTableName, fields, []string{orgUniqueIndex, orgLogoUniqueIndex})
	return tbl.CreateTable()
}

func orgToQueryParam(org *Org) (res coresvc.QueryParams, err error) {
	return coresvc.AnyToQueryParam(org, true)
}

func (a *AccountDB) GetOrg(filterParam *coresvc.QueryParams) (*Org, error) {
	var o Org
	selectStmt, args, err := coresvc.BaseQueryBuilder(filterParam.Params, OrgTableName, a.orgColumns, "eq").ToSql()
	if err != nil {
		return nil, err
	}
	doc, err := a.db.QueryOne(selectStmt, args...)
	if err != nil {
		return nil, err
	}
	err = doc.StructScan(&o)
	return &o, err
}

func (a *AccountDB) ListOrg(filterParam *coresvc.QueryParams, orderBy string, limit, cursor int64, sqlMatcher string) ([]*Org, int64, error) {
	var orgs []*Org
	if sqlMatcher == "" {
		sqlMatcher = "like"
	}
	baseStmt := coresvc.BaseQueryBuilder(filterParam.Params, OrgTableName, a.orgColumns, sqlMatcher)
	selectStmt, args, err := coresvc.ListSelectStatement(baseStmt, orderBy, limit, &cursor, DefaultCursor)
	if err != nil {
		return nil, 0, err
	}
	res, err := a.db.Query(selectStmt, args...)
	if err != nil {
		return nil, 0, err
	}
	err = res.Iterate(func(d document.Document) error {
		var org Org
		if err = document.StructScan(d, &org); err != nil {
			return err
		}
		orgs = append(orgs, &org)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	_ = res.Close()
	if len(orgs) == 1 {
		return orgs, orgs[0].CreatedAt, nil
	}
	return orgs, orgs[len(orgs)-1].CreatedAt, nil
}

func (a *AccountDB) ListNonSubbed(accountId string, filterParams *coresvc.QueryParams, orderBy string, limit, cursor int64) ([]*Org, int64, error) {
	baseStmt := sq.Select(a.orgColumns).From(OrgTableName)
	if accountId != "" {
		roles, err := a.FetchRoles(accountId)
		if err != nil {
			return nil, 0, err
		}
		orgIdMap := map[string]string{}
		for _, r := range roles {
			if r.OrgId != "" {
				// dedup
				orgIdMap[r.OrgId] = r.OrgId
			}
		}
		var orgIdList []string
		if len(orgIdMap) != 0 {
			for k, _ := range orgIdMap {
				orgIdList = append(orgIdList, k)
			}
			baseStmt = baseStmt.Where("id NOT IN ?", orgIdList)
		}
	}
	if filterParams != nil && filterParams.Params != nil {
		for k, v := range filterParams.Params {
			baseStmt = baseStmt.Where(sq.Like{k: a.BuildSearchQuery(v.(string))})
		}
	}
	var orgs []*Org
	selectStmt, args, err := coresvc.ListSelectStatement(baseStmt, orderBy, limit, &cursor, DefaultCursor)
	if err != nil {
		return nil, 0, err
	}
	res, err := a.db.Query(selectStmt, args...)
	if err != nil {
		return nil, 0, err
	}
	err = res.Iterate(func(d document.Document) error {
		var org Org
		if err = document.StructScan(d, &org); err != nil {
			return err
		}
		orgs = append(orgs, &org)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	_ = res.Close()
	if len(orgs) == 1 {
		return orgs, 0, nil
	}
	return orgs, orgs[len(orgs)-1].CreatedAt, nil
}

func (a *AccountDB) InsertOrg(o *Org) error {
	filterParam, err := orgToQueryParam(o)
	if err != nil {
		return err
	}
	columns, values := filterParam.ColumnsAndValues()
	if len(columns) != len(values) {
		return fmt.Errorf("error: length mismatch: cols: %d, vals: %d", len(columns), len(values))
	}
	stmt, args, err := sq.Insert(OrgTableName).
		Columns(columns...).
		Values(values...).
		ToSql()
	a.log.Debugf("insert to %s table", OrgTableName)
	if err != nil {
		return err
	}
	return a.db.Exec(stmt, args...)
}

func (a *AccountDB) UpdateOrg(o *Org) error {
	filterParam, err := orgToQueryParam(o)
	if err != nil {
		return err
	}
	a.log.Debugf("org update param: %v", filterParam)
	delete(filterParam.Params, "id")
	stmt, args, err := sq.Update(OrgTableName).SetMap(filterParam.Params).
		Where(sq.Eq{"id": o.Id}).ToSql()
	if err != nil {
		return err
	}
	a.log.Debugf(
		"update org statement: %v, args: %v", stmt,
		args,
	)
	return a.db.Exec(stmt, args...)
}

func (a *AccountDB) DeleteOrg(id string) error {
	stmt, args, err := sq.Delete(OrgTableName).Where("id = ?", id).ToSql()
	if err != nil {
		return err
	}
	pstmt, pargs, err := sq.Delete(ProjectTableName).Where("org_id = ?", id).ToSql()
	if err != nil {
		return err
	}
	return a.db.BulkExec(map[string][]interface{}{
		stmt:  args,
		pstmt: pargs,
	})
}
