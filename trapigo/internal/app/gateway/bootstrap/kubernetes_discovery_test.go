package bootstrap

import (
	"context"
	"reflect"
	"testing"

	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDiscoverKubernetesServersUsesReadyPodEndpointsOnNamedPort(t *testing.T) {
	ready := true
	notReady := false
	terminating := true
	port := int32(8080)
	portName := "http"
	protocol := corev1.ProtocolTCP
	podRef := corev1.ObjectReference{Kind: "Pod", Name: "go-order-service-1"}

	client := fake.NewSimpleClientset(
		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "go-order-service-a",
				Namespace: "default",
				Labels: map[string]string{
					discoveryv1.LabelServiceName: "go-order-service",
				},
			},
			Ports: []discoveryv1.EndpointPort{{
				Name:     &portName,
				Port:     &port,
				Protocol: &protocol,
			}},
			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses:  []string{"10.0.0.2"},
					Conditions: discoveryv1.EndpointConditions{Ready: &ready},
					TargetRef:  &podRef,
				},
				{
					Addresses:  []string{"10.0.0.1"},
					Conditions: discoveryv1.EndpointConditions{Ready: &ready},
					TargetRef:  &podRef,
				},
				{
					Addresses:  []string{"10.0.0.3"},
					Conditions: discoveryv1.EndpointConditions{Ready: &notReady},
					TargetRef:  &podRef,
				},
				{
					Addresses:  []string{"10.0.0.5"},
					Conditions: discoveryv1.EndpointConditions{Ready: &ready, Terminating: &terminating},
					TargetRef:  &podRef,
				},
				{
					Addresses:  []string{"10.0.0.4"},
					Conditions: discoveryv1.EndpointConditions{Ready: &ready},
					TargetRef:  &corev1.ObjectReference{Kind: "Service", Name: "not-a-pod"},
				},
			},
		},
	)

	servers, err := discoverKubernetesServers(context.Background(), client, "default", config.KubernetesServiceConfig{
		Service: "go-order-service",
		Port:    "http",
	})
	if err != nil {
		t.Fatalf("discoverKubernetesServers returned an error: %v", err)
	}

	got := make([]string, len(servers))
	for i, server := range servers {
		got[i] = server.URL
	}
	want := []string{"http://10.0.0.1:8080", "http://10.0.0.2:8080"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected discovered servers: got %v, want %v", got, want)
	}
}

func TestDiscoverKubernetesServersReturnsNoServersForNonMatchingPort(t *testing.T) {
	port := int32(8080)
	portName := "metrics"
	protocol := corev1.ProtocolTCP

	client := fake.NewSimpleClientset(&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "go-order-service-a",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "go-order-service",
			},
		},
		Ports: []discoveryv1.EndpointPort{{
			Name:     &portName,
			Port:     &port,
			Protocol: &protocol,
		}},
	})

	servers, err := discoverKubernetesServers(context.Background(), client, "default", config.KubernetesServiceConfig{
		Service: "go-order-service",
		Port:    "http",
	})
	if err != nil {
		t.Fatalf("discoverKubernetesServers returned an error: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected no servers for non-matching port, got %v", servers)
	}
}

