package wait_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

//
// https://github.com/testcontainers/testcontainers-go/issues/183
func ExampleHTTPStrategy() {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "gogs/gogs:0.11.91",
		ExposedPorts: []string{"3000/tcp"},
		WaitingFor:   wait.ForHTTP("/").WithPort("3000/tcp"),
	}

	gogs, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		panic(err)
	}

	// Here you have a running container

	_ = gogs.Terminate(ctx)
}

func TestHTTPStrategyWaitUntilReady(t *testing.T) {
	workdir, err := os.Getwd()
	if err != nil {
		t.Error(err)
		return
	}

	capath := workdir + "/testdata/root.pem"
	cafile, err := ioutil.ReadFile(capath)
	if err != nil {
		t.Errorf("can't load ca file: %v", err)
		return
	}

	certpool := x509.NewCertPool()
	if !certpool.AppendCertsFromPEM(cafile) {
		t.Errorf("the ca file isn't valid")
		return
	}

	tlsconfig := &tls.Config{RootCAs: certpool, ServerName: "testcontainer.go.test"}
	dockerReq := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context: workdir + "/testdata",
		},
		ExposedPorts: []string{"6443/tcp"},
		WaitingFor: wait.NewHTTPStrategy("/").WithTLS(true, tlsconfig).
			WithStartupTimeout(time.Second * 10).WithPort("6443/tcp").
			WithMethod(http.MethodPost).WithBody(bytes.NewReader([]byte("ping"))),
	}

	t.Log("creating container")
	container, err := testcontainers.GenericContainer(context.Background(),
		testcontainers.GenericContainerRequest{ContainerRequest: dockerReq, Started: true})
	if err != nil {
		t.Error(err)
		return
	}
	defer container.Terminate(context.Background()) // nolint: errcheck

	t.Log("requesting")
	host, err := container.Host(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	port, err := container.MappedPort(context.Background(), "6443/tcp")
	if err != nil {
		t.Error(err)
		return
	}
	client := http.Client{Transport: &http.Transport{TLSClientConfig: tlsconfig}}
	resp, err := client.Get(fmt.Sprintf("https://%s:%s/ping", host, port.Port()))
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()

	t.Log("verify http status code")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status code isn't ok: %s", resp.Status)
		return
	}

	t.Log("verify response data")
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
		return
	}
	if string(data) != "pong" {
		t.Errorf("should returns 'pong'")
	}
}
