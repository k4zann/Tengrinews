// Package ui embeds the HTML templates and static assets, so the binary runs from any directory.
package ui

import "embed"

// Files holds templates/ and assets/. The all: prefix keeps the _*.scss files, which are served today.
//
//go:embed templates all:assets
var Files embed.FS
