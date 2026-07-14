package integration_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	userv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/user/v1"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServerContainer(t *testing.T) {
	if os.Getenv("RUN_CONTAINER_TESTS") != "1" {
		t.Skip("set RUN_CONTAINER_TESTS=1 to run the container test")
	}
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx := t.Context()
	repositoryRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	container, err := testcontainers.Run(
		ctx,
		"",
		testcontainers.WithDockerfile(testcontainers.FromDockerfile{
			Context:    repositoryRoot,
			Dockerfile: "Dockerfile",
		}),
		testcontainers.WithExposedPorts("8080/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("8080/tcp").WithStartupTimeout(3*time.Minute),
		),
	)
	if err != nil {
		t.Fatalf("testcontainers.Run() error = %v", err)
	}
	testcontainers.CleanupContainer(t, container)

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("Container.Endpoint() error = %v", err)
	}
	connection, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })

	client := userv1.NewUserServiceClient(connection)
	created, err := client.CreateUser(ctx, &userv1.CreateUserRequest{
		Name:  "Grace Hopper",
		Email: "grace@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if created.GetUser().GetId() != "user-001" {
		t.Fatalf("CreateUser() id = %q, want %q", created.GetUser().GetId(), "user-001")
	}
}
