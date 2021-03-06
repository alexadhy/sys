package repo

import (
	"context"
	"fmt"

	sharedConfig "go.amplifyedge.org/sys-share-v2/sys-core/service/config"
	"go.amplifyedge.org/sys-v2/sys-account/service/go/pkg/dao"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	sharedAuth "go.amplifyedge.org/sys-share-v2/sys-account/service/go/pkg/shared"
	rpc "go.amplifyedge.org/sys-share-v2/sys-account/service/go/rpc/v2"
	coresvc "go.amplifyedge.org/sys-v2/sys-core/service/go/pkg/coredb"
)

func (ad *SysAccountRepo) projectFetchOrg(req *dao.Project) (*rpc.Project, error) {
	org, err := ad.store.GetOrg(&coresvc.QueryParams{Params: map[string]interface{}{"id": req.OrgId}})
	if err != nil {
		return nil, err
	}
	orgLogo, err := ad.frepo.DownloadFile("", org.LogoResourceId)
	if err != nil {
		return nil, err
	}
	pkgOrg, err := org.ToRpcOrg(nil, orgLogo.Binary)
	if err != nil {
		return nil, err
	}
	projectLogo, err := ad.frepo.DownloadFile("", req.LogoResourceId)
	if err != nil {
		return nil, err
	}
	return req.ToRpcProject(pkgOrg, projectLogo.Binary)
}

func (ad *SysAccountRepo) NewProject(ctx context.Context, in *rpc.ProjectRequest) (*rpc.Project, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot insert project: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	params := map[string]interface{}{}
	if in.OrgId != "" {
		params["id"] = in.OrgId
	}
	if in.OrgName != "" {
		params["name"] = in.OrgName
	}
	// check org existence
	o, err := ad.store.GetOrg(&coresvc.QueryParams{Params: params})
	if err != nil {
		return nil, err
	}
	var logoBytes []byte
	if in.LogoUploadBytes != "" {
		logoBytes, err = sharedConfig.DecodeB64(in.LogoUploadBytes)
	}
	// do the permission check here
	if err = ad.allowNewProject(ctx, in.OrgId); err != nil {
		return nil, err
	}
	logo, err := ad.frepo.UploadFile(in.LogoFilepath, logoBytes)
	if err != nil {
		return nil, err
	}
	// this is the key
	in.LogoFilepath = logo.ResourceId
	in.OrgId = o.Id
	req, err := ad.store.FromRpcProject(in)
	if err != nil {
		return nil, err
	}
	if err = ad.store.InsertProject(req); err != nil {
		return nil, err
	}
	proj, err := ad.store.GetProject(&coresvc.QueryParams{Params: map[string]interface{}{"id": req.Id}})
	if err != nil {
		return nil, err
	}
	return ad.projectFetchOrg(proj)
}

func (ad *SysAccountRepo) GetProject(ctx context.Context, in *rpc.IdRequest) (*rpc.Project, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot get project: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	params := map[string]interface{}{}
	if in.Id != "" {
		params["id"] = in.Id
	}
	if in.Name != "" {
		params["name"] = in.Name
	}
	proj, err := ad.store.GetProject(&coresvc.QueryParams{Params: params})
	if err != nil {
		return nil, err
	}
	return ad.projectFetchOrg(proj)
}

func (ad *SysAccountRepo) ListProject(ctx context.Context, in *rpc.ListRequest) (*rpc.ListResponse, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list project: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	var limit, cursor int64
	limit = in.PerPageEntries
	orderBy := in.OrderBy
	var err error
	filtersJson := map[string]interface{}{}
	if err = sharedConfig.UnmarshalJson(in.GetFilters(), &filtersJson); err != nil {
		return nil, err
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
	projects, next, err := ad.store.ListProject(filter, orderBy, limit, cursor, in.Matcher)
	var pkgProjects []*rpc.Project
	for _, p := range projects {
		pkgProject, err := ad.projectFetchOrg(p)
		if err != nil {
			return nil, err
		}
		pkgProjects = append(pkgProjects, pkgProject)
	}
	return &rpc.ListResponse{
		Projects:   pkgProjects,
		NextPageId: fmt.Sprintf("%d", next),
	}, nil
}

func (ad *SysAccountRepo) UpdateProject(ctx context.Context, in *rpc.ProjectUpdateRequest) (*rpc.Project, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list project: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	proj, err := ad.store.GetProject(&coresvc.QueryParams{Params: map[string]interface{}{"id": in.Id}})
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		proj.Name = in.Name
	}
	if err = ad.allowUpdateDeleteProject(ctx, proj.OrgId, proj.Id); err != nil {
		return nil, err
	}
	if in.LogoFilepath != "" && len(in.LogoUploadBytes) != 0 {
		updatedLogo, err := ad.frepo.UploadFile(in.LogoFilepath, in.LogoUploadBytes)
		if err != nil {
			return nil, err
		}
		proj.LogoResourceId = updatedLogo.ResourceId
	}
	err = ad.store.UpdateProject(proj)
	if err != nil {
		return nil, err
	}
	proj, err = ad.store.GetProject(&coresvc.QueryParams{Params: map[string]interface{}{"id": proj.Id}})
	if err != nil {
		return nil, err
	}
	return ad.projectFetchOrg(proj)
}

func (ad *SysAccountRepo) DeleteProject(ctx context.Context, in *rpc.IdRequest) (*emptypb.Empty, error) {
	if in == nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot list project: %v", sharedAuth.Error{Reason: sharedAuth.ErrInvalidParameters})
	}
	proj, err := ad.store.GetProject(&coresvc.QueryParams{Params: map[string]interface{}{"id": in.Id}})
	if err != nil {
		return nil, err
	}
	if err = ad.allowUpdateDeleteProject(ctx, proj.OrgId, proj.Id); err != nil {
		return nil, err
	}
	err = ad.store.DeleteProject(in.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
