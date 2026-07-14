package user

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"buf.build/go/protovalidate"
	commonv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/common/v1"
	userv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	mu      sync.RWMutex
	users   []*userv1.User
	byID    map[string]*userv1.User
	byEmail map[string]struct{}
	nextID  atomic.Uint64
}

func NewService() *Service {
	return &Service{
		byID:    make(map[string]*userv1.User),
		byEmail: make(map[string]struct{}),
	}
}

func (s *Service) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[req.GetEmail()]; exists {
		return nil, status.Errorf(codes.AlreadyExists, "email %q already exists", req.GetEmail())
	}

	id := fmt.Sprintf("user-%03d", s.nextID.Add(1))
	created := &userv1.User{
		Id:        id,
		Name:      req.GetName(),
		Email:     req.GetEmail(),
		CreatedAt: timestamppb.Now(),
	}
	s.users = append(s.users, created)
	s.byID[id] = created
	s.byEmail[created.GetEmail()] = struct{}{}
	return &userv1.CreateUserResponse{User: created}, nil
}

func (s *Service) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	found, ok := s.byID[req.GetId()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "user %q not found", req.GetId())
	}
	return &userv1.GetUserResponse{User: found}, nil
}

func (s *Service) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	offset, err := pageOffset(req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if offset > len(s.users) {
		return nil, status.Error(codes.InvalidArgument, "page_token is outside the result set")
	}
	pageSize := int(req.GetPageSize())
	if pageSize == 0 {
		pageSize = 20
	}
	end := min(offset+pageSize, len(s.users))
	nextToken := ""
	if end < len(s.users) {
		nextToken = strconv.Itoa(end)
	}

	return &userv1.ListUsersResponse{
		Users: append([]*userv1.User(nil), s.users[offset:end]...),
		Page: &commonv1.PageInfo{
			NextPageToken: nextToken,
			TotalSize:     int32(len(s.users)),
		},
	}, nil
}

func (s *Service) WatchUsers(_ *userv1.WatchUsersRequest, stream grpc.ServerStreamingServer[userv1.WatchUsersResponse]) error {
	s.mu.RLock()
	users := append([]*userv1.User(nil), s.users...)
	s.mu.RUnlock()

	for _, current := range users {
		if err := stream.Send(&userv1.WatchUsersResponse{User: current}); err != nil {
			return fmt.Errorf("sending user %q: %w", current.GetId(), err)
		}
	}
	return nil
}

func pageOffset(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(token)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("page_token must be a non-negative integer")
	}
	return offset, nil
}
