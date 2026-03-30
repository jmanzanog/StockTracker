package web

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

type failingFS struct{}

func (failingFS) Open(name string) (fs.File, error) {
	return nil, errors.New("boom")
}

func TestDashboardPage(t *testing.T) {
	content, err := DashboardPage()
	if err != nil {
		t.Fatalf("expected embedded dashboard page, got error: %v", err)
	}

	body := string(content)
	if !strings.Contains(body, "Stock Tracker Dashboard") {
		t.Fatalf("expected dashboard title in embedded page")
	}

	if !strings.Contains(body, "api/v1/dashboard?sparklines=7") {
		t.Fatalf("expected dashboard bootstrap URL in embedded page")
	}
}

func TestReadDashboardPage_Success(t *testing.T) {
	fsys := fstest.MapFS{
		"dashboard.html": &fstest.MapFile{Data: []byte("<html>ok</html>")},
	}

	content, err := readDashboardPage(fsys)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if got := string(content); got != "<html>ok</html>" {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestReadDashboardPage_Error(t *testing.T) {
	_, err := readDashboardPage(failingFS{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "read dashboard page") {
		t.Fatalf("expected wrapped read error, got: %v", err)
	}
}
