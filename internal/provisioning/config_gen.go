package provisioning

import "fmt"

// GenerateNodeConfig generates a minimal YAML configuration for a node agent.
func GenerateNodeConfig(controllerAddr, enrollmentToken, dataDir string) string {
	return fmt.Sprintf(`role: node
log_level: info
node:
  controller_addr: %q
  enrollment_token: %q
  data_dir: %q
`, controllerAddr, enrollmentToken, dataDir)
}
