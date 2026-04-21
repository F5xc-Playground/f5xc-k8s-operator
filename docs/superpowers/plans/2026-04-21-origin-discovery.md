# Origin Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the OriginPool CRD with a `discover` field that dynamically resolves origin server addresses from Kubernetes Services, Ingress, Gateway, and OpenShift Routes.

**Architecture:** The OriginPool controller gains a resolution step before XC API mapping. Pure resolver functions extract address+port from K8s resource objects. Dynamic watches via field indexers trigger re-reconciliation when referenced resources change. Gateway API and OpenShift Route watches are conditional on CRD presence.

**Tech Stack:** Go 1.26, controller-runtime v0.23.3, envtest, sigs.k8s.io/gateway-api, github.com/openshift/api

---

### Task 1: Add types, constants, and dependencies

**Files:**
- Modify: `api/v1alpha1/shared_types.go`
- Modify: `api/v1alpha1/originpool_types.go`
- Modify: `api/v1alpha1/constants.go`
- Modify: `go.mod`

- [ ] **Step 1: Add Gateway API and OpenShift API dependencies**

```bash
go get sigs.k8s.io/gateway-api@latest
go get github.com/openshift/api@latest
go mod tidy
```

- [ ] **Step 2: Add ResourceRef to shared_types.go**

Add after the `RoutePool` type:

```go
type ResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}
```

- [ ] **Step 3: Add OriginServerDiscover and DiscoveredOrigin to originpool_types.go**

Add `Discover` field to `OriginServer`:

```go
type OriginServer struct {
	PublicIP      *PublicIP              `json:"publicIP,omitempty"`
	PublicName    *PublicName            `json:"publicName,omitempty"`
	PrivateIP     *PrivateIP             `json:"privateIP,omitempty"`
	PrivateName   *PrivateName           `json:"privateName,omitempty"`
	K8SService    *K8SService            `json:"k8sService,omitempty"`
	ConsulService *ConsulService         `json:"consulService,omitempty"`
	Discover      *OriginServerDiscover  `json:"discover,omitempty"`
}
```

Add new types after `ConsulService`:

```go
type OriginServerDiscover struct {
	Resource        ResourceRef `json:"resource"`
	AddressOverride string      `json:"addressOverride,omitempty"`
	PortOverride    *uint32     `json:"portOverride,omitempty"`
}
```

Add `DiscoveredOrigins` field to `OriginPoolStatus`:

```go
type OriginPoolStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	XCResourceVersion  string             `json:"xcResourceVersion,omitempty"`
	XCUID              string             `json:"xcUID,omitempty"`
	XCNamespace        string             `json:"xcNamespace,omitempty"`
	DiscoveredOrigins  []DiscoveredOrigin `json:"discoveredOrigins,omitempty"`
}
```

Add `DiscoveredOrigin` type:

```go
type DiscoveredOrigin struct {
	Resource    ResourceRef `json:"resource"`
	Address     string      `json:"address,omitempty"`
	Port        uint32      `json:"port,omitempty"`
	AddressType string      `json:"addressType,omitempty"`
	Status      string      `json:"status"`
	Message     string      `json:"message,omitempty"`
}
```

- [ ] **Step 4: Add constants to constants.go**

Add to the existing `const` block:

```go
	ReasonDiscoveryPending = "DiscoveryPending"
	ReasonDiscoveryFailed  = "DiscoveryFailed"

	AddressTypeIP   = "IP"
	AddressTypeFQDN = "FQDN"

	DiscoveryStatusResolved = "Resolved"
	DiscoveryStatusPending  = "Pending"
```

- [ ] **Step 5: Regenerate deepcopy and verify build**

```bash
export PATH="$HOME/go/bin:$PATH"
controller-gen object paths="./api/..."
go build ./...
go vet ./...
```

- [ ] **Step 6: Run existing tests to verify no regressions**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1
```

Expected: all existing tests pass (no behavioral changes yet).

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum api/v1alpha1/ 
git commit -m "Add origin discovery types, constants, and Gateway/OpenShift dependencies"
```

---

### Task 2: Service resolver — pure functions and tests

**Files:**
- Create: `internal/controller/origin_resolver.go`
- Create: `internal/controller/origin_resolver_test.go`

- [ ] **Step 1: Write the failing tests for Service resolver**

Create `internal/controller/origin_resolver_test.go`:

```go
package controller

import (
	"testing"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveService_LoadBalancerWithIP(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "203.0.113.50"}},
			},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.50", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveService_LoadBalancerWithHostname(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 80}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{Hostname: "a1b2c3.elb.amazonaws.com"}},
			},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "a1b2c3.elb.amazonaws.com", result.Address)
	assert.Equal(t, uint32(80), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveService_LoadBalancerPending(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443}},
		},
		Status: corev1.ServiceStatus{},
	}

	result := ResolveService(svc, nil)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no loadBalancer ingress")
}

func TestResolveService_NodePort(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{{NodePort: 30080}},
		},
	}
	nodes := []corev1.Node{
		{
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeExternalIP, Address: "198.51.100.10"},
				},
			},
		},
	}

	result := ResolveService(svc, nodes)
	assert.False(t, result.Pending)
	assert.Equal(t, "198.51.100.10", result.Address)
	assert.Equal(t, uint32(30080), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveService_NodePortNoExternalIP(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{{NodePort: 30080}},
		},
	}
	nodes := []corev1.Node{
		{
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
				},
			},
		},
	}

	result := ResolveService(svc, nodes)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no nodes with external IP")
}

func TestResolveService_ExternalName(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "backend.example.com",
			Ports:        []corev1.ServicePort{{Port: 8443}},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "backend.example.com", result.Address)
	assert.Equal(t, uint32(8443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveService_ExternalNameNoPort(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "backend.example.com",
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "backend.example.com", result.Address)
	assert.Equal(t, uint32(0), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveService_ExternalIPs(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:        corev1.ServiceTypeClusterIP,
			ExternalIPs: []string{"203.0.113.100"},
			Ports:       []corev1.ServicePort{{Port: 443}},
		},
	}

	result := ResolveService(svc, nil)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.100", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveService_ClusterIPOnly(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{Port: 80}},
		},
	}

	result := ResolveService(svc, nil)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "ClusterIP")
}

func TestResolveService_ExternalIPsTakesPriorityOverLoadBalancer(t *testing.T) {
	svc := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type:        corev1.ServiceTypeLoadBalancer,
			ExternalIPs: []string{"198.51.100.50"},
			Ports:       []corev1.ServicePort{{Port: 443}},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "203.0.113.1"}},
			},
		},
	}

	result := ResolveService(svc, nil)
	assert.Equal(t, "198.51.100.50", result.Address)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/controller/ -run TestResolveService -count=1
```

