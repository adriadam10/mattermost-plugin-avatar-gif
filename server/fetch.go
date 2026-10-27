package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var (
	errBadURL      = errors.New("only https URLs are allowed")
	errPrivateHost = errors.New("URL points to a private address")
	errFileTooBig  = errors.New("file too large")
)

// Sharing pages (Giphy, Tenor...) are HTML; their og:image is the GIF itself.
var ogImage = regexp.MustCompile(`<meta[^>]+property=["']og:image["'][^>]+content=["']([^"']+)["']`)

// Users choose the URL, so the server must not be usable to reach internal services.
var httpClient = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
			Control: rejectPrivate,
		}).DialContext,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "https" {
			return errBadURL
		}
		return nil
	},
}

func rejectPrivate(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !isPublic(ip) {
		return errPrivateHost
	}
	return nil
}

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func isPublic(ip net.IP) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !cgnat.Contains(ip)
}

// fetchGIF downloads a GIF, following the og:image of a sharing page if needed.
func fetchGIF(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	data, err := download(ctx, rawURL, maxBytes)
	if err != nil || isGIF(data) {
		return data, err
	}
	image := pickOgImage(string(data))
	if image == "" {
		return nil, errNotGIF
	}
	return download(ctx, image, maxBytes)
}

// pickOgImage prefers a .gif among the page's og:image tags: Giphy lists a .webp first.
func pickOgImage(page string) string {
	var first string
	for _, m := range ogImage.FindAllStringSubmatch(page, -1) {
		link := strings.ReplaceAll(m[1], "&amp;", "&")
		if u, err := url.Parse(link); err == nil && strings.HasSuffix(u.Path, ".gif") {
			return link
		}
		if first == "" {
			first = link
		}
	}
	return first
}

func download(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errBadURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/gif,text/html;q=0.5")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errFileTooBig
	}
	return data, nil
}
