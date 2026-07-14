package user_test

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net"
	"net/url"
	"testing"

	"github.com/9Ashwin/grpc-buf-demo/db"
	userv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/user/v1"
	"github.com/9Ashwin/grpc-buf-demo/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestUserService(t *testing.T) {
	database := newTestDatabase(t)
	client := newTestClient(t, database)
	ctx := t.Context()

	if _, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "Ada", Email: "not-an-email"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CreateUser() code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}

	created, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "Ada", Email: "ada@example.com"})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.GetUser().GetId() != "user-001" {
		t.Fatalf("CreateUser() id = %q, want %q", created.GetUser().GetId(), "user-001")
	}
	if _, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "Ada", Email: "ada@example.com"}); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("CreateUser(duplicate) code = %v, want %v", status.Code(err), codes.AlreadyExists)
	}
	second, err := client.CreateUser(ctx, &userv1.CreateUserRequest{Name: "Grace", Email: "grace@example.com"})
	if err != nil {
		t.Fatalf("CreateUser(second) error = %v", err)
	}

	got, err := client.GetUser(ctx, &userv1.GetUserRequest{Id: created.GetUser().GetId()})
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.GetUser().GetEmail() != created.GetUser().GetEmail() {
		t.Fatalf("GetUser() email = %q, want %q", got.GetUser().GetEmail(), created.GetUser().GetEmail())
	}

	list, err := client.ListUsers(ctx, &userv1.ListUsersRequest{PageSize: 1})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if list.GetPage().GetTotalSize() != 2 || list.GetPage().GetNextPageToken() != "1" || len(list.GetUsers()) != 1 {
		t.Fatalf("ListUsers() = %+v, want first of two users", list)
	}
	next, err := client.ListUsers(ctx, &userv1.ListUsersRequest{PageSize: 1, PageToken: list.GetPage().GetNextPageToken()})
	if err != nil {
		t.Fatalf("ListUsers(next) error = %v", err)
	}
	if len(next.GetUsers()) != 1 || next.GetUsers()[0].GetId() != second.GetUser().GetId() || next.GetPage().GetNextPageToken() != "" {
		t.Fatalf("ListUsers(next) = %+v, want second user and no next page", next)
	}

	stream, err := client.WatchUsers(ctx, &userv1.WatchUsersRequest{})
	if err != nil {
		t.Fatalf("WatchUsers() error = %v", err)
	}
	streamed, err := stream.Recv()
	if err != nil {
		t.Fatalf("WatchUsers().Recv() error = %v", err)
	}
	if streamed.GetUser().GetId() != created.GetUser().GetId() {
		t.Fatalf("WatchUsers().Recv() id = %q, want %q", streamed.GetUser().GetId(), created.GetUser().GetId())
	}
	streamed, err = stream.Recv()
	if err != nil {
		t.Fatalf("WatchUsers().Recv(second) error = %v", err)
	}
	if streamed.GetUser().GetId() != second.GetUser().GetId() {
		t.Fatalf("WatchUsers().Recv(second) id = %q, want %q", streamed.GetUser().GetId(), second.GetUser().GetId())
	}
	if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("WatchUsers().Recv() final error = %v, want EOF", err)
	}
}

func newTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := db.Open(t.Context(), "file:"+url.QueryEscape(t.Name())+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func newTestClient(t *testing.T, database *sql.DB) userv1.UserServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	userv1.RegisterUserServiceServer(server, user.NewService(database))
	go func() {
		if err := server.Serve(listener); err != nil {
			t.Errorf("Serve() error = %v", err)
		}
	}()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn)
}