Expected: FAIL — `ResolveService` undefined.

- [ ] **Step 3: Implement the Service resolver**

Create `internal/controller/origin_resolver.go`:

```go
package controller

import (
	"fmt"
	"net"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type ResolvedOrigin struct {
	Address     string
	Port        uint32
	AddressType string
	Pending     bool
	Message     string
}

func classifyAddress(addr string) string {
	if net.ParseIP(addr) != nil {
		return v1alpha1.AddressTypeIP
	}
	return v1alpha1.AddressTypeFQDN
}

func ResolveService(svc *corev1.Service, nodes []corev1.Node) ResolvedOrigin {
	// Priority: ExternalName > externalIPs > LoadBalancer > NodePort > ClusterIP (fail)
	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return resolveExternalName(svc)
	}
	if len(svc.Spec.ExternalIPs) > 0 {
		return resolveExternalIPs(svc)
	}
	if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
		return resolveLoadBalancer(svc)
	}
	if svc.Spec.Type == corev1.ServiceTypeNodePort {
		return resolveNodePort(svc, nodes)
	}
	return ResolvedOrigin{
		Pending: true,
		Message: "Service type ClusterIP is not externally routable",
	}
}

func resolveLoadBalancer(svc *corev1.Service) ResolvedOrigin {
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return ResolvedOrigin{
			Pending: true,
			Message: "Service has no loadBalancer ingress assigned",
		}
	}
	ingress := svc.Status.LoadBalancer.Ingress[0]
	addr := ingress.IP
	if addr == "" {
		addr = ingress.Hostname
	}
	return ResolvedOrigin{
		Address:     addr,
		Port:        servicePort(svc),
		AddressType: classifyAddress(addr),
	}
}

func resolveNodePort(svc *corev1.Service, nodes []corev1.Node) ResolvedOrigin {
	for _, node := range nodes {
		if !isNodeReady(&node) {
			continue
		}
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP {
				var port uint32
				if len(svc.Spec.Ports) > 0 {
					port = uint32(svc.Spec.Ports[0].NodePort)
				}
				return ResolvedOrigin{
					Address:     addr.Address,
					Port:        port,
					AddressType: v1alpha1.AddressTypeIP,
				}
			}
		}
	}
	return ResolvedOrigin{
		Pending: true,
		Message: "Service type NodePort but no nodes with external IP found",
	}
}

func resolveExternalName(svc *corev1.Service) ResolvedOrigin {
	return ResolvedOrigin{
		Address:     svc.Spec.ExternalName,
		Port:        servicePort(svc),
		AddressType: v1alpha1.AddressTypeFQDN,
	}
}

func resolveExternalIPs(svc *corev1.Service) ResolvedOrigin {
	return ResolvedOrigin{
		Address:     svc.Spec.ExternalIPs[0],
		Port:        servicePort(svc),
		AddressType: classifyAddress(svc.Spec.ExternalIPs[0]),
	}
}

func servicePort(svc *corev1.Service) uint32 {
	if len(svc.Spec.Ports) > 0 {
		return uint32(svc.Spec.Ports[0].Port)
	}
	return 0
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func ResolveDiscover(discover *v1alpha1.OriginServerDiscover, resolved ResolvedOrigin) ResolvedOrigin {
	if discover.AddressOverride != "" {
		resolved.Address = discover.AddressOverride
		resolved.AddressType = classifyAddress(discover.AddressOverride)
	}
	if discover.PortOverride != nil {
		resolved.Port = *discover.PortOverride
	}
	return resolved
}

func UnsupportedKindError(kind string) ResolvedOrigin {
	return ResolvedOrigin{
		Pending: true,
		Message: fmt.Sprintf("unsupported resource kind %q", kind),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/controller/ -run TestResolveService -count=1 -v
```

Expected: all 11 tests pass.

- [ ] **Step 5: Write override tests**

Add to `origin_resolver_test.go`:

```go
func TestResolveDiscover_AddressOverride(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "10.0.0.1",
		Port:        80,
		AddressType: v1alpha1.AddressTypeIP,
	}
	discover := &v1alpha1.OriginServerDiscover{
		AddressOverride: "203.0.113.50",
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, "203.0.113.50", result.Address)
	assert.Equal(t, uint32(80), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveDiscover_PortOverride(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "1.2.3.4",
		Port:        80,
		AddressType: v1alpha1.AddressTypeIP,
	}
	port := uint32(8443)
	discover := &v1alpha1.OriginServerDiscover{
		PortOverride: &port,
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, "1.2.3.4", result.Address)
	assert.Equal(t, uint32(8443), result.Port)
}

func TestResolveDiscover_BothOverrides(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "10.0.0.1",
		Port:        80,
		AddressType: v1alpha1.AddressTypeIP,
	}
	port := uint32(443)
	discover := &v1alpha1.OriginServerDiscover{
		AddressOverride: "lb.example.com",
		PortOverride:    &port,
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, "lb.example.com", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveDiscover_AddressOverrideIP(t *testing.T) {
	resolved := ResolvedOrigin{
		Address:     "host.example.com",
		Port:        443,
		AddressType: v1alpha1.AddressTypeFQDN,
	}
	discover := &v1alpha1.OriginServerDiscover{
		AddressOverride: "198.51.100.1",
	}

	result := ResolveDiscover(discover, resolved)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}
```

- [ ] **Step 6: Run all resolver tests**

```bash
go test ./internal/controller/ -run "TestResolve" -count=1 -v
```

Expected: all 15 tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/controller/origin_resolver.go internal/controller/origin_resolver_test.go
git commit -m "Add Service resolver with override support and unit tests"
```

---

### Task 3: Ingress resolver and tests

**Files:**
- Modify: `internal/controller/origin_resolver.go`
- Modify: `internal/controller/origin_resolver_test.go`

- [ ] **Step 1: Write the failing tests for Ingress resolver**

Add to `origin_resolver_test.go`:

```go
import (
	networkingv1 "k8s.io/api/networking/v1"
)

func TestResolveIngress_WithIP(t *testing.T) {
	ing := &networkingv1.Ingress{
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "203.0.113.10"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.10", result.Address)
	assert.Equal(t, uint32(80), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveIngress_WithHostname(t *testing.T) {
	ing := &networkingv1.Ingress{
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{Hostname: "ingress.example.com"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.False(t, result.Pending)
	assert.Equal(t, "ingress.example.com", result.Address)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveIngress_WithTLS(t *testing.T) {
	ing := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{Hosts: []string{"example.com"}}},
		},
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.Equal(t, uint32(443), result.Port)
}

func TestResolveIngress_NoTLS(t *testing.T) {
	ing := &networkingv1.Ingress{
		Status: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}},
			},
		},
	}

	result := ResolveIngress(ing)
	assert.Equal(t, uint32(80), result.Port)
}

