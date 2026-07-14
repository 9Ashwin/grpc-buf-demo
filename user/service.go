package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"buf.build/go/protovalidate"
	"github.com/9Ashwin/grpc-buf-demo/db"
	commonv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/common/v1"
	userv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Service struct {
	database *sql.DB
	queries  *db.Queries
}

func NewService(database *sql.DB) *Service {
	return &Service{database: database, queries: db.New(database)}
}

func (s *Service) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	created, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Name:      req.GetName(),
		Email:     req.GetEmail(),
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Errorf(codes.AlreadyExists, "email %q already exists", req.GetEmail())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "creating user")
	}

	user, err := toProto(created)
	if err != nil {
		return nil, status.Error(codes.Internal, "reading created user")
	}
	return &userv1.CreateUserResponse{User: user}, nil
}

func (s *Service) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	id, err := parseUserID(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	found, err := s.queries.GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Errorf(codes.NotFound, "user %q not found", req.GetId())
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "getting user")
	}
	user, err := toProto(found)
	if err != nil {
		return nil, status.Error(codes.Internal, "reading user")
	}
	return &userv1.GetUserResponse{User: user}, nil
}

func (s *Service) ListUsers(ctx context.Context, req *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	offset, err := pageOffset(req.GetPageToken())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	pageSize := int64(req.GetPageSize())
	if pageSize == 0 {
		pageSize = 20
	}

	tx, err := s.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, status.Error(codes.Internal, "starting list transaction")
	}
	defer func() { _ = tx.Rollback() }()
	queries := s.queries.WithTx(tx)
	total, err := queries.CountUsers(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "counting users")
	}
	if offset > total {
		return nil, status.Error(codes.InvalidArgument, "page_token is outside the result set")
	}
	rows, err := queries.ListUsers(ctx, db.ListUsersParams{PageOffset: offset, PageSize: pageSize})
	if err != nil {
		return nil, status.Error(codes.Internal, "listing users")
	}
	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "committing list transaction")
	}

	users := make([]*userv1.User, 0, len(rows))
	for _, row := range rows {
		user, err := toProto(row)
		if err != nil {
			return nil, status.Error(codes.Internal, "reading listed user")
		}
		users = append(users, user)
	}
	nextToken := ""
	if next := offset + int64(len(rows)); next < total {
		nextToken = strconv.FormatInt(next, 10)
	}
	return &userv1.ListUsersResponse{
		Users: users,
		Page: &commonv1.PageInfo{
			NextPageToken: nextToken,
			TotalSize:     int32(total),
		},
	}, nil
}

func (s *Service) WatchUsers(req *userv1.WatchUsersRequest, stream grpc.ServerStreamingServer[userv1.WatchUsersResponse]) error {
	if err := protovalidate.Validate(req); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	users, err := s.queries.ListAllUsers(stream.Context())
	if err != nil {
		return status.Error(codes.Internal, "listing users for watch")
	}
	for _, current := range users {
		user, err := toProto(current)
		if err != nil {
			return status.Error(codes.Internal, "reading watched user")
		}
		if err := stream.Send(&userv1.WatchUsersResponse{User: user}); err != nil {
			return fmt.Errorf("sending user %q: %w", user.GetId(), err)
		}
	}
	return nil
}

func toProto(user db.User) (*userv1.User, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at for user %d: %w", user.ID, err)
	}
	return &userv1.User{
		Id:        fmt.Sprintf("user-%03d", user.ID),
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: timestamppb.New(createdAt),
	}, nil
}

func parseUserID(value string) (int64, error) {
	number, ok := strings.CutPrefix(value, "user-")
	if !ok {
		return 0, fmt.Errorf("id must use the user-NNN format")
	}
	id, err := strconv.ParseInt(number, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("id must use the user-NNN format")
	}
	return id, nil
}

func pageOffset(token string) (int64, error) {
	if token == "" {
		return 0, nil
	}
	offset, err := strconv.ParseInt(token, 10, 64)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("page_token must be a non-negative integer")
	}
	return offset, nil
}
