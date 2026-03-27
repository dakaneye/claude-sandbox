package cli

import (
	"bufio"
	"os"
	"strings"
)

// readLogTail reads the last n lines from a file.
// Returns empty string if file doesn't exist or is empty.
func readLogTail(path string, lines int) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Read all lines into a ring buffer of size n
	ring := make([]string, lines)
	index := 0
	count := 0

	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		ring[index] = scanner.Text()
		index = (index + 1) % lines
		count++
	}

	// Check for scanner errors (I/O issues, line too long)
	if scanner.Err() != nil || count == 0 {
		return ""
	}

	// Build result from ring buffer in correct order
	var result []string
	if count < lines {
		result = ring[:count]
	} else {
		// Ring buffer wrapped, start from index
		result = make([]string, lines)
		for i := 0; i < lines; i++ {
			result[i] = ring[(index+i)%lines]
		}
	}

	return strings.Join(result, "\n")
}
