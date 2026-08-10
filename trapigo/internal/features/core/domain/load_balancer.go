package domain

type Router struct {
	PathPrefix  string
	ServiceName string
	BackendPool *BackendPool
}

type LoadBalancer struct {
	Routers []*Router
}

func NewRouter(pathPrefix, serviceName string, backendPool *BackendPool) *Router {
	return &Router{
		PathPrefix:  pathPrefix,
		ServiceName: serviceName,
		BackendPool: backendPool,
	}
}

func NewLoadBalancer(routers ...*Router) *LoadBalancer {
	return &LoadBalancer{
		Routers: routers,
	}
}

func (lb *LoadBalancer) AddRouter(router *Router) {
	lb.Routers = append(lb.Routers, router)
}
