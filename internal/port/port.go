package port

import (
	"fmt"
	"hash/fnv"
)

const (
	rangeMin = 10000
	rangeMax = 60000
)

// Compute returns a deterministic host port in [10000, 60000) derived from
// the project name, service name, and container port using FNV-1a hashing.
// If the initial slot collides with a member of used, it increments until a
// free slot is found.
func Compute(project, service string, containerPort uint16, used map[uint16]struct{}) uint16 {
	key := fmt.Sprintf("%s/%s/%d", project, service, containerPort)

	h := fnv.New32a()
	// fnv hash.Write never returns an error.
	_, _ = h.Write([]byte(key))

	size := uint32(rangeMax - rangeMin)
	port := uint16(h.Sum32()%size) + rangeMin

	for _, ok := used[port]; ok; _, ok = used[port] {
		port++
		if port >= rangeMax {
			port = rangeMin
		}
	}

	return port
}