func TestBuildLoadBalancerUsesDiscoveredPodEndpoints(t *testing.T) {
	ready := true
	port := int32(8080)
	portName := "http"
	protocol := corev1.ProtocolTCP
	podRef := corev1.ObjectReference{Kind: "Pod", Name: "go-order-service-1"}
	client := fake.NewSimpleClientset(&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "go-order-service-a",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "go-order-service",
			},
		},
		Ports: []discoveryv1.EndpointPort{{
			Name:     &portName,
			Port:     &port,
			Protocol: &protocol,
		}},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{Ready: &ready},
				TargetRef:  &podRef,
			},
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: &ready},
				TargetRef:  &podRef,
			},
		},
	})

	builder := &kubernetesBackendBuilder{namespace: "default", client: client}
	loadBalancer, proxies, err := buildLoadBalancerWithDiscovery(config.HTTPConfig{
		Routers: map[string]config.Router{
			"go-orders-router": {
				PathPrefix: "/api/v1/go-orders",
				Service:    "go-order-service",
			},
		},
		Services: map[string]config.Service{
			"go-order-service": {
				Kubernetes: &config.KubernetesServiceConfig{
					Service: "go-order-service",
					Port:    "http",
				},
			},
		},
	}, builder)
	if err != nil {
		t.Fatalf("buildLoadBalancerWithDiscovery returned an error: %v", err)
	}

	if len(loadBalancer.Routers) != 1 {
		t.Fatalf("expected one gateway route, got %d", len(loadBalancer.Routers))
	}
	pool := loadBalancer.Routers[0].BackendPool
	if len(pool.Backends) != 2 {
		t.Fatalf("expected two discovered backends, got %d", len(pool.Backends))
	}
	if got := pool.RoundRobinAtomic().Id; got != "http://10.0.0.1:8080" {
		t.Fatalf("unexpected first round-robin backend: %s", got)
	}
	if got := pool.RoundRobinAtomic().Id; got != "http://10.0.0.2:8080" {
		t.Fatalf("unexpected second round-robin backend: %s", got)
	}
	if len(proxies) != 2 {
		t.Fatalf("expected a proxy for each discovered backend, got %d", len(proxies))
	}
}

func TestBuildLoadBalancerPreservesStaticServers(t *testing.T) {
	t.Setenv("POD_NAMESPACE", "")
	cfg := config.HTTPConfig{
		Routers: map[string]config.Router{
			"orders": {PathPrefix: "/orders", Service: "orders"},
		},
		Services: map[string]config.Service{
			"orders": {LoadBalancer: config.LoadBalancer{Servers: []config.Server{
				{URL: "http://order-1:8080"},
				{URL: "http://order-2:8080"},
				{URL: "http://order-3:8080"},
			}}},
		},
	}
	lb, proxies, err := buildLoadBalancer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool := lb.Routers[0].BackendPool
	for _, want := range cfg.Services["orders"].LoadBalancer.Servers {
		if got := pool.RoundRobinAtomic().Id; got != want.URL {
			t.Fatalf("got backend %q, want %q", got, want.URL)
		}
		if proxies[want.URL] == nil {
			t.Fatalf("missing static proxy for %q", want.URL)
		}
	}
}

func TestBuildLoadBalancerSupportsMixedServices(t *testing.T) {
	ready := true
	port := int32(8080)
	portName := "http"
	protocol := corev1.ProtocolTCP
	podRef := corev1.ObjectReference{Kind: "Pod", Name: "orders-1"}
	client := fake.NewSimpleClientset(&discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orders-a",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "orders",
			},
		},
		Ports: []discoveryv1.EndpointPort{{
			Name:     &portName,
			Port:     &port,
			Protocol: &protocol,
		}},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
			TargetRef:  &podRef,
		}},
	})
	lb, proxies, err := buildLoadBalancerWithDiscovery(config.HTTPConfig{
		Routers: map[string]config.Router{
			"static": {PathPrefix: "/static", Service: "static"},
			"kube":   {PathPrefix: "/kube", Service: "kube"},
		},
		Services: map[string]config.Service{
			"static": {LoadBalancer: config.LoadBalancer{Servers: []config.Server{{URL: "http://static:8080"}}}},
			"kube":   {Kubernetes: &config.KubernetesServiceConfig{Service: "orders", Port: "http"}},
		},
	}, &kubernetesBackendBuilder{namespace: "default", client: client})
	if err != nil {
		t.Fatal(err)
	}
	if len(lb.Routers) != 2 || len(proxies) != 2 {
		t.Fatalf("unexpected mixed configuration: routes=%d, proxies=%d", len(lb.Routers), len(proxies))
	}
}
