package xcclient

import (
	"context"
	"encoding/json"
)

// XCClient is the interface satisfied by *Client. It covers all CRUD methods
// for the seven supported F5 XC resources plus the ClientNeedsUpdate helper.
//
// A compile-time conformance check in client_test.go ensures *Client always
// satisfies this interface.
type XCClient interface {
	// HTTPLoadBalancer
	CreateHTTPLoadBalancer(ctx context.Context, ns string, lb *HTTPLoadBalancerCreate) (*HTTPLoadBalancer, error)
	GetHTTPLoadBalancer(ctx context.Context, ns, name string) (*HTTPLoadBalancer, error)
	ReplaceHTTPLoadBalancer(ctx context.Context, ns, name string, lb *HTTPLoadBalancerReplace) (*HTTPLoadBalancer, error)
	DeleteHTTPLoadBalancer(ctx context.Context, ns, name string) error
	ListHTTPLoadBalancers(ctx context.Context, ns string) ([]*HTTPLoadBalancer, error)

	// TCPLoadBalancer
	CreateTCPLoadBalancer(ctx context.Context, ns string, lb *TCPLoadBalancerCreate) (*TCPLoadBalancer, error)
	GetTCPLoadBalancer(ctx context.Context, ns, name string) (*TCPLoadBalancer, error)
	ReplaceTCPLoadBalancer(ctx context.Context, ns, name string, lb *TCPLoadBalancerReplace) (*TCPLoadBalancer, error)
	DeleteTCPLoadBalancer(ctx context.Context, ns, name string) error
	ListTCPLoadBalancers(ctx context.Context, ns string) ([]*TCPLoadBalancer, error)

	// OriginPool
	CreateOriginPool(ctx context.Context, ns string, pool *OriginPoolCreate) (*OriginPool, error)
	GetOriginPool(ctx context.Context, ns, name string) (*OriginPool, error)
	ReplaceOriginPool(ctx context.Context, ns, name string, pool *OriginPoolReplace) (*OriginPool, error)
	DeleteOriginPool(ctx context.Context, ns, name string) error
	ListOriginPools(ctx context.Context, ns string) ([]*OriginPool, error)

	// HealthCheck — Create and Replace take values, not pointers
	CreateHealthCheck(ctx context.Context, ns string, hc CreateHealthCheck) (*HealthCheck, error)
	GetHealthCheck(ctx context.Context, ns, name string) (*HealthCheck, error)
	ReplaceHealthCheck(ctx context.Context, ns, name string, hc ReplaceHealthCheck) (*HealthCheck, error)
	DeleteHealthCheck(ctx context.Context, ns, name string) error
	ListHealthChecks(ctx context.Context, ns string) ([]*HealthCheck, error)

	// AppFirewall
	CreateAppFirewall(ctx context.Context, ns string, fw *AppFirewallCreate) (*AppFirewall, error)
	GetAppFirewall(ctx context.Context, ns, name string) (*AppFirewall, error)
	ReplaceAppFirewall(ctx context.Context, ns, name string, fw *AppFirewallReplace) (*AppFirewall, error)
	DeleteAppFirewall(ctx context.Context, ns, name string) error
	ListAppFirewalls(ctx context.Context, ns string) ([]*AppFirewall, error)

	// ServicePolicy
	CreateServicePolicy(ctx context.Context, ns string, sp *ServicePolicyCreate) (*ServicePolicy, error)
	GetServicePolicy(ctx context.Context, ns, name string) (*ServicePolicy, error)
	ReplaceServicePolicy(ctx context.Context, ns, name string, sp *ServicePolicyReplace) (*ServicePolicy, error)
	DeleteServicePolicy(ctx context.Context, ns, name string) error
	ListServicePolicies(ctx context.Context, ns string) ([]*ServicePolicy, error)

	// XCRateLimiter — Create and Replace take values, not pointers
	CreateRateLimiter(ctx context.Context, ns string, rl XCRateLimiterCreate) (*XCRateLimiter, error)
	GetRateLimiter(ctx context.Context, ns, name string) (*XCRateLimiter, error)
	ReplaceRateLimiter(ctx context.Context, ns, name string, rl XCRateLimiterReplace) (*XCRateLimiter, error)
	DeleteRateLimiter(ctx context.Context, ns, name string) error
	ListRateLimiters(ctx context.Context, ns string) ([]*XCRateLimiter, error)

	// Certificate
	CreateCertificate(ctx context.Context, ns string, cert *CertificateCreate) (*Certificate, error)
	GetCertificate(ctx context.Context, ns, name string) (*Certificate, error)
	ReplaceCertificate(ctx context.Context, ns, name string, cert *CertificateReplace) (*Certificate, error)
	DeleteCertificate(ctx context.Context, ns, name string) error
	ListCertificates(ctx context.Context, ns string) ([]*Certificate, error)

	// Diff helper
	ClientNeedsUpdate(current, desired json.RawMessage) (bool, error)
}
