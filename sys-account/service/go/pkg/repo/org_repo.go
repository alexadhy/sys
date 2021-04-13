package repo

import (
	"context"
	"fmt"
	sharedAuth "go.amplifyedge.org/sys-share-v2/sys-account/service/go/pkg/shared"
	rpc "go.amplifyedge.org/sys-share-v2/sys-account/service/go/rpc/v2"
	sharedConfig "go.amplifyedge.org/sys-share-v2/sys-core/service/config"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/dao"
	coresvc "go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/coredb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"sync"
)

func (ad *SysAccountRepo) NewOrg(ctx context.Context, in *rpc.OrgRequest) (*rpc.Org, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot insert org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	var err error
	if err = ad.allowNewOrg(ctx); err != nil {
		return nil, err
	}
	var logoBytes []byte
	if in.LogoUploadBytes != "" {
		logoBytes, err = sharedConfig.DecodeB64(in.LogoUploadBytes)
	}
	logo, err := ad.frepo.UploadFile(in.LogoFilepath, logoBytes)
	if err != nil {
		return nil, err
	}
	// this is the key
	in.LogoFilepath = logo.ResourceId
	req, err := ad.store.FromrpcOrgRequest(in, "")
	if err != nil {
		ad.log.Debugf("unable to convert org request to dao object: %v", err)
		return nil, err
	}
	ad.log.Debugf("New Org Input: %v", req)
	if err = ad.store.InsertOrg(req); err != nil {
		ad.log.Debugf("unable to insert new org to db: %v", err)
		return nil, err
	}
	org, err := ad.store.GetOrg(&coresvc.QueryParams{Params: map[string]interface{}{"id": req.Id}})
	if err != nil {
		ad.log.Debugf("unable to get new org from db: %v", err)
		return nil, err
	}
	logoFile, err := ad.frepo.DownloadFile("", logo.ResourceId)
	if err != nil {
		return nil, err
	}
	return org.ToRpcOrg(nil, logoFile.Binary)
}

func (ad *SysAccountRepo) orgFetchProjects(org *dao.Org) (*rpc.Org, error) {
	orgLogo, err := ad.frepo.DownloadFile("", org.LogoResourceId)
	if err != nil {
		return nil, err
	}
	projects, _, err := ad.store.ListProject(
		&coresvc.QueryParams{Params: map[string]interface{}{"org_id": org.Id}},
		"name ASC", dao.DefaultLimit, 0, "eq",
	)
	if err != nil {
		if err.Error() == "document not found" {
			return org.ToRpcOrg(nil, orgLogo.Binary)
		}
		return nil, err
	}
	if len(projects) > 0 {
		var pkgProjects []*rpc.Project
		for _, p := range projects {
			projectLogo, err := ad.frepo.DownloadFile("", p.LogoResourceId)
			if err != nil {
				return nil, err
			}
			proj, err := p.ToRpcProject(nil, projectLogo.Binary)
			if err != nil {
				return nil, err
			}
			pkgProjects = append(pkgProjects, proj)
		}
		return org.ToRpcOrg(pkgProjects, orgLogo.Binary)
	}
	return org.ToRpcOrg(nil, orgLogo.Binary)
}

func (ad *SysAccountRepo) GetOrg(ctx context.Context, in *rpc.IdRequest) (*rpc.Org, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot get org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	params := map[string]interface{}{}
	if in.Id != "" {
		params["id"] = in.Id
	}
	if in.Name != "" {
		params["name"] = in.Name
	}
	org, err := ad.store.GetOrg(&coresvc.QueryParams{Params: params})
	if err != nil {
		return nil, err
	}
	return ad.orgFetchProjects(org)
}

