package prefixlist

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// filterNameCase is a test case for a provider constructor of the shape
// func(a, b []string) Provider, where Name() encodes both filter lists into
// the provider's name.
type filterNameCase struct {
	name     string
	a        []string
	b        []string
	expected string
}

// runFilterNameCases exercises newProvider against each case, asserting the
// resulting provider's Name(). Shared by TestGoogleProviderWithFilters and
// TestAtlassianProviderWithFilters, whose constructors differ only in the
// semantic meaning of their two []string filter parameters (scopes/services
// vs. regions/products), not in shape or behaviour.
func runFilterNameCases(t *testing.T, newProvider func(a, b []string) Provider, tests []filterNameCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newProvider(tt.a, tt.b)
			assert.Equal(t, tt.expected, provider.Name())
		})
	}
}

func TestGoogleProviderWithFilters(t *testing.T) {
	runFilterNameCases(t, func(scopes, services []string) Provider {
		return NewGoogleProvider(scopes, services)
	}, []filterNameCase{
		{name: "no filters", a: nil, b: nil, expected: "google"},
		{name: "with scope", a: []string{"us-central1"}, b: nil, expected: "google-us-central1"},
		{name: "with service", a: nil, b: []string{"Google Cloud"}, expected: "google-Google Cloud"},
		{
			name:     "with scope and service",
			a:        []string{"us-central1"},
			b:        []string{"Google Cloud"},
			expected: "google-Google Cloud-us-central1",
		},
		{
			name:     "with multiple scopes and services",
			a:        []string{"us-central1", "europe-west1"},
			b:        []string{"Google Cloud", "Google Cloud Storage"},
			expected: "google-Google Cloud,Google Cloud Storage-us-central1,europe-west1",
		},
	})
}

func TestAtlassianProviderWithFilters(t *testing.T) {
	runFilterNameCases(t, func(regions, products []string) Provider {
		return NewAtlassianProvider(regions, products)
	}, []filterNameCase{
		{name: "no filters", a: nil, b: nil, expected: "atlassian"},
		{name: "with region", a: []string{"global"}, b: nil, expected: "atlassian-global"},
		{name: "with product", a: nil, b: []string{"jira"}, expected: "atlassian-jira"},
		{
			name:     "with region and product",
			a:        []string{"global"},
			b:        []string{"jira"},
			expected: "atlassian-jira-global",
		},
		{
			name:     "with multiple regions and products",
			a:        []string{"global", "us-east-1"},
			b:        []string{"jira", "confluence"},
			expected: "atlassian-jira,confluence-global,us-east-1",
		},
	})
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