func TestResolveIngress_Pending(t *testing.T) {
	ing := &networkingv1.Ingress{}

	result := ResolveIngress(ing)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no loadBalancer ingress")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/controller/ -run TestResolveIngress -count=1
```

Expected: FAIL — `ResolveIngress` undefined.

- [ ] **Step 3: Implement the Ingress resolver**

Add to `origin_resolver.go` (add `networkingv1 "k8s.io/api/networking/v1"` to imports):

```go
func ResolveIngress(ing *networkingv1.Ingress) ResolvedOrigin {
	if len(ing.Status.LoadBalancer.Ingress) == 0 {
		return ResolvedOrigin{
			Pending: true,
			Message: "Ingress has no loadBalancer ingress assigned",
		}
	}

	ingress := ing.Status.LoadBalancer.Ingress[0]
	addr := ingress.IP
	if addr == "" {
		addr = ingress.Hostname
	}

	port := uint32(80)
	if len(ing.Spec.TLS) > 0 {
		port = 443
	}

	return ResolvedOrigin{
		Address:     addr,
		Port:        port,
		AddressType: classifyAddress(addr),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/controller/ -run TestResolveIngress -count=1 -v
```

Expected: all 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/origin_resolver.go internal/controller/origin_resolver_test.go
git commit -m "Add Ingress resolver with unit tests"
```

---

### Task 4: Gateway resolver and tests

**Files:**
- Modify: `internal/controller/origin_resolver.go`
- Modify: `internal/controller/origin_resolver_test.go`

- [ ] **Step 1: Write the failing tests for Gateway resolver**

Add to `origin_resolver_test.go`:

```go
import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestResolveGateway_WithIPAddress(t *testing.T) {
	addrType := gatewayv1.IPAddressType
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Port: 8443}},
		},
		Status: gatewayv1.GatewayStatus{
			Addresses: []gatewayv1.GatewayStatusAddress{
				{Type: &addrType, Value: "203.0.113.20"},
			},
		},
	}

	result := ResolveGateway(gw)
	assert.False(t, result.Pending)
	assert.Equal(t, "203.0.113.20", result.Address)
	assert.Equal(t, uint32(8443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}

func TestResolveGateway_WithHostname(t *testing.T) {
	addrType := gatewayv1.HostnameAddressType
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Port: 443}},
		},
		Status: gatewayv1.GatewayStatus{
			Addresses: []gatewayv1.GatewayStatusAddress{
				{Type: &addrType, Value: "gateway.example.com"},
			},
		},
	}

	result := ResolveGateway(gw)
	assert.False(t, result.Pending)
	assert.Equal(t, "gateway.example.com", result.Address)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveGateway_Pending(t *testing.T) {
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Port: 443}},
		},
	}

	result := ResolveGateway(gw)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no addresses")
}

func TestResolveGateway_NilAddressType(t *testing.T) {
	gw := &gatewayv1.Gateway{
		Spec: gatewayv1.GatewaySpec{
			Listeners: []gatewayv1.Listener{{Port: 443}},
		},
		Status: gatewayv1.GatewayStatus{
			Addresses: []gatewayv1.GatewayStatusAddress{
				{Value: "1.2.3.4"},
			},
		},
	}

	result := ResolveGateway(gw)
	assert.False(t, result.Pending)
	assert.Equal(t, "1.2.3.4", result.Address)
	assert.Equal(t, v1alpha1.AddressTypeIP, result.AddressType)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/controller/ -run TestResolveGateway -count=1
```

Expected: FAIL — `ResolveGateway` undefined.

- [ ] **Step 3: Implement the Gateway resolver**

Add to `origin_resolver.go` (add `gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"` to imports):

```go
func ResolveGateway(gw *gatewayv1.Gateway) ResolvedOrigin {
	if len(gw.Status.Addresses) == 0 {
		return ResolvedOrigin{
			Pending: true,
			Message: "Gateway has no addresses assigned",
		}
	}

	addr := gw.Status.Addresses[0]
	addrType := v1alpha1.AddressTypeIP
	if addr.Type != nil && *addr.Type == gatewayv1.HostnameAddressType {
		addrType = v1alpha1.AddressTypeFQDN
	} else {
		addrType = classifyAddress(addr.Value)
	}

	var port uint32
	if len(gw.Spec.Listeners) > 0 {
		port = uint32(gw.Spec.Listeners[0].Port)
	}

	return ResolvedOrigin{
		Address:     addr.Value,
		Port:        port,
		AddressType: addrType,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/controller/ -run TestResolveGateway -count=1 -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/origin_resolver.go internal/controller/origin_resolver_test.go
git commit -m "Add Gateway resolver with unit tests"
```

---

### Task 5: OpenShift Route resolver and tests

**Files:**
- Modify: `internal/controller/origin_resolver.go`
- Modify: `internal/controller/origin_resolver_test.go`

- [ ] **Step 1: Write the failing tests for Route resolver**

Add to `origin_resolver_test.go`:

```go
import (
	routev1 "github.com/openshift/api/route/v1"
)

func TestResolveRoute_AdmittedWithTLS(t *testing.T) {
	route := &routev1.Route{
		Spec: routev1.RouteSpec{
			TLS: &routev1.TLSConfig{Termination: routev1.TLSTerminationEdge},
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "myapp.apps.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue},
					},
				},
			},
		},
	}

	result := ResolveRoute(route)
	assert.False(t, result.Pending)
	assert.Equal(t, "myapp.apps.example.com", result.Address)
	assert.Equal(t, uint32(443), result.Port)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, result.AddressType)
}

func TestResolveRoute_AdmittedNoTLS(t *testing.T) {
	route := &routev1.Route{
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "myapp.apps.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionTrue},
					},
				},
			},
		},
	}

	result := ResolveRoute(route)
	assert.False(t, result.Pending)
	assert.Equal(t, uint32(80), result.Port)
}

func TestResolveRoute_NotAdmitted(t *testing.T) {
	route := &routev1.Route{
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "myapp.apps.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{Type: routev1.RouteAdmitted, Status: corev1.ConditionFalse},
					},
				},
			},
		},
	}

	result := ResolveRoute(route)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "not admitted")
}

func TestResolveRoute_NoIngress(t *testing.T) {
	route := &routev1.Route{}

	result := ResolveRoute(route)
	assert.True(t, result.Pending)
	assert.Contains(t, result.Message, "no ingress status")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/controller/ -run TestResolveRoute -count=1
```

Expected: FAIL — `ResolveRoute` undefined.

- [ ] **Step 3: Implement the Route resolver**

Add to `origin_resolver.go` (add `routev1 "github.com/openshift/api/route/v1"` to imports):

```go
func ResolveRoute(route *routev1.Route) ResolvedOrigin {
	if len(route.Status.Ingress) == 0 {
		return ResolvedOrigin{
			Pending: true,
			Message: "Route has no ingress status",
		}
	}

	for _, ri := range route.Status.Ingress {
		for _, cond := range ri.Conditions {
			if cond.Type == routev1.RouteAdmitted && cond.Status == corev1.ConditionTrue {
				port := uint32(80)
				if route.Spec.TLS != nil {
					port = 443
				}
				return ResolvedOrigin{
					Address:     ri.Host,
					Port:        port,
					AddressType: v1alpha1.AddressTypeFQDN,
				}
			}
		}
	}

	return ResolvedOrigin{
		Pending: true,
		Message: "Route is not admitted",
	}
}
```

- [ ] **Step 4: Run all resolver tests**

```bash
go test ./internal/controller/ -run "TestResolve" -count=1 -v
```

Expected: all resolver tests pass (Service, Ingress, Gateway, Route, overrides).

- [ ] **Step 5: Commit**

```bash
git add internal/controller/origin_resolver.go internal/controller/origin_resolver_test.go
git commit -m "Add OpenShift Route resolver with unit tests"
```

---

### Task 6: Extend mapper to handle resolved discover origins

**Files:**
- Modify: `internal/controller/originpool_mapper.go`
- Modify: `internal/controller/originpool_mapper_test.go`

- [ ] **Step 1: Write the failing test for mapper with resolved origins**

Add to `originpool_mapper_test.go`:

```go
func TestBuildOriginPoolCreate_WithResolvedDiscoverOrigins(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "discover-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "my-svc"},
				}},
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Ingress", Name: "my-ing"},
				}},
			},
			Port: 443,
		},
	}

	resolved := []*ResolvedOrigin{
		nil, // static origin — no resolution
		{Address: "203.0.113.50", Port: 443, AddressType: v1alpha1.AddressTypeIP},
		{Address: "ingress.example.com", Port: 443, AddressType: v1alpha1.AddressTypeFQDN},
	}

	result := buildOriginPoolCreate(cr, "ns", resolved)
	require.Len(t, result.Spec.OriginServers, 3)

	// First: static PublicIP unchanged
	assert.Equal(t, "1.2.3.4", result.Spec.OriginServers[0].PublicIP.IP)

	// Second: resolved IP → PublicIP
	assert.NotNil(t, result.Spec.OriginServers[1].PublicIP)
	assert.Equal(t, "203.0.113.50", result.Spec.OriginServers[1].PublicIP.IP)

	// Third: resolved FQDN → PublicName
	assert.NotNil(t, result.Spec.OriginServers[2].PublicName)
	assert.Equal(t, "ingress.example.com", result.Spec.OriginServers[2].PublicName.DNSName)
}

func TestBuildOriginPoolCreate_NilResolvedBackwardsCompatible(t *testing.T) {
	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "static-pool", Namespace: "ns"},
		Spec: v1alpha1.OriginPoolSpec{
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
			},
			Port: 443,
		},
	}

	result := buildOriginPoolCreate(cr, "ns", nil)
	require.Len(t, result.Spec.OriginServers, 1)
	assert.Equal(t, "1.2.3.4", result.Spec.OriginServers[0].PublicIP.IP)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/controller/ -run "TestBuildOriginPoolCreate_WithResolved|TestBuildOriginPoolCreate_NilResolved" -count=1
```

Expected: FAIL — `buildOriginPoolCreate` has wrong number of arguments.

- [ ] **Step 3: Update mapper functions to accept resolved origins**

Modify `originpool_mapper.go`. Update all three builder functions to accept `resolved []*ResolvedOrigin`:

```go
func buildOriginPoolCreate(cr *v1alpha1.OriginPool, xcNamespace string, resolved []*ResolvedOrigin) *xcclient.OriginPoolCreate {
	return &xcclient.OriginPoolCreate{
		Metadata: xcclient.ObjectMeta{
			Name:      cr.Name,
			Namespace: xcNamespace,
		},
		Spec: mapOriginPoolSpec(&cr.Spec, resolved),
	}
}

func buildOriginPoolReplace(cr *v1alpha1.OriginPool, xcNamespace, resourceVersion string, resolved []*ResolvedOrigin) *xcclient.OriginPoolReplace {
	return &xcclient.OriginPoolReplace{
		Metadata: xcclient.ObjectMeta{
			Name:            cr.Name,
			Namespace:       xcNamespace,
			ResourceVersion: resourceVersion,
		},
		Spec: mapOriginPoolSpec(&cr.Spec, resolved),
	}
}

func buildOriginPoolDesiredSpecJSON(cr *v1alpha1.OriginPool, xcNamespace string, resolved []*ResolvedOrigin) (json.RawMessage, error) {
	create := buildOriginPoolCreate(cr, xcNamespace, resolved)
	return json.Marshal(create.Spec)
}

func mapOriginPoolSpec(spec *v1alpha1.OriginPoolSpec, resolved []*ResolvedOrigin) xcclient.OriginPoolSpec {
	out := xcclient.OriginPoolSpec{
		Port:                  spec.Port,
		LoadBalancerAlgorithm: spec.LoadBalancerAlgorithm,
	}

	for i, s := range spec.OriginServers {
		if resolved != nil && i < len(resolved) && resolved[i] != nil {
			out.OriginServers = append(out.OriginServers, mapResolvedOriginServer(resolved[i]))
		} else {
			out.OriginServers = append(out.OriginServers, mapOriginServer(&s))
		}
	}

	for _, hc := range spec.HealthChecks {
		out.HealthCheck = append(out.HealthCheck, mapObjectRef(&hc))
	}

	if spec.UseTLS != nil {
		out.UseTLS = json.RawMessage(spec.UseTLS.Raw)
	}
	if spec.NoTLS != nil {
		out.NoTLS = json.RawMessage(spec.NoTLS.Raw)
	}

	return out
}

func mapResolvedOriginServer(r *ResolvedOrigin) xcclient.OriginServer {
	var out xcclient.OriginServer
	if r.AddressType == v1alpha1.AddressTypeIP {
		out.PublicIP = &xcclient.PublicIP{IP: r.Address}
	} else {
		out.PublicName = &xcclient.PublicName{DNSName: r.Address}
	}
	return out
}
```

- [ ] **Step 4: Update existing mapper tests and controller to pass nil for resolved**

Update all existing calls in `originpool_controller.go` to pass `nil`:

In `handleCreate`:
```go
create := buildOriginPoolCreate(cr, xcNS, nil)
```

In `handleUpdate`:
```go
desiredJSON, err := buildOriginPoolDesiredSpecJSON(cr, xcNS, nil)
```
and:
```go
replace := buildOriginPoolReplace(cr, xcNS, current.Metadata.ResourceVersion, nil)
```

Update existing tests in `originpool_mapper_test.go` — add `nil` as the third argument to all `buildOriginPoolCreate`, `buildOriginPoolReplace`, and `buildOriginPoolDesiredSpecJSON` calls:

- `TestBuildOriginPoolCreate_BasicFields`: `buildOriginPoolCreate(cr, "default", nil)`
- `TestBuildOriginPoolCreate_AllOriginServerTypes`: `buildOriginPoolCreate(cr, "ns", nil)`
- `TestBuildOriginPoolCreate_HealthChecks`: `buildOriginPoolCreate(cr, "ns", nil)`
- `TestBuildOriginPoolCreate_TLSPassthrough`: `buildOriginPoolCreate(cr, "ns", nil)`
- `TestBuildOriginPoolReplace_IncludesResourceVersion`: `buildOriginPoolReplace(cr, "ns", "rv-123", nil)`
- `TestBuildOriginPoolCreate_XCNamespaceOverride`: `buildOriginPoolCreate(cr, "xc-override-ns", nil)`
- `TestBuildDesiredSpecJSON`: `buildOriginPoolDesiredSpecJSON(cr, "ns", nil)`

- [ ] **Step 5: Run all tests to verify no regressions**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1
```

Expected: all tests pass including new mapper tests and all existing tests.

- [ ] **Step 6: Commit**

```bash
git add internal/controller/originpool_mapper.go internal/controller/originpool_mapper_test.go internal/controller/originpool_controller.go
git commit -m "Extend mapper to handle resolved discover origins"
```

---

### Task 7: Controller resolution step and discovery status

**Files:**
- Modify: `internal/controller/originpool_controller.go`

- [ ] **Step 1: Add RBAC markers and imports**

Add new RBAC markers after the existing ones in `originpool_controller.go`:

```go
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch
```

Add new imports:

```go
import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	routev1 "github.com/openshift/api/route/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)
```

- [ ] **Step 2: Add hasDiscoverOrigins helper and resolveAllOrigins method**

Add to `originpool_controller.go`:

```go
func hasDiscoverOrigins(cr *v1alpha1.OriginPool) bool {
	for _, os := range cr.Spec.OriginServers {
		if os.Discover != nil {
			return true
		}
	}
	return false
}

func (r *OriginPoolReconciler) resolveAllOrigins(ctx context.Context, cr *v1alpha1.OriginPool) ([]*ResolvedOrigin, []v1alpha1.DiscoveredOrigin, bool) {
	resolved := make([]*ResolvedOrigin, len(cr.Spec.OriginServers))
	var discovered []v1alpha1.DiscoveredOrigin
	allResolved := true

	for i, os := range cr.Spec.OriginServers {
		if os.Discover == nil {
			continue
		}

		ref := os.Discover.Resource
		ns := ref.Namespace
		if ns == "" {
			ns = cr.Namespace
		}

		raw := r.resolveResource(ctx, ref.Kind, ns, ref.Name)
		final := ResolveDiscover(os.Discover, raw)

		do := v1alpha1.DiscoveredOrigin{
			Resource: v1alpha1.ResourceRef{Kind: ref.Kind, Name: ref.Name, Namespace: ns},
			Status:   v1alpha1.DiscoveryStatusResolved,
		}

		if final.Pending {
			do.Status = v1alpha1.DiscoveryStatusPending
			do.Message = final.Message
			allResolved = false
		} else {
			do.Address = final.Address
			do.Port = final.Port
			do.AddressType = final.AddressType
			resolved[i] = &final
		}

		discovered = append(discovered, do)
	}

	return resolved, discovered, allResolved
}

func (r *OriginPoolReconciler) resolveResource(ctx context.Context, kind, ns, name string) ResolvedOrigin {
	key := client.ObjectKey{Namespace: ns, Name: name}

	switch kind {
	case "Service":
		var svc corev1.Service
		if err := r.Get(ctx, key, &svc); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Service %s/%s not found: %v", ns, name, err)}
		}
		var nodes corev1.NodeList
		if svc.Spec.Type == corev1.ServiceTypeNodePort {
			if err := r.List(ctx, &nodes); err != nil {
				return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("failed to list nodes: %v", err)}
			}
		}
		return ResolveService(&svc, nodes.Items)

	case "Ingress":
		var ing networkingv1.Ingress
		if err := r.Get(ctx, key, &ing); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Ingress %s/%s not found: %v", ns, name, err)}
		}
		return ResolveIngress(&ing)

	case "Gateway":
		var gw gatewayv1.Gateway
		if err := r.Get(ctx, key, &gw); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Gateway %s/%s not found: %v", ns, name, err)}
		}
		return ResolveGateway(&gw)

	case "Route":
		var route routev1.Route
		if err := r.Get(ctx, key, &route); err != nil {
			return ResolvedOrigin{Pending: true, Message: fmt.Sprintf("Route %s/%s not found: %v", ns, name, err)}
		}
		return ResolveRoute(&route)

	default:
		return UnsupportedKindError(kind)
	}
}
```

- [ ] **Step 3: Insert resolution step into Reconcile**

Modify the `Reconcile` method. After the finalizer block and before `xcNS := resolveXCNamespace(cr)`, add:

```go
	// Resolve discover origins
	var resolved []*ResolvedOrigin
	if hasDiscoverOrigins(&cr) {
		var discovered []v1alpha1.DiscoveredOrigin
		var allResolved bool
		resolved, discovered, allResolved = r.resolveAllOrigins(ctx, &cr)
		cr.Status.DiscoveredOrigins = discovered

		if !allResolved {
			r.setCondition(&cr, v1alpha1.ConditionReady, metav1.ConditionFalse, v1alpha1.ReasonDiscoveryPending, "one or more discover origins are pending")
			cr.Status.ObservedGeneration = cr.Generation
			if err := r.Status().Update(ctx, &cr); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}
```

- [ ] **Step 4: Pass resolved to mapper calls**

Update `handleCreate` to accept and pass resolved:

```go
func (r *OriginPoolReconciler) handleCreate(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, xc xcclient.XCClient, xcNS string, resolved []*ResolvedOrigin) (ctrl.Result, error) {
	create := buildOriginPoolCreate(cr, xcNS, resolved)
```

Update `handleUpdate` to accept and pass resolved:

```go
func (r *OriginPoolReconciler) handleUpdate(ctx context.Context, log logr.Logger, cr *v1alpha1.OriginPool, xc xcclient.XCClient, xcNS string, current *xcclient.OriginPool, resolved []*ResolvedOrigin) (ctrl.Result, error) {
	desiredJSON, err := buildOriginPoolDesiredSpecJSON(cr, xcNS, resolved)
	...
	replace := buildOriginPoolReplace(cr, xcNS, current.Metadata.ResourceVersion, resolved)
```

Update the calls in `Reconcile`:

```go
	if errors.Is(err, xcclient.ErrNotFound) {
		return r.handleCreate(ctx, log, &cr, xc, xcNS, resolved)
	}

	return r.handleUpdate(ctx, log, &cr, xc, xcNS, current, resolved)
```

- [ ] **Step 5: Update status with discoveredOrigins on success**

In `setStatus`, after updating XC metadata, add discovered origins preservation. The discovered origins were already set on `cr.Status.DiscoveredOrigins` during resolution — they will be persisted by the subsequent `r.Status().Update(ctx, cr)` call in handleCreate/handleUpdate. No additional changes needed here since the status update happens after setStatus.

- [ ] **Step 6: Verify build**

```bash
go build ./...
go vet ./...
```

- [ ] **Step 7: Run existing tests (they use nil resolved, so should pass)**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1
```

Expected: all existing tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/controller/originpool_controller.go
git commit -m "Add origin resolution step to OriginPool reconciler"
```

---

### Task 8: Controller unit tests for discovery

**Files:**
- Create: `internal/controller/originpool_controller_discover_test.go`

- [ ] **Step 1: Write discovery controller tests**

Create `internal/controller/originpool_controller_discover_test.go`:

```go
package controller

import (
	"testing"
	"time"

	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestDiscover_ServiceLoadBalancerIP(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-svc-ip"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "my-svc", Namespace: "disc-svc-ip"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))

	// Update Service status with an IP
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "203.0.113.50"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-ip", Namespace: "disc-svc-ip"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "my-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-ip", Namespace: "disc-svc-ip"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()
	assert.True(t, created, "expected CreateOriginPool to be called")

	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, v1alpha1.DiscoveryStatusResolved, updated.Status.DiscoveredOrigins[0].Status)
	assert.Equal(t, "203.0.113.50", updated.Status.DiscoveredOrigins[0].Address)
	assert.Equal(t, v1alpha1.AddressTypeIP, updated.Status.DiscoveredOrigins[0].AddressType)
}

func TestDiscover_IngressHostname(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-ing"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "my-ing", Namespace: "disc-ing"},
	}
	require.NoError(t, testClient.Create(testCtx, ing))

	ing.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{Hostname: "ingress.example.com"}}
	require.NoError(t, testClient.Status().Update(testCtx, ing))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-ing", Namespace: "disc-ing"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Ingress", Name: "my-ing"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-ing", Namespace: "disc-ing"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, v1alpha1.DiscoveryStatusResolved, updated.Status.DiscoveredOrigins[0].Status)
	assert.Equal(t, "ingress.example.com", updated.Status.DiscoveredOrigins[0].Address)
	assert.Equal(t, v1alpha1.AddressTypeFQDN, updated.Status.DiscoveredOrigins[0].AddressType)
}

func TestDiscover_Pending(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-pending"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Service with no LB status
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-svc", Namespace: "disc-pending"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-pending", Namespace: "disc-pending"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "pending-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-pending", Namespace: "disc-pending"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	assert.Equal(t, v1alpha1.ReasonDiscoveryPending, cond.Reason)

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()
	assert.False(t, created, "expected CreateOriginPool NOT to be called when pending")
}

func TestDiscover_MixedStaticAndDiscover(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-mixed"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "mixed-svc", Namespace: "disc-mixed"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-mixed", Namespace: "disc-mixed"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{PublicIP: &v1alpha1.PublicIP{IP: "1.2.3.4"}},
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "mixed-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-mixed", Namespace: "disc-mixed"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	fake.mu.Lock()
	created := fake.createCalled
	fake.mu.Unlock()
	assert.True(t, created, "expected create with both static and discovered origins")
}

func TestDiscover_AddressOverride(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-override"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "override-svc", Namespace: "disc-override"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "10.0.0.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-override", Namespace: "disc-override"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource:        v1alpha1.ResourceRef{Kind: "Service", Name: "override-svc"},
					AddressOverride: "203.0.113.99",
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-override", Namespace: "disc-override"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, "203.0.113.99", updated.Status.DiscoveredOrigins[0].Address)
}

func TestDiscover_MissingResource(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-missing"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-missing", Namespace: "disc-missing"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "nonexistent"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-missing", Namespace: "disc-missing"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionFalse)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))

	cond := meta.FindStatusCondition(updated.Status.Conditions, v1alpha1.ConditionReady)
	assert.Equal(t, v1alpha1.ReasonDiscoveryPending, cond.Reason)
}

func TestDiscover_CrossNamespace(t *testing.T) {
	setupSuite(t)
	fake := &fakeXCClient{}
	r := newReconciler(fake)
	startManager(t, r)

	nsA := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-xns-a"}}
	nsB := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "disc-xns-b"}}
	require.NoError(t, testClient.Create(testCtx, nsA))
	require.NoError(t, testClient.Create(testCtx, nsB))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "xns-svc", Namespace: "disc-xns-b"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "disc-pool-xns", Namespace: "disc-xns-a"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "xns-svc", Namespace: "disc-xns-b"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "disc-pool-xns", Namespace: "disc-xns-a"}
	waitForCondition(t, testCtx, key, v1alpha1.ConditionReady, metav1.ConditionTrue)

	var updated v1alpha1.OriginPool
	require.NoError(t, testClient.Get(testCtx, key, &updated))
	require.Len(t, updated.Status.DiscoveredOrigins, 1)
	assert.Equal(t, "198.51.100.1", updated.Status.DiscoveredOrigins[0].Address)
}
```

- [ ] **Step 2: Run discovery controller tests**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run "TestDiscover_" -count=1 -v
```