func (ad *SysAccountRepo) ListOrg(ctx context.Context, in *rpc.ListRequest) (*rpc.ListResponse, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	var accRoles []*rpc.UserRoles
	_, acc, _ := ad.accountFromClaims(ctx)
	if acc != nil {
		accRoles = acc.GetRoles()
	}
	var limit, cursor int64
	limit = in.PerPageEntries
	orderBy := in.OrderBy
	var err error
	filtersJson := map[string]interface{}{}
	if err = sharedConfig.UnmarshalJson(in.GetFilters(), &filtersJson); err != nil {
		filtersJson = map[string]interface{}{}
	}
	filter := &coresvc.QueryParams{Params: filtersJson}
	if in.IsDescending {
		orderBy += " DESC"
	} else {
		orderBy += " ASC"
	}
	cursor, err = ad.getCursor(in.CurrentPageId)
	if err != nil {
		return nil, err
	}
	if limit == 0 {
		limit = dao.DefaultLimit
	}
	orgs, next, err := ad.store.ListOrg(filter, orderBy, limit, cursor, in.Matcher)
	var pkgOrgs []*rpc.Org
	for _, org := range orgs {
		pkgOrg, err := ad.orgFetchProjects(org)
		if err != nil {
			return nil, err
		}
		var subbed []*rpc.Project
		var wg sync.WaitGroup
		if in.GetSubscribedOnly() {
			ad.filterSubscribedOrgProj(
				pkgOrg,
				accRoles,
				func(r *rpc.UserRoles, proj *rpc.Project) {
					if r.GetProjectId() == proj.GetId() {
						subbed = append(subbed, proj)
					}
				},
				&wg,
			)
		} else {
			ad.filterSubscribedOrgProj(
				pkgOrg,
				accRoles,
				func(r *rpc.UserRoles, proj *rpc.Project) {
					if r.GetProjectId() != proj.GetId() {
						subbed = append(subbed, proj)
					}
				},
				&wg,
			)
		}
		wg.Wait()
		pkgOrg.Projects = subbed
		if len(pkgOrg.GetProjects()) > 0 {
			pkgOrgs = append(pkgOrgs, pkgOrg)
		}
	}
	return &rpc.ListResponse{
		Orgs:       pkgOrgs,
		NextPageId: fmt.Sprintf("%d", next),
	}, nil
}

// filter subscribed orgs and projects
func (ad *SysAccountRepo) filterSubscribedOrgProj(
	org *rpc.Org,
	accRoles []*rpc.UserRoles,
	filterFunc func(r *rpc.UserRoles, proj *rpc.Project),
	wg *sync.WaitGroup,
) {
	for _, p := range org.GetProjects() {
		wg.Add(1)
		go func(proj *rpc.Project, o *rpc.Org) {
			for _, r := range accRoles {
				filterFunc(r, proj)
			}
			wg.Done()
		}(p, org)
	}
}

func (ad *SysAccountRepo) UpdateOrg(ctx context.Context, in *rpc.OrgUpdateRequest) (*rpc.Org, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	org, err := ad.store.GetOrg(&coresvc.QueryParams{Params: map[string]interface{}{"id": in.Id}})
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		org.Name = in.Name
	}
	if err = ad.allowUpdateDeleteOrg(ctx, org.Id); err != nil {
		return nil, err
	}
	if in.LogoFilepath != "" && len(in.LogoUploadBytes) != 0 {
		updatedLogo, err := ad.frepo.UploadFile(in.LogoFilepath, in.LogoUploadBytes)
		if err != nil {
			return nil, err
		}
		org.LogoResourceId = updatedLogo.ResourceId
	}
	if in.Contact != "" {
		org.Contact = in.Contact
	}
	ad.log.Debugf("Updated org: %v", org)
	err = ad.store.UpdateOrg(org)
	if err != nil {
		return nil, err
	}
	org, err = ad.store.GetOrg(&coresvc.QueryParams{Params: map[string]interface{}{"id": org.Id}})
	return ad.orgFetchProjects(org)
}

func (ad *SysAccountRepo) DeleteOrg(ctx context.Context, in *rpc.IdRequest) (*emptypb.Empty, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	org, err := ad.GetOrg(ctx, &rpc.IdRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}
	if err = ad.allowUpdateDeleteOrg(ctx, org.Id); err != nil {
		return nil, err
	}
	for _, proj := range org.Projects {
		if _, err = ad.DeleteProject(ctx, &rpc.IdRequest{Id: proj.Id}); err != nil {
			return nil, err
		}
	}
	err = ad.store.DeleteOrg(in.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
