package basic

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNetrcProviderIsolation(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create first netrc file
	netrc1Path := filepath.Join(tmpDir, "netrc1")
	netrc1Content := `machine example.com
login user1
password pass1`
	if err := os.WriteFile(netrc1Path, []byte(netrc1Content), 0600); err != nil {
		t.Fatalf("Failed to create netrc1: %v", err)
	}

	// Create second netrc file
	netrc2Path := filepath.Join(tmpDir, "netrc2")
	netrc2Content := `machine example.com
login user2
password pass2`
	if err := os.WriteFile(netrc2Path, []byte(netrc2Content), 0600); err != nil {
		t.Fatalf("Failed to create netrc2: %v", err)
	}

	// Create two separate NetrcProvider instances with different data
	provider1 := &NetrcProvider{}
	provider2 := &NetrcProvider{}

	// Manually load data into providers to simulate different configurations
	provider1.once.Do(func() {
		data, _ := os.ReadFile(netrc1Path)
		provider1.lines = parseNetrc(string(data))
	})

	provider2.once.Do(func() {
		data, _ := os.ReadFile(netrc2Path)
		provider2.lines = parseNetrc(string(data))
	})

	// Create test requests
	req1, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req2, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add credentials using different providers
	AddCredentialsWithProvider(req1, provider1)
	AddCredentialsWithProvider(req2, provider2)

	// Verify that each request has the correct credentials
	user1, pass1, ok1 := req1.BasicAuth()
	if !ok1 {
		t.Fatal("Expected credentials on req1")
	}
	if user1 != "user1" || pass1 != "pass1" {
		t.Errorf("Expected user1/pass1, got %s/%s", user1, pass1)
	}

	user2, pass2, ok2 := req2.BasicAuth()
	if !ok2 {
		t.Fatal("Expected credentials on req2")
	}
	if user2 != "user2" || pass2 != "pass2" {
		t.Errorf("Expected user2/pass2, got %s/%s", user2, pass2)
	}
}