Expected: all 7 tests pass.

- [ ] **Step 3: Run full test suite for regressions**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/controller/originpool_controller_discover_test.go
git commit -m "Add controller unit tests for origin discovery"
```

---

### Task 9: Dynamic watches and field indexer

**Files:**
- Modify: `internal/controller/originpool_controller.go`

- [ ] **Step 1: Add field indexer and watch setup to SetupWithManager**

Replace the existing `SetupWithManager` method:

```go
const discoverIndexKey = "spec.originServers.discover"

func discoverIndexFunc(obj client.Object) []string {
	cr, ok := obj.(*v1alpha1.OriginPool)
	if !ok {
		return nil
	}
	var refs []string
	for _, os := range cr.Spec.OriginServers {
		if os.Discover != nil {
			ref := os.Discover.Resource
			ns := ref.Namespace
			if ns == "" {
				ns = cr.Namespace
			}
			refs = append(refs, fmt.Sprintf("%s/%s/%s", ref.Kind, ns, ref.Name))
		}
	}
	return refs
}

func (r *OriginPoolReconciler) mapServiceToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Service", obj)
}

func (r *OriginPoolReconciler) mapIngressToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Ingress", obj)
}

func (r *OriginPoolReconciler) mapNodeToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	// Node changes affect all OriginPools referencing NodePort Services.
	// We re-reconcile all OriginPools that reference any Service (let the resolver re-check).
	var pools v1alpha1.OriginPoolList
	if err := r.List(ctx, &pools); err != nil {
		return nil
	}
	var requests []ctrl.Request
	seen := make(map[types.NamespacedName]bool)
	for _, pool := range pools.Items {
		for _, os := range pool.Spec.OriginServers {
			if os.Discover != nil && os.Discover.Resource.Kind == "Service" {
				key := types.NamespacedName{Name: pool.Name, Namespace: pool.Namespace}
				if !seen[key] {
					requests = append(requests, ctrl.Request{NamespacedName: key})
					seen[key] = true
				}
			}
		}
	}
	return requests
}

