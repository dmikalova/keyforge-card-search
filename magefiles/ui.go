//go:build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// UI holds the browser checks of the search page.
type UI mg.Namespace

// Check builds the site, starts it on a free port, and runs the uicheck
// module in headless Chrome at a desk width and a phone width. The site uses
// the fixture dataset, or DATASET_URL if it is set. The screenshots go to
// tmp/ui. It needs Chrome, so it is not part of ci:check.
func (UI) Check() error {
	bin := "tmp/ui/search"
	if err := sh.RunV("go", "build", "-o", bin, "./cmd/search"); err != nil {
		return err
	}

	port, err := freePort()
	if err != nil {
		return err
	}

	srv := exec.Command(bin)
	srv.Env = append(os.Environ(), "PORT="+strconv.Itoa(port))
	srv.Stderr = os.Stderr

	if err := srv.Start(); err != nil {
		return err
	}

	defer func() { _ = srv.Process.Kill() }()

	base := "http://localhost:" + strconv.Itoa(port)
	if err := waitUp(base + "/health"); err != nil {
		return err
	}

	out, err := os.Getwd()
	if err != nil {
		return err
	}

	return sh.RunV("go", "-C", "uicheck", "run", ".", "-base", base, "-out", out+"/tmp/ui")
}

// freePort returns a TCP port that no program listens on.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	defer func() { _ = l.Close() }()

	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("no TCP address")
	}

	return addr.Port, nil
}

// waitUp waits until the URL answers 200, for up to 30 seconds.
func waitUp(u string) error {
	for range 60 {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("the site did not start: %s", u)
}
