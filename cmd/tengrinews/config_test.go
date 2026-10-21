package main

import (
	"strings"
	"testing"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadConfig(t *testing.T) {
	const key, uri = "k3y-marker", "mongodb://user:s3cr3t@host/"
	tests := []struct {
		name    string
		env     map[string]string
		want    config
		missing []string // names the error must contain
		absent  []string // text the error must not contain
	}{
		{name: "both set", env: map[string]string{"NEWSDATA_API_KEY": key, "MONGODB_URI": uri}, want: config{key, uri}},
		{name: "both missing", env: nil, missing: []string{"NEWSDATA_API_KEY", "MONGODB_URI"}},
		{name: "both empty", env: map[string]string{"NEWSDATA_API_KEY": "", "MONGODB_URI": ""}, missing: []string{"NEWSDATA_API_KEY", "MONGODB_URI"}},
		{
			name:    "key missing",
			env:     map[string]string{"MONGODB_URI": uri},
			missing: []string{"NEWSDATA_API_KEY"},
			absent:  []string{"MONGODB_URI", "s3cr3t"},
		},
		{
			name:    "uri missing",
			env:     map[string]string{"NEWSDATA_API_KEY": key},
			missing: []string{"MONGODB_URI"},
			absent:  []string{"NEWSDATA_API_KEY", key},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(envOf(tt.env))
			if len(tt.missing) == 0 {
				if err != nil || got != tt.want {
					t.Fatalf("loadConfig = %+v, %v; want %+v, nil", got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("loadConfig = %+v, nil; want an error", got)
			}
			for _, name := range tt.missing {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("error %q does not name %s", err, name)
				}
			}
			for _, s := range tt.absent {
				if strings.Contains(err.Error(), s) {
					t.Errorf("error %q contains %q", err, s)
				}
			}
		})
	}
}
