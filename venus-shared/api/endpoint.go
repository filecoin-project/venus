package api

import (
	"fmt"
	"net/url"
)

func Endpoint(raw string, ver uint32) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}

	if u.Scheme == "" {
		return "", fmt.Errorf("scheme is required")
	}

	if u.Host == "" {
		return "", fmt.Errorf("host is required")
	}

	// raw url contains more than just scheme://host(:prot)
	if u.Path != "" && u.Path != "/" {
		return raw, nil
	}

	return fmt.Sprintf("%s://%s/rpc/v%d", u.Scheme, u.Host, ver), nil
}
