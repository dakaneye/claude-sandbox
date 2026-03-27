package state

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// PickSession displays an interactive picker for multiple sessions.
// Returns the selected session or error if canceled.
func PickSession(sessions []*Session) (*Session, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions available")
	}

	fmt.Println("Multiple sessions found. Select one:")
	fmt.Println()

	for i, sess := range sessions {
		name := sess.ID
		if sess.Name != "" {
			name = fmt.Sprintf("%s (%s)", sess.Name, sess.ID)
		}
		age := time.Since(sess.CreatedAt).Round(time.Minute)
		fmt.Printf("  [%d] %s - %s (%s ago)\n", i+1, name, sess.Status, age)
	}

	fmt.Println()
	fmt.Print("Enter number (or 'q' to cancel): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "q" || input == "" {
		return nil, fmt.Errorf("canceled")
	}

	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(sessions) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	return sessions[num-1], nil
}
