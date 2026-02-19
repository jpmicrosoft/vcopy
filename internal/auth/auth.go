package auth

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Authenticate resolves tokens for source and target GitHub hosts.
// It supports three modes: "auto" (try gh CLI first, fallback to PAT),
// "gh" (gh CLI only), and "pat" (PAT only).
func Authenticate(method, sourceHost, targetHost, sourceToken, targetToken string) (string, string, error) {
	switch method {
	case "auto":
		return autoAuth(sourceHost, targetHost, sourceToken, targetToken)
	case "gh":
		return ghAuth(sourceHost, targetHost)
	case "pat":
		return patAuth(sourceHost, targetHost, sourceToken, targetToken)
	default:
		return "", "", fmt.Errorf("unknown auth method: %s", method)
	}
}

func autoAuth(sourceHost, targetHost, srcToken, tgtToken string) (string, string, error) {
	var err error

	if srcToken == "" {
		srcToken, err = tryGHToken(sourceHost)
		if err != nil {
			srcToken, err = promptForToken(sourceHost, "source")
			if err != nil {
				return "", "", err
			}
		}
	}

	if tgtToken == "" {
		tgtToken, err = tryGHToken(targetHost)
		if err != nil {
			tgtToken, err = promptForToken(targetHost, "target")
			if err != nil {
				return "", "", err
			}
		}
	}

	return srcToken, tgtToken, nil
}

func ghAuth(sourceHost, targetHost string) (string, string, error) {
	srcToken, err := tryGHToken(sourceHost)
	if err != nil {
		return "", "", fmt.Errorf("gh auth failed for source host %s: %w", sourceHost, err)
	}

	tgtToken, err := tryGHToken(targetHost)
	if err != nil {
		return "", "", fmt.Errorf("gh auth failed for target host %s: %w", targetHost, err)
	}

	return srcToken, tgtToken, nil
}

func patAuth(sourceHost, targetHost, srcToken, tgtToken string) (string, string, error) {
	var err error

	if srcToken == "" {
		srcToken, err = promptForToken(sourceHost, "source")
		if err != nil {
			return "", "", err
		}
	}

	if tgtToken == "" {
		tgtToken, err = promptForToken(targetHost, "target")
		if err != nil {
			return "", "", err
		}
	}

	return srcToken, tgtToken, nil
}

// tryGHToken attempts to get a token from the gh CLI for the given host.
func tryGHToken(host string) (string, error) {
	cmd := exec.Command("gh", "auth", "token", "--hostname", host)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh CLI not available or not authenticated for %s", host)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("gh CLI returned empty token for %s", host)
	}
	return token, nil
}

// promptForToken prompts the user to enter a PAT interactively.
func promptForToken(host, label string) (string, error) {
	fmt.Printf("Enter PAT for %s (%s): ", label, host)
	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("token cannot be empty for %s (%s)", label, host)
	}
	return token, nil
}
