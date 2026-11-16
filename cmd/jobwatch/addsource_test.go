package main

import (
	"testing"

	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

func TestSourceFromURLBambooHR(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		typeFlag   string
		config     string
		wantName   string
		wantType   model.SourceType
		wantConfig string
	}{
		{
			name:       "careers url",
			url:        "https://semble.bamboohr.com/careers",
			wantName:   "semble",
			wantType:   model.SourceBambooHR,
			wantConfig: `{"subdomain":"semble"}`,
		},
		{
			name:       "listing url",
			url:        "https://semble.bamboohr.com/careers/list",
			wantName:   "semble",
			wantType:   model.SourceBambooHR,
			wantConfig: `{"subdomain":"semble"}`,
		},
		{
			name:       "explicit type with config override",
			url:        "https://careers.example.com/jobs",
			typeFlag:   "bamboohr",
			config:     `{"subdomain":"acme","company":"Acme"}`,
			wantName:   "careers.example.com",
			wantType:   model.SourceBambooHR,
			wantConfig: `{"company":"Acme","subdomain":"acme"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := sourceFromURL(tt.url, "", tt.typeFlag, tt.config)
			if err != nil {
				t.Fatalf("sourceFromURL: %v", err)
			}
			if src.Name != tt.wantName || src.Type != tt.wantType || src.ConfigJSON != tt.wantConfig {
				t.Errorf("sourceFromURL = name %q type %q config %s, want %q %q %s",
					src.Name, src.Type, src.ConfigJSON, tt.wantName, tt.wantType, tt.wantConfig)
			}
			if !src.Enabled {
				t.Error("source not enabled")
			}
			if tt.typeFlag == "" {
				typ, dc, ok := sources.Detect(tt.url)
				if !ok || typ != model.SourceBambooHR || dc.Subdomain != "semble" {
					t.Errorf("Detect = %q %+v %v", typ, dc, ok)
				}
			}
		})
	}
}
