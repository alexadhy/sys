package repo

import (
	"context"
	rpc "go.amplifyedge.org/sys-share-v2/sys-account/service/go/rpc/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strconv"

	sharedAuth "go.amplifyedge.org/sys-share-v2/sys-account/service/go/pkg/shared"
	"go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/coredb"
	fileDao "go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/filesvc/dao"
)

func (ad *SysAccountRepo) getAccountAndRole(ctx context.Context, id, email string) (*rpc.Account, error) {
	var super *rpc.Account
	queryParams := map[string]interface{}{}
	if id != "" {
		queryParams["id"] = id
	}
	if email != "" {
		queryParams["email"] = email
	}
	acc, err := ad.store.GetAccount(&coredb.QueryParams{Params: queryParams})
	if err != nil {
		var param string
		if id != "" {
			param = id
		}
		if email != "" {
			param = email
		}
		super, err = ad.superDao.Get(ctx, param)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "cannot find user account: %v", sharedAuth.Error{Reason: sharedAuth.ErrAccountNotFound})
		}
		return super, nil
	}
	daoRoles, err := ad.store.ListRole(&coredb.QueryParams{Params: map[string]interface{}{
		"account_id": acc.ID,
	}})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "cannot find user role: %v", sharedAuth.Error{Reason: sharedAuth.ErrAccountNotFound})
	}
	var pkgRoles []*rpc.UserRoles
	for _, daoRole := range daoRoles {
		pkgRole, err := daoRole.ToPkgRole()
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "cannot find user role: %v", sharedAuth.Error{Reason: sharedAuth.ErrAccountNotFound})
		}
		pkgRoles = append(pkgRoles, pkgRole)
	}
	var avatar *fileDao.File
	if acc.AvatarResourceId != "" {
		avatar, err = ad.frepo.DownloadFile("", acc.AvatarResourceId)
		if err != nil {
			return nil, err
		}
		return acc.ToRpcAccount(pkgRoles, avatar.Binary)
	}
	return acc.ToRpcAccount(pkgRoles, nil)
}

func (ad *SysAccountRepo) listAccountsAndRoles(ctx context.Context, filter *coredb.QueryParams, orderBy string, limit, cursor int64, sqlMatcher string) ([]*rpc.Account, *int64, error) {
	listAccounts, next, err := ad.store.ListAccount(filter, orderBy, limit, cursor, sqlMatcher)
	if err != nil {
		return nil, nil, err
	}
	var accounts []*rpc.Account

	for _, acc := range listAccounts {
		daoRoles, err := ad.store.ListRole(&coredb.QueryParams{Params: map[string]interface{}{
			"account_id": acc.ID,
		}})
		if err != nil {
			return nil, nil, status.Errorf(codes.NotFound, "cannot find user roles: %v", sharedAuth.Error{Reason: sharedAuth.ErrAccountNotFound, Err: err})
		}
		var pkgRoles []*rpc.UserRoles
		for _, daoRole := range daoRoles {
			pkgRole, err := daoRole.ToPkgRole()
			if err != nil {
				return nil, nil, status.Errorf(codes.NotFound, "cannot find user roles: %v", sharedAuth.Error{Reason: sharedAuth.ErrAccountNotFound, Err: err})
			}
			pkgRoles = append(pkgRoles, pkgRole)
		}
		var avatar *fileDao.File
		var account *rpc.Account
		if acc.AvatarResourceId != "" {
			avatar, err = ad.frepo.DownloadFile("", acc.AvatarResourceId)
			if err != nil {
				return nil, nil, err
			}
			account, err = acc.ToRpcAccount(pkgRoles, avatar.Binary)
		} else {
			account, err = acc.ToRpcAccount(pkgRoles, nil)
		}
		if err != nil {
			return nil, nil, err
		}
		accounts = append(accounts, account)
	}

	// superuser
	if len(filter.Params) == 0 {
		supers, err := ad.superDao.List(ctx, "")
		if err != nil {
			return nil, nil, err
		}
		accounts = append(accounts, supers...)
	}

	return accounts, &next, nil
}

func (ad *SysAccountRepo) getCursor(currentCursor string) (int64, error) {
	if currentCursor != "" {
		return strconv.ParseInt(currentCursor, 10, 64)
	} else {
		return 0, nil
	}
}