func (r *OriginPoolReconciler) mapGatewayToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Gateway", obj)
}

func (r *OriginPoolReconciler) mapRouteToOriginPools(ctx context.Context, obj client.Object) []ctrl.Request {
	return r.mapResourceToOriginPools(ctx, "Route", obj)
}

func (r *OriginPoolReconciler) mapResourceToOriginPools(ctx context.Context, kind string, obj client.Object) []ctrl.Request {
	indexKey := fmt.Sprintf("%s/%s/%s", kind, obj.GetNamespace(), obj.GetName())
	var pools v1alpha1.OriginPoolList
	if err := r.List(ctx, &pools, client.MatchingFields{discoverIndexKey: indexKey}); err != nil {
		return nil
	}
	requests := make([]ctrl.Request, len(pools.Items))
	for i, pool := range pools.Items {
		requests[i] = ctrl.Request{
			NamespacedName: types.NamespacedName{Name: pool.Name, Namespace: pool.Namespace},
		}
	}
	return requests
}

func (r *OriginPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Register field indexer for discover references
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1alpha1.OriginPool{}, discoverIndexKey, discoverIndexFunc); err != nil {
		return fmt.Errorf("indexing discover references: %w", err)
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.OriginPool{}).
		Watches(&corev1.Service{}, handler.EnqueueRequestsFromMapFunc(r.mapServiceToOriginPools)).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.mapNodeToOriginPools)).
		Watches(&networkingv1.Ingress{}, handler.EnqueueRequestsFromMapFunc(r.mapIngressToOriginPools))

	// Conditional watches for optional CRDs
	if r.crdInstalled(mgr, "gateways.gateway.networking.k8s.io") {
		builder = builder.Watches(&gatewayv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayToOriginPools))
	}
	if r.crdInstalled(mgr, "routes.route.openshift.io") {
		builder = builder.Watches(&routev1.Route{}, handler.EnqueueRequestsFromMapFunc(r.mapRouteToOriginPools))
	}

	return builder.Complete(r)
}

