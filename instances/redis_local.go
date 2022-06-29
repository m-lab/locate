package instances

import (
	"context"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/m-lab/go/rtx"
)

func MustNewLocalRedisClient() *redisDatastoreClient {
	cli, err := client.NewClientWithOpts()
	rtx.Must(err, "failed to initialize a Docker client")

	ctx := context.Background()
	imageName := "redis"
	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	rtx.Must(err, "failed to pull Redis image")

	defer out.Close()
	io.Copy(os.Stdout, out)

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	rtx.Must(err, "failed to list containers")

	for _, container := range containers {
		if container.Image == imageName {
			err = cli.ContainerStop(ctx, container.ID, nil)
			rtx.Must(err, "failed to stop existing container")
		}
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Cmd:   strslice.StrSlice{"redis-server"},
		ExposedPorts: nat.PortSet{
			"6379": struct{}{},
		},
	}, &container.HostConfig{
		PortBindings: nat.PortMap{
			"6379": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "6379",
				},
			},
		},
	}, nil, nil, "")
	rtx.Must(err, "failed to create Docker container")

	err = cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	rtx.Must(err, "failed to start Docker container")

	return NewRedisDatastoreClient("localhost:6379")
}
