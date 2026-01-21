package prefixlist

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// ProviderConstructor is a function that creates a Provider from configuration.
// It receives a ProviderConfig and returns a Provider instance or an error.
//
// Custom providers can implement their own constructors and register them
// using RegisterProvider in an init() function. This allows for easy extension
// of the provider factory without modifying the core factory code.
type ProviderConstructor func(cfg ProviderConfig) (Provider, error)

// providerRegistry holds registered provider constructors.
var (
	providerRegistry   = make(map[string]ProviderConstructor)
	providerRegistryMu sync.RWMutex
)

// RegisterProvider registers a provider constructor for a given provider name.
// The name is case-insensitive and will be normalized to lowercase.
//
// This function is typically called in an init() function in the provider's source file.
// For example, to add a new provider:
//
//	func init() {
//	    RegisterProvider("myprovider", func(cfg ProviderConfig) (Provider, error) {
//	        // Parse configuration and create provider
//	        return NewMyProvider(cfg.Filter["option"]), nil
//	    })
//	}
//
// This registration-based approach reduces cyclomatic complexity by eliminating
// the need for a large switch statement in the factory function.
func RegisterProvider(name string, constructor ProviderConstructor) {
	providerRegistryMu.Lock()
	defer providerRegistryMu.Unlock()
	providerRegistry[strings.ToLower(name)] = constructor
}

// NewProviderFromConfig creates a provider instance from configuration.
// It looks up the provider by name in the registry and invokes its constructor.
// The provider must be registered via RegisterProvider before it can be instantiated.
func NewProviderFromConfig(cfg ProviderConfig) (Provider, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("provider %s is not enabled", cfg.Name)
	}

	name := strings.ToLower(cfg.Name)

	providerRegistryMu.RLock()
	constructor, ok := providerRegistry[name]
	providerRegistryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", cfg.Name)
	}

	return constructor(cfg)
}

// NewMultiProviderFromConfig creates a MultiProvider from configuration
func NewMultiProviderFromConfig(cfg Config, logger zerolog.Logger) (*MultiProvider, error) {
	var providers []Provider

	for _, providerCfg := range cfg.Providers {
		provider, err := NewProviderFromConfig(providerCfg)
		if err != nil {
			logger.Warn().Err(err).Str("provider", providerCfg.Name).Msg("failed to create provider")
			continue
		}

		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no valid providers configured")
	}

	return NewMultiProvider(providers, logger), nil
}
