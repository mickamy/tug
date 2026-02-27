package traefik

import "fmt"

// Labels returns the Traefik Docker labels for an HTTP service.
// If containerPort > 0, a load-balancer server port label is included.
func Labels(projectName, serviceName string, containerPort uint16) []string {
	host := fmt.Sprintf("%s.%s.localhost", serviceName, projectName)

	labels := []string{
		"traefik.enable=true",
		fmt.Sprintf("traefik.http.routers.%s-%s.rule=Host(`%s`)", projectName, serviceName, host),
	}

	if containerPort > 0 {
		labels = append(labels,
			fmt.Sprintf("traefik.http.services.%s-%s.loadbalancer.server.port=%d",
				projectName, serviceName, containerPort),
		)
	}

	return labels
}
