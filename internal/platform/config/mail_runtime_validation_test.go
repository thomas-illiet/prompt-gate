package config

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestMailConfigurationIsValidatedBeforeStartingWorkers(t *testing.T) {
	enabled := strings.Replace(validConfigYAML, "  environment: development", "  environment: development\n  notifications:\n    enabled: true\n    from_address: no-reply@example.test", 1) + "\nworker:\n  smtp:\n    host: smtp.example.test\n    port: 587\n    tls_mode: starttls\n    timeout: 10s\n    poll_interval: 5s\n    batch_size: 25\n    max_attempts: 8\n    lease_duration: 1m\n    retry_base: 1m\n"
	for _, tc := range []struct {
		path  string
		value any
	}{
		{"common.notifications.from_address", "Name <user@example.test>"},
		{"common.notifications.from_name", "App\r\nBcc: attacker@example.test"},
		{"common.notifications.reply_to", "not-an-address"},
		{"worker.smtp.host", ""}, {"worker.smtp.port", 65536}, {"worker.smtp.tls_mode", "unsafe"},
		{"worker.smtp.username", "operator"}, {"worker.smtp.password", "password-without-username"},
		{"worker.smtp.batch_size", 0}, {"worker.smtp.max_attempts", 0}, {"worker.smtp.timeout", "0s"},
		{"worker.smtp.poll_interval", "-1s"}, {"worker.smtp.lease_duration", "1ms"}, {"worker.smtp.retry_base", "0s"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			var document map[string]any
			if err := yaml.Unmarshal([]byte(enabled), &document); err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(tc.path, ".")
			node := document
			for _, part := range parts[:len(parts)-1] {
				child, ok := node[part].(map[string]any)
				if !ok {
					child = map[string]any{}
					node[part] = child
				}
				node = child
			}
			node[parts[len(parts)-1]] = tc.value
			raw, err := yaml.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LoadWorker(writeConfig(t, string(raw))); err == nil || !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("invalid configuration %s: %v", tc.path, err)
			}
		})
	}
	for _, mode := range []string{"plain", "starttls", "tls"} {
		cfg, err := LoadWorker(writeConfig(t, strings.Replace(enabled, "tls_mode: starttls", "tls_mode: "+mode, 1)))
		if err != nil || cfg.SMTPTLSMode != mode {
			t.Fatalf("valid SMTP mode %s rejected: %v", mode, err)
		}
	}
	for _, setting := range []string{"notification_cleanup_interval", "notification_retention", "quota_notification_interval"} {
		if _, err := LoadSchedule(writeConfig(t, enabled+strings.Replace("\nschedule:\n  notification_cleanup_interval: 1h\n  notification_retention: 1h\n  quota_notification_interval: 1h\n", setting+": 1h", setting+": 0s", 1))); err == nil || !strings.Contains(err.Error(), setting) {
			t.Fatalf("invalid scheduling interval %s: %v", setting, err)
		}
	}
}
