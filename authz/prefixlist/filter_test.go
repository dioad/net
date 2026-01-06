package prefixlist

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoogleProviderWithFilters(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		services []string
		expected string
	}{
		{
			name:     "no filters",
			scopes:   nil,
			services: nil,
			expected: "google",
		},
		{
			name:     "with scope",
			scopes:   []string{"us-central1"},
			services: nil,
			expected: "google-us-central1",
		},
		{
			name:     "with service",
			scopes:   nil,
			services: []string{"Google Cloud"},
			expected: "google-Google Cloud",
		},
		{
			name:     "with scope and service",
			scopes:   []string{"us-central1"},
			services: []string{"Google Cloud"},
			expected: "google-Google Cloud-us-central1",
		},
		{
			name:     "with multiple scopes and services",
			scopes:   []string{"us-central1", "europe-west1"},
			services: []string{"Google Cloud", "Google Cloud Storage"},
			expected: "google-Google Cloud,Google Cloud Storage-us-central1,europe-west1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewGoogleProvider(tt.scopes, tt.services)
			assert.Equal(t, tt.expected, provider.Name())
		})
	}
}

func TestAtlassianProviderWithFilters(t *testing.T) {
	tests := []struct {
		name     string
		regions  []string
		products []string
		expected string
	}{
		{
			name:     "no filters",
			regions:  nil,
			products: nil,
			expected: "atlassian",
		},
		{
			name:     "with region",
			regions:  []string{"global"},
			products: nil,
			expected: "atlassian-global",
		},
		{
			name:     "with product",
			regions:  nil,
			products: []string{"jira"},
			expected: "atlassian-jira",
		},
		{
			name:     "with region and product",
			regions:  []string{"global"},
			products: []string{"jira"},
			expected: "atlassian-jira-global",
		},
		{
			name:     "with multiple regions and products",
			regions:  []string{"global", "us-east-1"},
			products: []string{"jira", "confluence"},
			expected: "atlassian-jira,confluence-global,us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewAtlassianProvider(tt.regions, tt.products)
			assert.Equal(t, tt.expected, provider.Name())
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item in slice",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "item not in slice",
			slice:    []string{"apple", "banana", "cherry"},
			item:     "orange",
			expected: false,
		},
		{
			name:     "case insensitive match",
			slice:    []string{"Apple", "Banana", "Cherry"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "banana",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name     string
		haystack []string
		needles  []string
		expected bool
	}{
		{
			name:     "one match",
			haystack: []string{"apple", "banana", "cherry"},
			needles:  []string{"banana"},
			expected: true,
		},
		{
			name:     "multiple needles with match",
			haystack: []string{"apple", "banana", "cherry"},
			needles:  []string{"orange", "banana"},
			expected: true,
		},
		{
			name:     "no match",
			haystack: []string{"apple", "banana", "cherry"},
			needles:  []string{"orange", "grape"},
			expected: false,
		},
		{
			name:     "empty needles",
			haystack: []string{"apple", "banana", "cherry"},
			needles:  []string{},
			expected: true, // empty needles means no filter
		},
		{
			name:     "case insensitive",
			haystack: []string{"Apple", "Banana", "Cherry"},
			needles:  []string{"banana"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsAny(tt.haystack, tt.needles)
			assert.Equal(t, tt.expected, result)
		})
	}
}
