package main

import (
	"fmt"
	"strings"
)

type config struct {
	newsdataKey string
	mongoURI    string
}

// loadConfig reads the two required variables. A missing one is reported by name only:
// values never appear in an error.
func loadConfig(getenv func(string) string) (config, error) {
	c := config{newsdataKey: getenv("NEWSDATA_API_KEY"), mongoURI: getenv("MONGODB_URI")}
	var missing []string
	if c.newsdataKey == "" {
		missing = append(missing, "NEWSDATA_API_KEY")
	}
	if c.mongoURI == "" {
		missing = append(missing, "MONGODB_URI")
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	return c, nil
}
