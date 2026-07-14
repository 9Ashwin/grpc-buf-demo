package user_test

import (
	"context"
	"io"
	"net"
	"testing"

	userv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/user/v1"
	"github.com/9Ashwin/grpc-buf-demo/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestUserService(t *testing.T) {
	client := newTestClient(t)
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
	if list.GetPage().GetTotalSize() != 1 || len(list.GetUsers()) != 1 {
		t.Fatalf("ListUsers() = %+v, want one user", list)
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
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("WatchUsers().Recv() final error = %v, want EOF", err)
	}
}

func newTestClient(t *testing.T) userv1.UserServiceClient {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	userv1.RegisterUserServiceServer(server, user.NewService())
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
