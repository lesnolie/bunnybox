//go:build e2e

package main

// The local automation socket is opt-in and excluded from ordinary builds.
import _ "github.com/egoist/mygo/plugins/automation"
