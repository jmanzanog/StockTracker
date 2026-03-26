package web

import (
	"embed"
	"fmt"
)

//go:embed dashboard.html
var assets embed.FS

func DashboardPage() ([]byte, error) {
	content, err := assets.ReadFile("dashboard.html")
	if err != nil {
		return nil, fmt.Errorf("read dashboard page: %w", err)
	}

	return content, nil
}
