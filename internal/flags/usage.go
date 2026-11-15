package flags

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/viral32111/tapo-sensors/internal/config"
)

const (
	PROJECT_TITLE       = "Tapo Sensors"
	PROJECT_DESCRIPTION = "A Prometheus exporter for TP-Link Tapo H200 sensor hubs."
	PROJECT_URL         = "https://github.com/viral32111/tapo-sensors"
	PROJECT_COPYRIGHT   = "Copyright (C) 2026 viral32111, under the GNU AGPL v3.0 license."
)

const NAME_COLUMN_WIDTH = 27

type usageEntry struct {
	name        string
	description string
	fallback    string
}

func (entry usageEntry) describe() string {
	if entry.fallback == "" {
		return entry.description
	}

	return fmt.Sprintf("%s (default: '%s')", entry.description, entry.fallback)
}

func PrintUsage(writer io.Writer) {
	flagEntries := []usageEntry{
		{"-v, --version", "Print version & exit.", ""},
		{"-u, --usage", "Show this usage message & exit.", ""},
	}

	environmentEntries := []usageEntry{
		{"LOG_LEVEL", "Minimum level of logs to print (debug, info, warn, error)", strings.ToLower(config.DEFAULT_LOG_LEVEL.String())},
		{"HTTP_ADDRESS", "IP address to listen for HTTP requests on", config.DEFAULT_HTTP_ADDRESS},
		{"HTTP_PORT", "TCP port to listen for HTTP requests on", strconv.Itoa(config.DEFAULT_HTTP_PORT)},
		{"METRICS_PATH", "HTTP path to expose Prometheus metrics on", config.DEFAULT_METRICS_PATH},
		{"METRICS_GO_BUILTIN", "Whether to include Go runtime & process metrics", strconv.FormatBool(config.DEFAULT_METRICS_GO_BUILTIN)},
		{"METRICS_INTERVAL", "Interval between polls of the hub", config.DEFAULT_METRICS_INTERVAL.String()},
		{"ACCOUNT_EMAIL", "Email address of the TP-Link Tapo account (required)", ""},
		{"ACCOUNT_PASSWORD", "Password of the TP-Link Tapo account (required)", ""},
		{"HUB_NAME", "Friendly name for the sensor hub (required)", ""},
		{"HUB_IP_ADDRESS", "IP address of the sensor hub (required)", ""},
	}

	for index := range environmentEntries {
		environmentEntries[index].name = config.ENVIRONMENT_VARIABLE_PREFIX + environmentEntries[index].name
	}

	columnWidth := NAME_COLUMN_WIDTH
	for _, entry := range append(flagEntries, environmentEntries...) {
		columnWidth = max(columnWidth, len(entry.name)+1)
	}

	fmt.Fprintf(writer, "%s - %s\n", PROJECT_TITLE, PROJECT_DESCRIPTION)
	fmt.Fprintf(writer, "  %s\n", PROJECT_URL)
	fmt.Fprintf(writer, "\n")

	fmt.Fprintf(writer, "Usage:\n")
	fmt.Fprintf(writer, "  %s [flags]\n", os.Args[0])
	fmt.Fprintf(writer, "\n")

	fmt.Fprintf(writer, "Flags:\n")
	for _, entry := range flagEntries {
		fmt.Fprintf(writer, "  %-*s%s\n", columnWidth, entry.name, entry.describe())
	}
	fmt.Fprintf(writer, "\n")

	fmt.Fprintf(writer, "Environment variables:\n")
	for _, entry := range environmentEntries {
		fmt.Fprintf(writer, "  %-*s%s\n", columnWidth, entry.name, entry.describe())
	}
	fmt.Fprintf(writer, "\n")

	fmt.Fprintf(writer, "%s\n", PROJECT_COPYRIGHT)
}
