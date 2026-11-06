// Package reporting renders deployment outcomes as local JSON and HTML reports.
// Reports never contain credentials; task parameters are deliberately excluded.
package reporting

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

// Report is a self-contained deployment report.
type Report struct {
	Version        int                  `json:"version"`
	GeneratedAt    time.Time            `json:"generated_at"`
	ProfileID      string               `json:"profile_id"`
	ProfileName    string               `json:"profile_name"`
	SessionID      string               `json:"session_id"`
	System         inventory.SystemInfo `json:"system"`
	StartedAt      time.Time            `json:"started_at"`
	FinishedAt     time.Time            `json:"finished_at"`
	RebootRequired bool                 `json:"reboot_required"`
	Cancelled      bool                 `json:"cancelled"`
	Results        []engine.TaskResult  `json:"results"`
	Tasks          []engine.PreviewTask `json:"tasks"`
	Summary        Summary              `json:"summary"`
}

// Summary aggregates status counts.
type Summary struct {
	Total            int `json:"total"`
	Success          int `json:"success"`
	AlreadyInstalled int `json:"already_installed"`
	Failed           int `json:"failed"`
	Skipped          int `json:"skipped"`
	NotApplicable    int `json:"not_applicable"`
	Cancelled        int `json:"cancelled"`
	RebootRequired   int `json:"reboot_required"`
}

// Build creates a report from the original tasks and their results. Task
// parameters are reduced to non-secret preview metadata.
func Build(sessionID, profileID, profileName string, system inventory.SystemInfo, tasks []engine.Task, results []engine.TaskResult, started, finished time.Time, reboot, cancelled bool) (Report, error) {
	preview, err := engine.PreviewPlan(tasks)
	if err != nil {
		return Report{}, fmt.Errorf("report plan is invalid")
	}
	report := Report{
		Version: 1, GeneratedAt: time.Now().UTC(), ProfileID: profileID, ProfileName: profileName,
		SessionID: sessionID, System: system, StartedAt: started, FinishedAt: finished,
		RebootRequired: reboot, Cancelled: cancelled,
		Results: append([]engine.TaskResult{}, results...), Tasks: preview.Tasks,
	}
	for _, result := range results {
		report.Summary.Total++
		switch result.Status {
		case engine.Success:
			report.Summary.Success++
		case engine.AlreadyInstalled:
			report.Summary.AlreadyInstalled++
		case engine.Failed:
			report.Summary.Failed++
		case engine.Skipped:
			report.Summary.Skipped++
		case engine.NotApplicable:
			report.Summary.NotApplicable++
		case engine.Cancelled:
			report.Summary.Cancelled++
		}
		if result.RebootRequired {
			report.Summary.RebootRequired++
		}
	}
	return report, nil
}

