package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed dashboard.html
var assets embed.FS

func DashboardPage() ([]byte, error) {
	return readDashboardPage(assets)
}

func readDashboardPage(fsys fs.FS) ([]byte, error) {
	content, err := fs.ReadFile(fsys, "dashboard.html")
	if err != nil {
		return nil, fmt.Errorf("read dashboard page: %w", err)
	}

	return content, nil
}
