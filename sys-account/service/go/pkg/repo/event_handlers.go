package repo

import (
	"context"
	"errors"
	"fmt"
	utilities "go.amplifyedge.org/sys-share-v2/sys-core/service/config"
	coreRpc "go.amplifyedge.org/sys-share-v2/sys-core/service/go/rpc/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sharedAuth "go.amplifyedge.org/sys-share-v2/sys-account/service/go/pkg/shared"
	rpc "go.amplifyedge.org/sys-share-v2/sys-account/service/go/rpc/v2"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/dao"

	sharedBus "go.amplifyedge.org/sys-share-v2/sys-core/service/go/pkg/bus"
	"go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/coredb"
)

func (ad *SysAccountRepo) onDeleteProject(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const projectIdKey = "project_id"
	deleteRequestMap, err := getEventIdMap(in, projectIdKey)
	if err != nil {
		return nil, err
	}
	projId := deleteRequestMap[projectIdKey].(string)
	_, err = ad.DeleteProject(ctx, &rpc.IdRequest{Id: projId})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"success":    true,
		"successMsg": fmt.Sprintf("successfully deleted project with id: %s", projId),
	}, nil
}

func (ad *SysAccountRepo) onDeleteOrg(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const orgIdKey = "org_id"
	deleteRequestMap, err := getEventIdMap(in, orgIdKey)
	if err != nil {
		return nil, err
	}
	orgId := deleteRequestMap[orgIdKey].(string)
	_, err = ad.DeleteOrg(ctx, &rpc.IdRequest{Id: orgId})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"success":    true,
		"successMsg": fmt.Sprintf("successfully deleted org with id: %s", orgId),
	}, nil
}

func (ad *SysAccountRepo) onDeleteAccount(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const accountIdKey = "account_id"
	deleteRequestMap, err := getEventIdMap(in, accountIdKey)
	if err != nil {
		return nil, err
	}
	accountId := deleteRequestMap[accountIdKey].(string)
	_, err = ad.DisableAccount(ctx, &rpc.DisableAccountRequest{AccountId: accountId})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"success":    true,
		"successMsg": fmt.Sprintf("successfully deleted account with id: %s", accountId),
	}, nil
}

func getEventIdMap(in *coreRpc.EventRequest, key string) (map[string]interface{}, error) {
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	if requestMap[key] == nil || requestMap[key].(string) == "" {
		return nil, sharedBus.Error{
			Reason: sharedBus.ErrInvalidEventPayload,
			Err:    fmt.Errorf("%s id is not valid", key),
		}
	}
	return requestMap, nil
}

func (ad *SysAccountRepo) onCheckProjectExists(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const projectIdKey = "sys_account_project_ref_id"
	const projectNameKey = "sys_account_project_ref_name"
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	rmap := map[string]interface{}{}
	if requestMap[projectIdKey] != nil && requestMap[projectIdKey].(string) != "" {
		rmap["id"] = requestMap[projectIdKey]
	}
	if requestMap[projectNameKey] != nil && requestMap[projectNameKey].(string) != "" {
		rmap["name"] = requestMap[projectNameKey]
	}
	proj, err := ad.store.GetProject(&coredb.QueryParams{Params: rmap})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"exists":     true,
		projectIdKey: proj.Id,
	}, nil
}

func (ad *SysAccountRepo) onCheckOrgExists(_ context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const orgIdKey = "sys_account_org_ref_id"
	const orgNameKey = "sys_account_org_ref_name"
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	rmap := map[string]interface{}{}
	if requestMap[orgIdKey] != nil && requestMap[orgIdKey].(string) != "" {
		rmap["id"] = requestMap[orgIdKey]
	}
	if requestMap[orgNameKey] != nil && requestMap[orgNameKey].(string) != "" {
		rmap["name"] = requestMap[orgNameKey]
	}
	org, err := ad.store.GetOrg(&coredb.QueryParams{Params: rmap})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"exists": true,
		orgIdKey: org.Id,
	}, nil
}

func (ad *SysAccountRepo) onCheckAccountExists(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const accountIdKey = "sys_account_user_ref_id"
	const accountNameKey = "sys_account_user_ref_name"
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	rmap := map[string]interface{}{}
	if requestMap[accountIdKey] != nil && requestMap[accountIdKey].(string) != "" {
		rmap["id"] = requestMap[accountIdKey]
	}
	if requestMap[accountNameKey] != nil && requestMap[accountNameKey].(string) != "" {
		rmap["email"] = requestMap[accountNameKey]
	}
	acc, err := ad.store.GetAccount(&coredb.QueryParams{Params: rmap})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"exists":     true,
		accountIdKey: acc.ID,
	}, nil
}