func (r *OriginPoolReconciler) crdInstalled(mgr ctrl.Manager, crdName string) bool {
	_, err := mgr.GetRESTMapper().RESTMapping(
		schema.GroupKind{
			Group: strings.Split(crdName, ".")[1] + "." + strings.Split(crdName, ".")[2],
			Kind:  strings.TrimSuffix(strings.Split(crdName, ".")[0], "s"),
		},
	)
	return err == nil
}
```

Wait — the `crdInstalled` function is fragile with string parsing. Let me use a cleaner approach:

```go
func crdInstalled(mgr ctrl.Manager, group, kind string) bool {
	_, err := mgr.GetRESTMapper().RESTMapping(schema.GroupKind{Group: group, Kind: kind})
	return err == nil
}
```

And the conditional watches:

```go
	if crdInstalled(mgr, "gateway.networking.k8s.io", "Gateway") {
		builder = builder.Watches(&gatewayv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.mapGatewayToOriginPools))
	}
	if crdInstalled(mgr, "route.openshift.io", "Route") {
		builder = builder.Watches(&routev1.Route{}, handler.EnqueueRequestsFromMapFunc(r.mapRouteToOriginPools))
	}
```

Add these imports to `originpool_controller.go`:

```go
import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	routev1 "github.com/openshift/api/route/v1"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)
```

- [ ] **Step 2: Verify build**

```bash
go build ./...
go vet ./...
```

- [ ] **Step 3: Run full test suite**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1
```

