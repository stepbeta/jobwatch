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

func TestSourceFromURLWTTJ(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantName string
		wantType model.SourceType
	}{
		{
			name:     "company careers page",
			url:      "https://www.welcometothejungle.com/en/companies/fabriq/jobs",
			wantName: "fabriq",
			wantType: model.SourceWTTJ,
		},
		{
			name:     "host without www",
			url:      "https://welcometothejungle.com/fr/companies/joko",
			wantName: "joko",
			wantType: model.SourceWTTJ,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := sourceFromURL(tt.url, "", "", "")
			if err != nil {
				t.Fatalf("sourceFromURL: %v", err)
			}
			if src.Name != tt.wantName || src.Type != tt.wantType || src.ConfigJSON != "{}" {
				t.Errorf("sourceFromURL = name %q type %q config %s, want %q %q {}",
					src.Name, src.Type, src.ConfigJSON, tt.wantName, tt.wantType)
			}
		})
	}

	// The marketplace pages are not company pages and cannot be detected.
	if _, err := sourceFromURL("https://www.welcometothejungle.com/en/jobs?query=sre", "", "", ""); err == nil {
		t.Error("marketplace URL was detected as a source")
	}
	if _, err := sourceFromURL("https://www.welcometothejungle.com/en/jobs?query=sre", "", "wttj", ""); err != nil {
		t.Errorf("explicit --type wttj rejected: %v", err)
	}
}
