package http_test

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	dhttp "github.com/dioad/net/http"
)

// Example demonstrates encoding and decoding structs to/from HTTP headers
func Example() {
	// Define a struct to encode
	type RequestMetadata struct {
		User    string
		Session string
		Tags    []string
	}

	metadata := RequestMetadata{
		User:    "user-123",
		Session: "session-456",
		Tags:    []string{"tag1", "tag2", "tag3"},
	}

	// Configure marshaling with a prefix and struct name
	opts := dhttp.HeaderMarshalOptions{
		Prefix:            "X",
		IncludeStructName: true,
	}

	// Marshal to HTTP header
	headers, err := dhttp.MarshalHeader(metadata, opts)
	if err != nil {
		log.Fatal(err)
	}

	// Print the headers
	fmt.Println("Encoded headers:")
	fmt.Printf("  X-Request-Metadata-User: %s\n", headers.Get("X-Request-Metadata-User"))
	fmt.Printf("  X-Request-Metadata-Session: %s\n", headers.Get("X-Request-Metadata-Session"))
	fmt.Printf("  X-Request-Metadata-Tags: %v\n", headers.Values("X-Request-Metadata-Tags"))

	// Unmarshal back to a struct
	var decoded RequestMetadata
	err = dhttp.UnmarshalHeader(headers, &decoded, opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nDecoded struct:\n")
	fmt.Printf("  User: %s\n", decoded.User)
	fmt.Printf("  Session: %s\n", decoded.Session)
	fmt.Printf("  Tags: %v\n", decoded.Tags)

	// Output:
	// Encoded headers:
	//   X-Request-Metadata-User: user-123
	//   X-Request-Metadata-Session: session-456
	//   X-Request-Metadata-Tags: [tag1 tag2 tag3]
	//
	// Decoded struct:
	//   User: user-123
	//   Session: session-456
	//   Tags: [tag1 tag2 tag3]
}

// ExampleMarshalHeader demonstrates basic marshaling of a struct to headers
func ExampleMarshalHeader() {
	type UserInfo struct {
		Name  string   `header:"X-User-Name"`
		Roles []string `header:"X-User-Roles"`
		ID    int      `header:"X-User-ID"`
	}

	user := UserInfo{
		Name:  "Jane Doe",
		Roles: []string{"admin", "editor"},
		ID:    12345,
	}

	// Use DefaultHeaderMarshalOptions which doesn't add any prefix or struct name.
	// Field names will be determined by the `header` tag if present,
	// or automatically converted to kebab-case.
	header, err := dhttp.MarshalHeader(user, dhttp.DefaultHeaderMarshalOptions())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("X-User-Name: %s\n", header.Get("X-User-Name"))
	fmt.Printf("X-User-Roles: %v\n", header.Values("X-User-Roles"))
	fmt.Printf("X-User-ID: %s\n", header.Get("X-User-ID"))

	// Output:
	// X-User-Name: Jane Doe
	// X-User-Roles: [admin editor]
	// X-User-ID: 12345
}

// ExampleUnmarshalHeader demonstrates basic unmarshaling from headers to a struct
func ExampleUnmarshalHeader() {
	type UserInfo struct {
		Name  string   `header:"X-User-Name"`
		Roles []string `header:"X-User-Roles"`
		ID    int      `header:"X-User-ID"`
	}

	header := http.Header{}
	header.Set("X-User-Name", "John Doe")
	header.Add("X-User-Roles", "viewer")
	header.Add("X-User-Roles", "support")
	header.Set("X-User-ID", "67890")

	var user UserInfo
	err := dhttp.UnmarshalHeader(header, &user, dhttp.DefaultHeaderMarshalOptions())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Name: %s\n", user.Name)
	fmt.Printf("Roles: %v\n", user.Roles)
	fmt.Printf("ID: %d\n", user.ID)

	// Output:
	// Name: John Doe
	// Roles: [viewer support]
	// ID: 67890
}

// ExampleMarshalHeader_options demonstrates usage of HeaderMarshalOptions to customize header names
func ExampleMarshalHeader_options() {
	type AppConfig struct {
		Enabled bool
		Timeout int
	}

	cfg := AppConfig{
		Enabled: true,
		Timeout: 30,
	}

	// Using options to add a prefix and include struct name in header names.
	// Field names are automatically converted to kebab-case.
	opts := dhttp.HeaderMarshalOptions{
		Prefix:            "X-App",
		IncludeStructName: true,
	}

	header, err := dhttp.MarshalHeader(cfg, opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Header names are generated as Prefix-StructName-FieldName (in kebab-case)
	// and canonicalized by http.Header.
	fmt.Printf("X-App-App-Config-Enabled: %s\n", header.Get("X-App-App-Config-Enabled"))
	fmt.Printf("X-App-App-Config-Timeout: %s\n", header.Get("X-App-App-Config-Timeout"))

	// Output:
	// X-App-App-Config-Enabled: true
	// X-App-App-Config-Timeout: 30
}

// ExampleMarshalHeader_withoutStructName demonstrates encoding without the struct name in headers
func ExampleMarshalHeader_withoutStructName() {
	type Config struct {
		ApiKey string
		Region string
	}

	config := Config{
		ApiKey: "secret-key",
		Region: "us-west-2",
	}

	// Configure without struct name
	opts := dhttp.HeaderMarshalOptions{
		Prefix:            "X",
		IncludeStructName: false,
	}

	headers, err := dhttp.MarshalHeader(config, opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Headers without struct name:")
	fmt.Printf("  X-Api-Key: %s\n", headers.Get("X-Api-Key"))
	fmt.Printf("  X-Region: %s\n", headers.Get("X-Region"))

	// Output:
	// Headers without struct name:
	//   X-Api-Key: secret-key
	//   X-Region: us-west-2
}

// ExampleMarshalHeader_customTags demonstrates using custom header tags
func ExampleMarshalHeader_customTags() {
	type Metadata struct {
		RequestID string `header:"request-id"`
		TraceID   string `header:"trace-id"`
		Internal  string `header:"-"` // This field will be ignored
	}

	metadata := Metadata{
		RequestID: "req-789",
		TraceID:   "trace-abc",
		Internal:  "should-not-appear",
	}

	opts := dhttp.DefaultHeaderMarshalOptions()

	headers, err := dhttp.MarshalHeader(metadata, opts)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Headers with custom tags:")
	fmt.Printf("  request-id: %s\n", headers.Get("request-id"))
	fmt.Printf("  trace-id: %s\n", headers.Get("trace-id"))
	fmt.Printf("  internal present: %v\n", headers.Get("internal") != "")

	// Output:
	// Headers with custom tags:
	//   request-id: req-789
	//   trace-id: trace-abc
	//   internal present: false
}

// ExampleUnmarshalHeader_middleware demonstrates using header marshaling in HTTP middleware
func ExampleUnmarshalHeader_middleware() {
	type RequestContext struct {
		TenantId string
		UserRole string
	}

	// Create a middleware that decodes headers into a struct
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			opts := dhttp.HeaderMarshalOptions{
				Prefix:            "X-Context",
				IncludeStructName: false,
			}

			var ctx RequestContext
			if err := dhttp.UnmarshalHeader(r.Header, &ctx, opts); err != nil {
				http.Error(w, "Invalid context headers", http.StatusBadRequest)
				return
			}

			// Use the decoded context
			fmt.Printf("Processing request for tenant: %s, role: %s\n", ctx.TenantId, ctx.UserRole)
			next.ServeHTTP(w, r)
		})
	}

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	wrappedHandler := middleware(handler)

	// Simulate a request with headers
	req, _ := http.NewRequest("GET", "/api/data", nil)
	req.Header.Set("X-Context-Tenant-Id", "tenant-123")
	req.Header.Set("X-Context-User-Role", "admin")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Process the request
	wrappedHandler.ServeHTTP(w, req)

	// Output:
	// Processing request for tenant: tenant-123, role: admin
}

// ExampleMarshalHeader_rfc9110Compliance demonstrates RFC 9110 compliance
// for handling multiple header occurrences
func ExampleMarshalHeader_rfc9110Compliance() {
	type DataList struct {
		Items []string
	}

	// Data with values containing commas
	data := DataList{
		Items: []string{"item1", "item2,with,comma", "item3"},
	}

	opts := dhttp.DefaultHeaderMarshalOptions()

	// Marshal to headers
	headers, _ := dhttp.MarshalHeader(data, opts)

	// Show how multiple occurrences are created (RFC 9110 Section 5.5)
	fmt.Println("Multiple header occurrences:")
	for _, item := range headers.Values("items") {
		fmt.Printf("  items: %s\n", item)
	}

	// Unmarshal back - values are preserved exactly
	var result DataList
	dhttp.UnmarshalHeader(headers, &result, opts)

	fmt.Printf("\nUnmarshaled values:\n")
	for i, item := range result.Items {
		fmt.Printf("  [%d]: %s\n", i, item)
	}

	// Output:
	// Multiple header occurrences:
	//   items: item1
	//   items: item2,with,comma
	//   items: item3
	//
	// Unmarshaled values:
	//   [0]: item1
	//   [1]: item2,with,comma
	//   [2]: item3
}
