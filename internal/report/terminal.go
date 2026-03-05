package report

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/jpmicrosoft/vcopy/internal/verify"
)

// PrintTerminal outputs the verification report to the terminal with colors.
func PrintTerminal(report *verify.VerificationReport) {
	green := color.New(color.FgGreen, color.Bold)
	red := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan)
	bold := color.New(color.Bold)

	fmt.Println()
	bold.Println("╔══════════════════════════════════════════════════════╗")
	bold.Println("║          VCOPY VERIFICATION REPORT                  ║")
	bold.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	cyan.Printf("  Source: %s (%s)\n", report.SourceRepo, report.SourceHost)
	cyan.Printf("  Target: %s (%s)\n", report.TargetRepo, report.TargetHost)
	cyan.Printf("  Time:   %s\n", report.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Println()

	bold.Println("  ┌──────────────────────────────────┬────────┬─────────────────────────────────┐")
	bold.Println("  │ Check                            │ Status │ Details                         │")
	bold.Println("  ├──────────────────────────────────┼────────┼─────────────────────────────────┤")

	for _, check := range report.Checks {
		name := padRight(check.Name, 32)
		details := padRight(truncate(check.Details, 31), 31)

		fmt.Print("  │ ")
		fmt.Print(name)
		fmt.Print(" │ ")

		switch check.Status {
		case verify.StatusPass:
			green.Printf(" %-5s", "PASS")
		case verify.StatusFail:
			red.Printf(" %-5s", "FAIL")
		case verify.StatusWarn:
			yellow.Printf(" %-5s", "WARN")
		default:
			fmt.Printf(" %-5s", check.Status)
		}

		fmt.Print(" │ ")
		fmt.Print(details)
		fmt.Println(" │")
	}

	bold.Println("  └──────────────────────────────────┴────────┴─────────────────────────────────┘")
	fmt.Println()

	if report.AllPassed() {
		green.Println("  ✓ Overall: PASSED")
	} else {
		red.Println("  ✗ Overall: FAILED")
	}
	fmt.Println()
}

func padRight(s string, length int) string {
	runes := []rune(s)
	if len(runes) >= length {
		return string(runes[:length])
	}
	return s + strings.Repeat(" ", length-len(runes))
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}
