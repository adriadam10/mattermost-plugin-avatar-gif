package main

import (
	"strconv"
	"strings"
)

type configuration struct {
	MaxSizeMB int
	MaxFrames int
}

func defaultConfiguration() *configuration {
	return &configuration{MaxSizeMB: 5, MaxFrames: 200}
}

func (c *configuration) maxBytes() int64 { return int64(c.MaxSizeMB) << 20 }
func (c *configuration) maxFrames() int  { return c.MaxFrames }

// rawConfiguration takes any: a number setting may come as a number or as a string.
type rawConfiguration struct {
	MaxSizeMB any
	MaxFrames any
}

func (r rawConfiguration) parse() *configuration {
	c := defaultConfiguration()
	if n := positive(r.MaxSizeMB); n > 0 {
		c.MaxSizeMB = n
	}
	if n := positive(r.MaxFrames); n > 0 {
		c.MaxFrames = n
	}
	return c
}

func positive(v any) int {
	switch v := v.(type) {
	case float64:
		return max(int(v), 0)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return max(n, 0)
	}
	return 0
}