func (ad *SysAccountRepo) onGetAccountEmail(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const accountIdKey = "sys_account_user_ref_id"
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	rmap := map[string]interface{}{}
	if requestMap[accountIdKey] != nil && requestMap[accountIdKey].(string) != "" {
		rmap["id"] = requestMap[accountIdKey]
	}
	acc, err := ad.store.GetAccount(&coredb.QueryParams{Params: rmap})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"exists": true,
		"email":  acc.Email,
	}, nil
}

func (ad *SysAccountRepo) onResetAllSysAccount(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	if err := ad.store.ResetAll(); err != nil {
		return nil, err
	}
	return map[string]interface{}{}, nil
}

// onCheckAllowProject checks current user account claims see if they are allowed to do any data operations on DiscoProject
// general rule is only superadmin, org admin, or the project admin for the specific project are allowed to do anything.
func (ad *SysAccountRepo) onCheckAllowProject(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const orgIdKey = "org_id"
	const projectIdKey = "project_id"
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	var proj *dao.Project
	rmap := map[string]interface{}{}
	if requestMap[projectIdKey] == nil || requestMap[projectIdKey] == "" {
		return nil, err
	}
	rmap["id"] = requestMap[projectIdKey]
	qp := &coredb.QueryParams{Params: rmap}
	proj, err = ad.store.GetProject(qp)
	if err != nil {
		return nil, err
	}
	if err = ad.allowUpdateDeleteProject(ctx, proj.OrgId, proj.Id); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"allowed": true,
	}, nil

}

// onCheckAllowSurveyUser only allows either the superuesr (for backup reason) or the user itself to be able to
// update or delete survey user data.
func (ad *SysAccountRepo) onCheckAllowSurveyUser(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const accountIdKey = "user_id"
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	if requestMap[accountIdKey] == nil || requestMap[accountIdKey] == "" {
		return nil, status.Errorf(codes.InvalidArgument, sharedAuth.Error{
			Reason: sharedAuth.ErrInvalidParameters,
			Err:    errors.New("invalid argument: missing account id"),
		}.Error())
	}
	_, curAcc, err := ad.accountFromClaims(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, sharedAuth.Error{Reason: sharedAuth.ErrRequestUnauthenticated, Err: err}.Error())
	}
	// allow superadmin to do anything
	allowSelf := sharedAuth.AllowSelf(curAcc, requestMap[accountIdKey].(string))
	allowSupers := sharedAuth.IsSuperadmin(curAcc.GetRoles())
	if allowSelf || allowSupers {
		return map[string]interface{}{
			"allowed": true,
		}, nil
	}
	return map[string]interface{}{
		"allowed": false,
	}, nil
}

// onJoinNewProject called whenever user joins a new project
func (ad *SysAccountRepo) onJoinNewProject(ctx context.Context, in *coreRpc.EventRequest) (map[string]interface{}, error) {
	const projectIdKey = "sys_account_project_id"
	requestMap, err := coredb.UnmarshalToMap(in.JsonPayload)
	if err != nil {
		return nil, err
	}
	_, curAcc, err := ad.accountFromClaims(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, sharedAuth.Error{Reason: sharedAuth.ErrRequestUnauthenticated, Err: err}.Error())
	}
	// check project exist
	proj, err := ad.store.GetProject(&coredb.QueryParams{Params: map[string]interface{}{"id": requestMap[projectIdKey].(string)}})
	if err != nil {
		return nil, err
	}
	for _, r := range curAcc.GetRoles() {
		if r.ProjectId == proj.Id {
			return map[string]interface{}{"ok": true}, nil
		}
	}
	if err = ad.store.InsertRole(&dao.Role{
		ID:        utilities.NewID(),
		AccountId: curAcc.Id,
		Role:      int(rpc.Roles_USER),
		ProjectId: proj.Id,
		OrgId:     proj.OrgId,
		CreatedAt: utilities.CurrentTimestamp(),
		UpdatedAt: utilities.CurrentTimestamp(),
	}); err != nil {
		return nil, err
	}
	return map[string]interface{}{"ok": true}, nil
}
