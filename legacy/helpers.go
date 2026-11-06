package main

import (
	"fmt"
	"strings"

	"windeploy/core/inventory"
	"windeploy/core/shares"
)

// formatSummary renders the read-only computer summary.
func formatSummary(hostname, osLabel, architecture, cpu string, logicalProcessors uint32, memoryBytes uint64, admin bool, activation string) string {
	if hostname == "" {
		hostname = "—"
	}
	if osLabel == "" {
		osLabel = "Windows (версия неизвестна)"
	}
	if cpu == "" {
		cpu = "—"
	}
	adminLabel := "нет"
	if admin {
		adminLabel = "есть"
	}
	activationLabel := "неизвестно"
	switch activation {
	case "genuine":
		activationLabel = "подтверждена"
	case "not-genuine":
		activationLabel = "не подтверждена"
	}
	return strings.Join([]string{
		"Компьютер: " + hostname,
		"Windows: " + osLabel,
		"Архитектура: " + architecture,
		"Процессор: " + cpu,
		fmt.Sprintf("Логических процессоров: %d", logicalProcessors),
		"Оперативная память: " + formatBytes(memoryBytes),
		"Права администратора: " + adminLabel,
		"Лицензия Windows: " + activationLabel,
	}, "\r\n")
}

func formatBytes(value uint64) string {
	if value == 0 {
		return "—"
	}
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d Б", value)
	}
	divisor, exponent := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %cиБ", float64(value)/float64(divisor), []rune("КМГТПЭ")[exponent])
}

// windowsFriendly returns a short human label for the NT version.
func windowsFriendly(version inventory.Version) string {
	switch {
	case version.Major == 10 && version.Build >= 22000:
		return "Windows 11"
	case version.Major == 10:
		return "Windows 10"
	case version.Major == 6 && version.Minor == 3:
		return "Windows 8.1"
	case version.Major == 6 && version.Minor == 2:
		return "Windows 8"
	case version.Major == 6 && version.Minor == 1:
		if version.ServicePackMajor >= 1 {
			return "Windows 7 SP1"
		}
		return "Windows 7"
	case version.Major == 6 && version.Minor == 0:
		return fmt.Sprintf("Windows Vista SP%d", version.ServicePackMajor)
	default:
		return fmt.Sprintf("Windows NT %d.%d (build %d)", version.Major, version.Minor, version.Build)
	}
}

// shareFromForm validates the legacy share form without touching the network.
func shareFromForm(unc, name, username string, shortcut bool) (shares.Share, error) {
	share := shares.Share{ID: "legacy-share", UNC: strings.TrimSpace(unc), Name: strings.TrimSpace(name), Username: strings.TrimSpace(username), Shortcut: shortcut}
	if err := share.Validate(); err != nil {
		return shares.Share{}, err
	}
	return share, nil
}
