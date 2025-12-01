package google

import "os"

// readFileFromOS reads a file from the filesystem
func readFileFromOS(filepath string) ([]byte, error) {
	return os.ReadFile(filepath)
}