Expected: all tests pass. Note: Gateway and Route watches won't register in envtest since those CRDs aren't installed, but the conditional check handles this gracefully.

- [ ] **Step 4: Commit**

```bash
git add internal/controller/originpool_controller.go
git commit -m "Add dynamic watches and field indexer for origin discovery"
```

---

### Task 10: Integration tests for discovery

**Files:**
- Create: `internal/controller/originpool_integration_discover_test.go`

- [ ] **Step 1: Write integration tests**

Create `internal/controller/originpool_integration_discover_test.go`:

```go
package controller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kreynolds/f5xc-k8s-operator/api/v1alpha1"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclient/testutil"
	"github.com/kreynolds/f5xc-k8s-operator/internal/xcclientset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestIntegrationDiscover_CreateWithServiceLB(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-create"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-svc", Namespace: "intd-create"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.1"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool", Namespace: "intd-create"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	result := waitForConditionResult(t, types.NamespacedName{Name: "intd-pool", Namespace: "intd-create"}, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)
	require.NotNil(t, result)

	// Verify the XC server received a POST with publicIP
	requests := srv.Requests()
	var postFound bool
	for _, r := range requests {
		if r.Method == "POST" {
			postFound = true
			var body struct {
				Spec struct {
					OriginServers []struct {
						PublicIP *struct {
							IP string `json:"ip"`
						} `json:"public_ip,omitempty"`
					} `json:"origin_servers"`
				} `json:"spec"`
			}
			require.NoError(t, json.Unmarshal(r.Body, &body))
			require.Len(t, body.Spec.OriginServers, 1)
			require.NotNil(t, body.Spec.OriginServers[0].PublicIP)
			assert.Equal(t, "198.51.100.1", body.Spec.OriginServers[0].PublicIP.IP)
		}
	}
	assert.True(t, postFound)
}

func TestIntegrationDiscover_PendingThenResolved(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-pending"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	// Create Service with no LB status (pending)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pending-svc", Namespace: "intd-pending"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool-pending", Namespace: "intd-pending"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-pending-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	// Should become pending
	key := types.NamespacedName{Name: "intd-pool-pending", Namespace: "intd-pending"}
	result := waitForConditionResult(t, key, v1alpha1.ConditionReady, metav1.ConditionFalse, 15*time.Second)
	require.NotNil(t, result)
	cond := meta.FindStatusCondition(result.Status.Conditions, v1alpha1.ConditionReady)
	assert.Equal(t, v1alpha1.ReasonDiscoveryPending, cond.Reason)

	// Now assign IP to Service
	require.NoError(t, testClient.Get(testCtx, types.NamespacedName{Name: "intd-pending-svc", Namespace: "intd-pending"}, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.2"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	// Should eventually become Ready
	result = waitForConditionResult(t, key, v1alpha1.ConditionReady, metav1.ConditionTrue, 30*time.Second)
	require.NotNil(t, result)
}

func TestIntegrationDiscover_ServiceIPChanges(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-change"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-change-svc", Namespace: "intd-change"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.10"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool-change", Namespace: "intd-change"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-change-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "intd-pool-change", Namespace: "intd-change"}
	waitForConditionResult(t, key, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)

	// Clear requests and change IP
	srv.ClearRequests()
	require.NoError(t, testClient.Get(testCtx, types.NamespacedName{Name: "intd-change-svc", Namespace: "intd-change"}, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.20"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	// Wait for a PUT with the new IP
	deadline := time.Now().Add(30 * time.Second)
	var putFound bool
	for time.Now().Before(deadline) {
		for _, r := range srv.Requests() {
			if r.Method == "PUT" {
				var body struct {
					Spec struct {
						OriginServers []struct {
							PublicIP *struct {
								IP string `json:"ip"`
							} `json:"public_ip,omitempty"`
						} `json:"origin_servers"`
					} `json:"spec"`
				}
				if err := json.Unmarshal(r.Body, &body); err == nil &&
					len(body.Spec.OriginServers) > 0 &&
					body.Spec.OriginServers[0].PublicIP != nil &&
					body.Spec.OriginServers[0].PublicIP.IP == "198.51.100.20" {
					putFound = true
				}
			}
		}
		if putFound {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	assert.True(t, putFound, "expected a PUT with the updated IP 198.51.100.20")
}

func TestIntegrationDiscover_DeleteWithDiscoverOrigin(t *testing.T) {
	setupSuite(t)

	srv := testutil.NewFakeXCServer()
	defer srv.Close()

	xcClient := newRealClient(t, srv.URL())
	cs := xcclientset.New(xcClient)
	reconciler := &OriginPoolReconciler{
		Log:       ctrl.Log.WithName("integration-discover"),
		ClientSet: cs,
	}
	startManager(t, reconciler)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "intd-delete"}}
	require.NoError(t, testClient.Create(testCtx, ns))

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-del-svc", Namespace: "intd-delete"},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{Port: 443, Protocol: corev1.ProtocolTCP}},
		},
	}
	require.NoError(t, testClient.Create(testCtx, svc))
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: "198.51.100.30"}}
	require.NoError(t, testClient.Status().Update(testCtx, svc))

	cr := &v1alpha1.OriginPool{
		ObjectMeta: metav1.ObjectMeta{Name: "intd-pool-del", Namespace: "intd-delete"},
		Spec: v1alpha1.OriginPoolSpec{
			Port: 443,
			OriginServers: []v1alpha1.OriginServer{
				{Discover: &v1alpha1.OriginServerDiscover{
					Resource: v1alpha1.ResourceRef{Kind: "Service", Name: "intd-del-svc"},
				}},
			},
		},
	}
	require.NoError(t, testClient.Create(testCtx, cr))

	key := types.NamespacedName{Name: "intd-pool-del", Namespace: "intd-delete"}
	waitForConditionResult(t, key, v1alpha1.ConditionSynced, metav1.ConditionTrue, 15*time.Second)

	require.NoError(t, testClient.Delete(testCtx, cr))

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var check v1alpha1.OriginPool
		err := testClient.Get(testCtx, key, &check)
		if err != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	var deleteFound bool
	for _, r := range srv.Requests() {
		if r.Method == "DELETE" {
			deleteFound = true
		}
	}
	assert.True(t, deleteFound, "expected a DELETE request to the fake server")
}
```

