package repo

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/amplify-cms/sys-share/sys-account/service/go/pkg"
	sharedAuth "github.com/amplify-cms/sys-share/sys-account/service/go/pkg/shared"
	sharedConfig "github.com/amplify-cms/sys-share/sys-core/service/config"
	"github.com/amplify-cms/sys/sys-account/service/go/pkg/dao"
	coresvc "github.com/amplify-cms/sys/sys-core/service/go/pkg/coredb"
)

func (ad *SysAccountRepo) NewOrg(ctx context.Context, in *pkg.OrgRequest) (*pkg.Org, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot insert org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	var err error
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
	req, err := ad.store.FromPkgOrgRequest(in, "")
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
	return org.ToPkgOrg(nil, logoFile.Binary)
}

func (ad *SysAccountRepo) orgFetchProjects(org *dao.Org) (*pkg.Org, error) {
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
			return org.ToPkgOrg(nil, orgLogo.Binary)
		}
		return nil, err
	}
	if len(projects) > 0 {
		var pkgProjects []*pkg.Project
		for _, p := range projects {
			projectLogo, err := ad.frepo.DownloadFile("", p.LogoResourceId)
			if err != nil {
				return nil, err
			}
			proj, err := p.ToPkgProject(nil, projectLogo.Binary)
			if err != nil {
				return nil, err
			}
			pkgProjects = append(pkgProjects, proj)
		}
		return org.ToPkgOrg(pkgProjects, orgLogo.Binary)
	}
	return org.ToPkgOrg(nil, orgLogo.Binary)
}

func (ad *SysAccountRepo) GetOrg(ctx context.Context, in *pkg.IdRequest) (*pkg.Org, error) {
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

func (ad *SysAccountRepo) ListOrg(ctx context.Context, in *pkg.ListRequest) (*pkg.ListResponse, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	var limit, cursor int64
	limit = in.PerPageEntries
	orderBy := in.OrderBy
	var err error
	filter := &coresvc.QueryParams{Params: in.Filters}
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
	var pkgOrgs []*pkg.Org
	for _, org := range orgs {
		pkgOrg, err := ad.orgFetchProjects(org)
		if err != nil {
			return nil, err
		}
		pkgOrgs = append(pkgOrgs, pkgOrg)
	}
	return &pkg.ListResponse{
		Orgs:       pkgOrgs,
		NextPageId: fmt.Sprintf("%d", next),
	}, nil
}

func (ad *SysAccountRepo) ListNonSubscribedOrgs(ctx context.Context, in *pkg.ListRequest) (*pkg.ListResponse, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	var limit, cursor int64
	limit = in.PerPageEntries
	orderBy := in.OrderBy
	var err error
	filter := &coresvc.QueryParams{Params: in.Filters}
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
	orgs, next, err := ad.store.ListNonSubbed(in.AccountId, filter, orderBy, limit, cursor)
	var pkgOrgs []*pkg.Org
	for _, org := range orgs {
		pkgOrg, err := ad.orgFetchProjects(org)
		if err != nil {
			return nil, err
		}
		pkgOrgs = append(pkgOrgs, pkgOrg)
	}
	return &pkg.ListResponse{
		Orgs:       pkgOrgs,
		NextPageId: fmt.Sprintf("%d", next),
	}, nil
}

func (ad *SysAccountRepo) UpdateOrg(ctx context.Context, in *pkg.OrgUpdateRequest) (*pkg.Org, error) {
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

func (ad *SysAccountRepo) DeleteOrg(ctx context.Context, in *pkg.IdRequest) (*emptypb.Empty, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list org: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	org, err := ad.GetOrg(ctx, &pkg.IdRequest{Id: in.Id})
	if err != nil {
		return nil, err
	}
	for _, proj := range org.Projects {
		if _, err = ad.DeleteProject(ctx, &pkg.IdRequest{Id: proj.Id}); err != nil {
			return nil, err
		}
	}
	err = ad.store.DeleteOrg(in.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