// WriteJSON writes the report as indented JSON.
func WriteJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// WriteHTML writes one escaped, self-contained HTML document.
func WriteHTML(writer io.Writer, report Report) error {
	var builder strings.Builder
	builder.WriteString(`<!doctype html><html lang="ru"><head><meta charset="utf-8"><title>WinDeploy — отчёт</title><style>
body{font-family:'Segoe UI',sans-serif;background:#0e131c;color:#e6edf7;margin:0;padding:32px}
h1{font-size:24px;margin:0 0 6px}h2{font-size:15px;margin:28px 0 10px;color:#b8cbe6}
.meta{color:#8fa2bd;font-size:12px;line-height:1.8}.cards{display:flex;gap:12px;flex-wrap:wrap;margin-top:18px}
.card{background:#16202e;border:1px solid #283549;border-radius:10px;padding:12px 16px;min-width:120px}
.card b{display:block;font-size:20px;margin-top:4px}table{width:100%;border-collapse:collapse;margin-top:12px;font-size:12px}
th,td{text-align:left;padding:9px 12px;border-bottom:1px solid #253246}th{color:#8fa2bd;font-size:10px;text-transform:uppercase;letter-spacing:1px}
.ok{color:#74dcb3}.fail{color:#f49aa3}.warn{color:#e8c57e}.muted{color:#8fa2bd}
footer{margin-top:28px;color:#7486a0;font-size:10px}
</style></head><body>`)
	builder.WriteString("<h1>WinDeploy — отчёт о развёртывании</h1>")
	builder.WriteString("<div class=\"meta\">")
	builder.WriteString("Профиль: " + html.EscapeString(report.ProfileName) + " (" + html.EscapeString(report.ProfileID) + ")<br>")
	builder.WriteString("Сессия: " + html.EscapeString(report.SessionID) + "<br>")
	builder.WriteString("Компьютер: " + html.EscapeString(report.System.Hostname) + "<br>")
	builder.WriteString("Начало: " + report.StartedAt.Local().Format("02.01.2006 15:04:05") + " · Окончание: " + report.FinishedAt.Local().Format("02.01.2006 15:04:05"))
	builder.WriteString("</div>")
	builder.WriteString("<div class=\"cards\">")
	writeCard := func(label string, value int, class string) {
		fmt.Fprintf(&builder, "<div class=\"card\">%s<b class=\"%s\">%d</b></div>", html.EscapeString(label), class, value)
	}
	writeCard("Всего", report.Summary.Total, "")
	writeCard("Успешно", report.Summary.Success+report.Summary.AlreadyInstalled, "ok")
	writeCard("Ошибки", report.Summary.Failed, "fail")
	writeCard("Пропущено", report.Summary.Skipped+report.Summary.NotApplicable, "muted")
	writeCard("Нужна перезагрузка", report.Summary.RebootRequired, "warn")
	builder.WriteString("</div>")
	if report.RebootRequired {
		builder.WriteString("<p class=\"warn\">Windows требует перезагрузку для завершения развёртывания.</p>")
	}
	if report.Cancelled {
		builder.WriteString("<p class=\"warn\">Выполнение было отменено; отчёт содержит частичные результаты.</p>")
	}
	builder.WriteString("<h2>Задачи</h2><table><thead><tr><th>Задача</th><th>Категория</th><th>Статус</th><th>Начало</th><th>Конец</th><th>Попытки</th><th>Диагностика</th></tr></thead><tbody>")
	tasks := map[string]engine.PreviewTask{}
	for _, task := range report.Tasks {
		tasks[task.ID] = task
	}
	for _, result := range report.Results {
		task := tasks[result.TaskID]
		statusClass := "ok"
		switch result.Status {
		case engine.Failed:
			statusClass = "fail"
		case engine.Skipped, engine.NotApplicable, engine.Cancelled:
			statusClass = "muted"
		}
		fmt.Fprintf(&builder, "<tr><td>%s</td><td>%s</td><td class=\"%s\">%s%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>",
			html.EscapeString(firstNonEmpty(task.Name, result.TaskID)), html.EscapeString(task.Category),
			statusClass, html.EscapeString(string(result.Status)), rebootSuffix(result.RebootRequired),
			formatTime(result.StartedAt), formatTime(result.FinishedAt), result.Attempts, html.EscapeString(result.Error))
	}
	builder.WriteString("</tbody></table>")
	builder.WriteString("<footer>Сгенерировано WinDeploy " + report.GeneratedAt.Local().Format("02.01.2006 15:04:05") + ". Отчёт не содержит паролей и параметров компонентов.</footer>")
	builder.WriteString("</body></html>")
	_, err := io.WriteString(writer, builder.String())
	return err
}

// Save writes JSON and HTML next to each other, refusing to overwrite.
func Save(directory string, report Report) (string, string, error) {
	if strings.TrimSpace(directory) == "" {
		return "", "", fmt.Errorf("report directory is required")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", "", fmt.Errorf("report directory is unavailable")
	}
	stem := fmt.Sprintf("windeploy-%s-%s", sanitizeID(report.SessionID), report.GeneratedAt.Format("20060102-150405"))
	jsonPath := filepath.Join(directory, stem+".json")
	htmlPath := filepath.Join(directory, stem+".html")
	if err := writeExclusive(jsonPath, func(writer io.Writer) error { return WriteJSON(writer, report) }); err != nil {
		return "", "", err
	}
	if err := writeExclusive(htmlPath, func(writer io.Writer) error { return WriteHTML(writer, report) }); err != nil {
		os.Remove(jsonPath)
		return "", "", err
	}
	return jsonPath, htmlPath, nil
}

func writeExclusive(path string, render func(io.Writer) error) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("report file already exists or cannot be created")
	}
	if err := render(file); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("report could not be written")
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("report could not be flushed")
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("report could not be closed")
	}
	return nil
}

func rebootSuffix(reboot bool) string {
	if reboot {
		return " · reboot"
	}
	return ""
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.Local().Format("15:04:05")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sanitizeID(value string) string {
	result := []rune{}
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result = append(result, r)
		}
	}
	if len(result) == 0 {
		return "session"
	}
	if len(result) > 48 {
		result = result[:48]
	}
	return string(result)
}