- [ ] **Step 2: Run integration tests**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./internal/controller/ -run "TestIntegrationDiscover_" -count=1 -v -timeout=300s
```

Expected: all 4 tests pass.

- [ ] **Step 3: Run full test suite**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/controller/originpool_integration_discover_test.go
git commit -m "Add integration tests for origin discovery"
```

---

### Task 11: Regenerate manifests, sample CRs, and final cleanup

**Files:**
- Modify: `config/crd/bases/xc.f5.com_originpools.yaml` (regenerated)
- Modify: `config/rbac/role.yaml` (regenerated)
- Modify: `config/samples/originpool.yaml` (if exists, update with discover example)
- Create: `config/samples/originpool-discover.yaml`

- [ ] **Step 1: Regenerate all manifests**

```bash
export PATH="$HOME/go/bin:$PATH"
controller-gen object paths="./api/..."
controller-gen crd rbac:roleName=manager-role paths="./..." output:crd:dir=config/crd/bases output:rbac:dir=config/rbac
```

- [ ] **Step 2: Verify CRD has discover field**

```bash
grep -A 5 "discover:" config/crd/bases/xc.f5.com_originpools.yaml | head -20
```

Expected: the discover field with resource, addressOverride, portOverride sub-fields.

- [ ] **Step 3: Verify RBAC has new permissions**

```bash
grep -A 2 "services\|ingresses\|gateways\|nodes\|routes" config/rbac/role.yaml
```

Expected: services, nodes, ingresses, gateways, routes with get/list/watch verbs.

- [ ] **Step 4: Create sample CR for discovery**

Create `config/samples/originpool-discover.yaml`:

```yaml
apiVersion: xc.f5.com/v1alpha1
kind: OriginPool
metadata:
  name: example-discover-pool
spec:
  port: 443
  originServers:
    - discover:
        resource:
          kind: Service
          name: my-nginx
          namespace: default
```

- [ ] **Step 5: Run gofmt and go vet**

```bash
gofmt -s -w .
go vet ./...
```

- [ ] **Step 6: Run full test suite one final time**

```bash
KUBEBUILDER_ASSETS="/Users/kevin/Library/Application Support/io.kubebuilder.envtest/k8s/1.35.0-darwin-arm64" go test ./... -count=1
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add config/ api/v1alpha1/zz_generated.deepcopy.go
git commit -m "Regenerate CRD/RBAC manifests and add discover sample CR"
```