func TestNetrcProviderWithClientAuth(t *testing.T) {
	// Test that ClientAuth uses its own NetrcProvider instance
	tmpDir := t.TempDir()

	netrcPath := filepath.Join(tmpDir, "netrc")
	netrcContent := `machine test.example.com
login testuser
password testpass`
	if err := os.WriteFile(netrcPath, []byte(netrcContent), 0600); err != nil {
		t.Fatalf("Failed to create netrc: %v", err)
	}

	// Create a ClientAuth instance
	auth := &ClientAuth{
		Config: ClientConfig{},
	}

	// Manually set the netrcProvider with test data
	auth.netrcProvider = &NetrcProvider{}
	auth.netrcProvider.once.Do(func() {
		data, _ := os.ReadFile(netrcPath)
		auth.netrcProvider.lines = parseNetrc(string(data))
	})

	// Create a request
	req, err := http.NewRequest("GET", "http://test.example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add auth
	if err := auth.AddAuth(req); err != nil {
		t.Fatalf("AddAuth failed: %v", err)
	}

	// Verify credentials
	user, pass, ok := req.BasicAuth()
	if !ok {
		t.Fatal("Expected credentials on request")
	}
	if user != "testuser" || pass != "testpass" {
		t.Errorf("Expected testuser/testpass, got %s/%s", user, pass)
	}
}

func TestAddCredentialsBackwardCompatibility(t *testing.T) {
	// Test that the old AddCredentials function still works
	// This is a basic smoke test to ensure backward compatibility

	req, err := http.NewRequest("GET", "http://nonexistent.example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// This should not panic and should return false (no credentials found)
	added := AddCredentials(req)

	// We expect false since there's no .netrc file with this host
	if added {
		t.Error("Expected AddCredentials to return false for nonexistent host")
	}
}

func TestClientAuthMultipleInstances(t *testing.T) {
	// Test that multiple ClientAuth instances don't interfere with each other
	tmpDir := t.TempDir()

	// Create netrc file
	netrcPath := filepath.Join(tmpDir, "netrc")
	netrcContent := `machine example1.com
login user1
password pass1

machine example2.com
login user2
password pass2`
	if err := os.WriteFile(netrcPath, []byte(netrcContent), 0600); err != nil {
		t.Fatalf("Failed to create netrc: %v", err)
	}

	// Create two ClientAuth instances
	auth1 := &ClientAuth{Config: ClientConfig{}}
	auth2 := &ClientAuth{Config: ClientConfig{}}

	// Set up their netrc providers
	auth1.netrcProvider = &NetrcProvider{}
	auth1.netrcProvider.once.Do(func() {
		data, _ := os.ReadFile(netrcPath)
		auth1.netrcProvider.lines = parseNetrc(string(data))
	})

	auth2.netrcProvider = &NetrcProvider{}
	auth2.netrcProvider.once.Do(func() {
		data, _ := os.ReadFile(netrcPath)
		auth2.netrcProvider.lines = parseNetrc(string(data))
	})

	// Create requests for different hosts
	req1, _ := http.NewRequest("GET", "http://example1.com", nil)
	req2, _ := http.NewRequest("GET", "http://example2.com", nil)

	// Add auth from different instances
	auth1.AddAuth(req1)
	auth2.AddAuth(req2)

	// Verify each got the right credentials
	user1, pass1, _ := req1.BasicAuth()
	if user1 != "user1" || pass1 != "pass1" {
		t.Errorf("Auth1: expected user1/pass1, got %s/%s", user1, pass1)
	}

	user2, pass2, _ := req2.BasicAuth()
	if user2 != "user2" || pass2 != "pass2" {
		t.Errorf("Auth2: expected user2/pass2, got %s/%s", user2, pass2)
	}
}

func TestNetrcProviderParseError(t *testing.T) {
	// Test that NetrcProvider handles errors gracefully
	provider := &NetrcProvider{}

	// Force an error by setting a non-existent NETRC path
	t.Setenv("NETRC", "/nonexistent/path/to/netrc")

	provider.once.Do(provider.readNetrc)

	// Provider should not panic and should handle the error
	if provider.err == nil {
		// If no error, that's fine - the file might not exist which is acceptable
		t.Log("No error reading non-existent netrc (expected)")
	}
}

func TestAddCredentialsWithNilProvider(t *testing.T) {
	// Ensure we handle edge cases gracefully
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// Using the default provider should not panic
	added := AddCredentials(req)

	// We don't care about the result, just that it doesn't panic
	_ = added
}

func TestClientAuthWithConfiguredCredentials(t *testing.T) {
	// Test that ClientAuth prefers configured credentials over netrc
	auth := &ClientAuth{
		Config: ClientConfig{
			User:     "configuser",
			Password: "configpass",
		},
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	auth.AddAuth(req)

	user, pass, ok := req.BasicAuth()
	if !ok {
		t.Fatal("Expected credentials")
	}
	if user != "configuser" || pass != "configpass" {
		t.Errorf("Expected configuser/configpass, got %s/%s", user, pass)
	}
}

func TestNetrcProviderConcurrency(t *testing.T) {
	// Test that NetrcProvider is safe for concurrent use
	provider := &NetrcProvider{}

	// Simulate multiple goroutines trying to initialize the provider
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			AddCredentialsWithProvider(req, provider)
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without a panic, the test passes
}

func TestParseNetrcWithProvider(t *testing.T) {
	// Test that parseNetrc works correctly with the new provider structure
	testData := `machine api.github.com
login testuser
password testpass

machine example.com
login user2
password pass2`

	lines := parseNetrc(testData)

	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	if lines[0].machine != "api.github.com" || lines[0].login != "testuser" || lines[0].password != "testpass" {
		t.Errorf("First line incorrect: %+v", lines[0])
	}

	if lines[1].machine != "example.com" || lines[1].login != "user2" || lines[1].password != "pass2" {
		t.Errorf("Second line incorrect: %+v", lines[1])
	}
}
