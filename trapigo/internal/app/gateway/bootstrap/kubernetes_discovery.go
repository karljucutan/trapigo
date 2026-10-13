package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"sort"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	endpointDiscoveryTimeout  = time.Minute
	endpointDiscoveryInterval = time.Second
)

type kubernetesBackendBuilder struct {
	namespace string
	client    kubernetes.Interface
	cache     map[string][]config.Server
}

func (b *kubernetesBackendBuilder) servers(serviceName string, target config.KubernetesServiceConfig) ([]config.Server, error) {
	if target.Service == "" || target.Port == "" {
		return nil, fmt.Errorf("Kubernetes service name and port name are required")
	}
	if b.namespace == "" {
		return nil, fmt.Errorf("POD_NAMESPACE must be set to use Kubernetes service discovery")
	}
	if servers, ok := b.cache[serviceName]; ok {
		return servers, nil
	}
	if b.client == nil {
		k8sConfig, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("configure in-cluster Kubernetes client: %w", err)
		}
		b.client, err = kubernetes.NewForConfig(k8sConfig)
		if err != nil {
			return nil, fmt.Errorf("create Kubernetes client: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), endpointDiscoveryTimeout)
	defer cancel()
	servers, err := waitForKubernetesServers(ctx, b.client, b.namespace, target)
	if err != nil {
		return nil, err
	}
	if b.cache == nil {
		b.cache = make(map[string][]config.Server)
	}
	b.cache[serviceName] = servers
	slog.Info("discovered Kubernetes service endpoints",
		"service", target.Service, "namespace", b.namespace, "endpoints", len(servers))
	return servers, nil
}

func waitForKubernetesServers(ctx context.Context, client kubernetes.Interface, namespace string, target config.KubernetesServiceConfig) ([]config.Server, error) {
	for {
		servers, err := discoverKubernetesServers(ctx, client, namespace, target)
		if err != nil {
			return nil, err
		}
		if len(servers) > 0 {
			return servers, nil
		}

		timer := time.NewTimer(endpointDiscoveryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("no ready Pod endpoints found for Service %q in namespace %q: %w", target.Service, namespace, ctx.Err())
		case <-timer.C:
		}
	}
}

func discoverKubernetesServers(ctx context.Context, client kubernetes.Interface, namespace string, target config.KubernetesServiceConfig) ([]config.Server, error) {
	if target.Service == "" || target.Port == "" {
		return nil, fmt.Errorf("Kubernetes service name and port name are required")
	}

	selector := labels.Set{discoveryv1.LabelServiceName: target.Service}.AsSelector().String()
	slices, err := client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list EndpointSlices for Service %q in namespace %q: %w", target.Service, namespace, err)
	}

	serverURLs := make(map[string]struct{})
	for _, endpointSlice := range slices.Items {
		ports := make([]int32, 0, len(endpointSlice.Ports))
		for _, port := range endpointSlice.Ports {
			if port.Name == nil || *port.Name != target.Port || port.Port == nil {
				continue
			}
			if port.Protocol != nil && *port.Protocol != "TCP" {
				continue
			}
			if *port.Port < 1 || *port.Port > 65535 {
				return nil, fmt.Errorf("EndpointSlice %q has invalid TCP port %d for port name %q", endpointSlice.Name, *port.Port, target.Port)
			}
			ports = append(ports, *port.Port)
		}
		if len(ports) == 0 {
			continue
		}

		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.TargetRef == nil || endpoint.TargetRef.Kind != "Pod" {
				continue
			}
			if endpoint.Conditions.Ready != nil && !*endpoint.Conditions.Ready {
				continue
			}
			if endpoint.Conditions.Terminating != nil && *endpoint.Conditions.Terminating {
				continue
			}
			for _, address := range endpoint.Addresses {
				ip := net.ParseIP(address)
				if ip == nil {
					return nil, fmt.Errorf("EndpointSlice %q contains invalid Pod IP %q", endpointSlice.Name, address)
				}
				for _, port := range ports {
					parsedURL := url.URL{
						Scheme: "http",
						Host:   net.JoinHostPort(ip.String(), fmt.Sprint(port)),
					}
					serverURLs[parsedURL.String()] = struct{}{}
				}
			}
		}
	}

	urls := make([]string, 0, len(serverURLs))
	for endpointURL := range serverURLs {
		urls = append(urls, endpointURL)
	}
	sort.Strings(urls)

	servers := make([]config.Server, 0, len(urls))
	for _, endpointURL := range urls {
		servers = append(servers, config.Server{URL: endpointURL})
	}
	return servers, nil
}
