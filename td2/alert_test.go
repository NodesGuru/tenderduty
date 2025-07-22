package tenderduty

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestSeverityThresholdToSeverities(t *testing.T) {
	tests := []struct {
		name      string
		threshold string
		expected  []string
	}{
		{
			name:      "critical threshold",
			threshold: "critical",
			expected:  []string{"critical"},
		},
		{
			name:      "warning threshold",
			threshold: "warning",
			expected:  []string{"critical", "warning"},
		},
		{
			name:      "info threshold",
			threshold: "info",
			expected:  []string{"critical", "warning", "info"},
		},
		{
			name:      "default threshold",
			threshold: "invalid",
			expected:  []string{"critical", "warning", "info"},
		},
		{
			name:      "case insensitive",
			threshold: "CRITICAL",
			expected:  []string{"critical"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SeverityThresholdToSeverities(tt.threshold)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("SeverityThresholdToSeverities(%s) = %v, want %v", tt.threshold, result, tt.expected)
			}
		})
	}
}

func TestAlarmCacheGetCount(t *testing.T) {
	cache := &alarmCache{
		AllAlarms: map[string]map[string]alertMsgCache{
			"chain1": {
				"alert1": {Message: "test1", SentTime: time.Now()},
				"alert2": {Message: "test2", SentTime: time.Now()},
			},
			"chain2": {
				"alert3": {Message: "test3", SentTime: time.Now()},
			},
		},
		notifyMux: sync.RWMutex{},
	}

	tests := []struct {
		name     string
		chain    string
		expected int
	}{
		{
			name:     "existing chain with alerts",
			chain:    "chain1",
			expected: 2,
		},
		{
			name:     "existing chain with one alert",
			chain:    "chain2",
			expected: 1,
		},
		{
			name:     "non-existing chain",
			chain:    "chain3",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cache.getCount(tt.chain)
			if result != tt.expected {
				t.Errorf("alarmCache.getCount(%s) = %v, want %v", tt.chain, result, tt.expected)
			}
		})
	}
}

func TestAlarmCacheClearAll(t *testing.T) {
	cache := &alarmCache{
		AllAlarms: map[string]map[string]alertMsgCache{
			"chain1": {
				"alert1": {Message: "test1", SentTime: time.Now()},
				"alert2": {Message: "test2", SentTime: time.Now()},
			},
		},
		notifyMux: sync.RWMutex{},
	}

	cache.clearAll("chain1")

	if len(cache.AllAlarms["chain1"]) != 0 {
		t.Errorf("Expected chain1 alerts to be cleared, but found %d alerts", len(cache.AllAlarms["chain1"]))
	}

	// Test clearing non-existing chain
	cache.clearAll("nonexistent")
	// Should not panic or cause errors
}

func TestShouldNotify(t *testing.T) {
	// Setup test alarm cache
	testAlarms := &alarmCache{
		SentPdAlarms:   make(map[string]alertMsgCache),
		SentTgAlarms:   make(map[string]alertMsgCache),
		SentDiAlarms:   make(map[string]alertMsgCache),
		SentSlkAlarms:  make(map[string]alertMsgCache),
		AllAlarms:      make(map[string]map[string]alertMsgCache),
		flappingAlarms: make(map[string]map[string]alertMsgCache),
		notifyMux:      sync.RWMutex{},
	}
	// Replace global alarms for testing
	originalAlarms := alarms
	alarms = testAlarms
	defer func() { alarms = originalAlarms }()

	tests := []struct {
		name        string
		msg         *alertMsg
		dest        notifyDest
		setupAlarms func()
		expected    bool
		description string
	}{
		{
			name: "first alert should notify",
			msg: &alertMsg{
				uniqueId: "test_alert_1",
				severity: "critical",
				resolved: false,
				alertConfig: &AlertConfig{
					Pagerduty: PDConfig{SeverityThreshold: "critical"},
				},
			},
			dest:        pd,
			setupAlarms: func() {},
			expected:    true,
			description: "First time sending alert should return true",
		},
		{
			name: "duplicate alert should not notify",
			msg: &alertMsg{
				uniqueId: "test_alert_2",
				severity: "critical",
				resolved: false,
				alertConfig: &AlertConfig{
					Pagerduty: PDConfig{SeverityThreshold: "critical"},
				},
			},
			dest: pd,
			setupAlarms: func() {
				testAlarms.SentPdAlarms["test_alert_2"] = alertMsgCache{
					Message:  "Previous alert",
					SentTime: time.Now().Add(-1 * time.Hour),
				}
			},
			expected:    false,
			description: "Duplicate alert should not notify",
		},
		{
			name: "resolved alert should notify",
			msg: &alertMsg{
				uniqueId: "test_alert_3",
				severity: "critical",
				resolved: true,
				alertConfig: &AlertConfig{
					Pagerduty: PDConfig{SeverityThreshold: "critical"},
				},
			},
			dest: pd,
			setupAlarms: func() {
				testAlarms.SentPdAlarms["test_alert_3"] = alertMsgCache{
					Message:  "Previous alert",
					SentTime: time.Now().Add(-1 * time.Hour),
				}
			},
			expected:    true,
			description: "Resolved alert should notify to clear",
		},
		{
			name: "severity below threshold should not notify",
			msg: &alertMsg{
				uniqueId: "test_alert_4",
				severity: "info",
				resolved: false,
				alertConfig: &AlertConfig{
					Pagerduty: PDConfig{SeverityThreshold: "critical"},
				},
			},
			dest:        pd,
			setupAlarms: func() {},
			expected:    false,
			description: "Alert with severity below threshold should not notify",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset alarms for each test
			testAlarms.SentPdAlarms = make(map[string]alertMsgCache)
			testAlarms.SentTgAlarms = make(map[string]alertMsgCache)
			testAlarms.SentDiAlarms = make(map[string]alertMsgCache)
			testAlarms.SentSlkAlarms = make(map[string]alertMsgCache)
			testAlarms.flappingAlarms = make(map[string]map[string]alertMsgCache)

			tt.setupAlarms()

			result := shouldNotify(tt.msg, tt.dest)
			if result != tt.expected {
				t.Errorf("%s: shouldNotify() = %v, want %v", tt.description, result, tt.expected)
			}
		})
	}
}

func TestBuildSlackMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      *alertMsg
		expected *SlackMessage
	}{
		{
			name: "alert message",
			msg: &alertMsg{
				chain:       "test-chain",
				message:     "Test alert message",
				resolved:    false,
				slkMentions: "@here",
			},
			expected: &SlackMessage{
				Text: "Test alert message",
				Attachments: []Attachment{
					{
						Title: "TenderDuty 🚨 ALERT:  test-chain @here",
						Color: "danger",
					},
				},
			},
		},
		{
			name: "resolved message",
			msg: &alertMsg{
				chain:       "test-chain",
				message:     "Test resolved message",
				resolved:    true,
				slkMentions: "@here",
			},
			expected: &SlackMessage{
				Text: "OK: Test resolved message",
				Attachments: []Attachment{
					{
						Title: "TenderDuty 💜 Resolved:  test-chain @here",
						Color: "good",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSlackMessage(tt.msg)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("buildSlackMessage() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestBuildDiscordMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      *alertMsg
		expected *DiscordMessage
	}{
		{
			name: "alert message",
			msg: &alertMsg{
				chain:    "test-chain",
				message:  "Test alert message",
				resolved: false,
			},
			expected: &DiscordMessage{
				Username: "Tenderduty",
				Content:  "🚨 ALERT: test-chain",
				Embeds: []DiscordEmbed{
					{
						Description: "Test alert message",
					},
				},
			},
		},
		{
			name: "resolved message",
			msg: &alertMsg{
				chain:    "test-chain",
				message:  "Test resolved message",
				resolved: true,
			},
			expected: &DiscordMessage{
				Username: "Tenderduty",
				Content:  "💜 Resolved: test-chain",
				Embeds: []DiscordEmbed{
					{
						Description: "Test resolved message",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildDiscordMessage(tt.msg)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("buildDiscordMessage() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

func TestNotifySlack(t *testing.T) {
	tests := []struct {
		name           string
		msg            *alertMsg
		serverResponse int
		expectError    bool
	}{
		{
			name: "successful notification",
			msg: &alertMsg{
				slk:         true,
				chain:       "test-chain",
				message:     "test message",
				resolved:    false,
				slkMentions: "@here",
				slkHook:     "", // will be set to test server URL
			},
			serverResponse: 200,
			expectError:    false,
		},
		{
			name: "server error",
			msg: &alertMsg{
				slk:         true,
				chain:       "test-chain",
				message:     "test message",
				resolved:    false,
				slkMentions: "@here",
				slkHook:     "", // will be set to test server URL
			},
			serverResponse: 500,
			expectError:    true,
		},
		{
			name: "slack disabled",
			msg: &alertMsg{
				slk: false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.msg.slk {
				// Create test server
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.serverResponse)
				}))
				defer server.Close()
				tt.msg.slkHook = server.URL
			}

			err := notifySlack(tt.msg)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestConfigAlert(t *testing.T) {
	// Create test config
	config := &Config{
		alertChan: make(chan *alertMsg, 10),
		chainsMux: sync.RWMutex{},
		Chains: map[string]*ChainConfig{
			"test-chain": {
				ChainId:    "test-chain-1",
				ValAddress: "testval123",
				Alerts: AlertConfig{
					Pagerduty: PDConfig{
						Enabled: &[]bool{true}[0],
						ApiKey:  "test-key",
					},
					Discord: DiscordConfig{
						Enabled: &[]bool{false}[0],
					},
					Telegram: TeleConfig{
						Enabled: &[]bool{true}[0],
						ApiKey:  "tg-key",
						Channel: "test-channel",
					},
					Slack: SlackConfig{
						Enabled: &[]bool{false}[0],
					},
				},
			},
		},
		DefaultAlertConfig: AlertConfig{
			Pagerduty: PDConfig{
				Enabled: &[]bool{true}[0],
			},
			Discord: DiscordConfig{
				Enabled: &[]bool{true}[0],
			},
			Telegram: TeleConfig{
				Enabled: &[]bool{true}[0],
			},
			Slack: SlackConfig{
				Enabled: &[]bool{true}[0],
			},
		},
	}

	alertID := "test_alert_id"
	config.alert("test-chain", "test message", "critical", false, &alertID)

	// Check that alert was sent to channel
	select {
	case alertMsg := <-config.alertChan:
		if alertMsg.message != "test message" {
			t.Errorf("Expected message 'test message', got '%s'", alertMsg.message)
		}
		if alertMsg.severity != "critical" {
			t.Errorf("Expected severity 'critical', got '%s'", alertMsg.severity)
		}
		if alertMsg.uniqueId != "test_alert_id" {
			t.Errorf("Expected uniqueId 'test_alert_id', got '%s'", alertMsg.uniqueId)
		}
		if alertMsg.pd != true {
			t.Errorf("Expected pagerduty to be enabled")
		}
		if alertMsg.tg != true {
			t.Errorf("Expected telegram to be enabled")
		}
		if alertMsg.disc != false {
			t.Errorf("Expected discord to be disabled")
		}
		if alertMsg.slk != false {
			t.Errorf("Expected slack to be disabled")
		}
	case <-time.After(time.Second):
		t.Error("Alert was not sent to channel")
	}
}

func TestApplyAlertDefaultsCustom(t *testing.T) {
	// Create default config
	defaultConfig := &AlertConfig{
		Stalled:           &[]int{10}[0],
		StalledAlerts:     &[]bool{true}[0],
		ConsecutiveMissed: &[]int{5}[0],
		ConsecutiveAlerts: &[]bool{true}[0],
		Pagerduty: PDConfig{
			Enabled:         &[]bool{true}[0],
			DefaultSeverity: "critical",
		},
	}

	// Create chain config with some values set
	chainConfig := &AlertConfig{
		Stalled: &[]int{15}[0], // This should not be overridden
		// StalledAlerts not set, should get default
		// ConsecutiveMissed not set, should get default
		ConsecutiveAlerts: &[]bool{false}[0], // This should not be overridden
		Pagerduty: PDConfig{
			Enabled: &[]bool{false}[0], // This should not be overridden
			// DefaultSeverity not set, should get default
		},
	}

	applyAlertDefaults(chainConfig, defaultConfig)

	// Check that zero values were filled in
	if *chainConfig.Stalled != 15 {
		t.Errorf("Expected Stalled to remain 15, got %d", *chainConfig.Stalled)
	}
	if *chainConfig.StalledAlerts != true {
		t.Errorf("Expected StalledAlerts to be set to default true")
	}
	if *chainConfig.ConsecutiveMissed != 5 {
		t.Errorf("Expected ConsecutiveMissed to be set to default 5, got %d", *chainConfig.ConsecutiveMissed)
	}
	if *chainConfig.ConsecutiveAlerts != false {
		t.Errorf("Expected ConsecutiveAlerts to remain false")
	}
	if *chainConfig.Pagerduty.Enabled != false {
		t.Errorf("Expected Pagerduty.Enabled to remain false")
	}
	if chainConfig.Pagerduty.DefaultSeverity != "critical" {
		t.Errorf("Expected Pagerduty.DefaultSeverity to be set to default 'critical', got '%s'", chainConfig.Pagerduty.DefaultSeverity)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectFatal   bool
		expectWarning bool
		description   string
	}{
		{
			name: "valid config",
			config: &Config{
				EnableDash:  true,
				Listen:      "http://localhost:8080",
				NodeDownMin: 5,
				DefaultAlertConfig: AlertConfig{
					Pagerduty: PDConfig{
						Enabled: &[]bool{true}[0],
						ApiKey:  "ValidKeyWithoutSpecialChars",
					},
				},
				GovernanceAlertsReminderInterval: 6,
				Chains: map[string]*ChainConfig{
					"test": {
						ChainId: "test-1",
					},
				},
			},
			expectFatal:   false,
			expectWarning: false,
			description:   "Valid configuration should not produce errors or warnings",
		},
		{
			name: "invalid pagerduty key",
			config: &Config{
				DefaultAlertConfig: AlertConfig{
					Pagerduty: PDConfig{
						Enabled: &[]bool{true}[0],
						ApiKey:  "invalid+key-with_special",
					},
				},
				Chains: map[string]*ChainConfig{
					"test": {
						ChainId: "test-1",
					},
				},
			},
			expectFatal: true,
			description: "Invalid PagerDuty key should produce fatal error",
		},
		{
			name: "node down minutes too low",
			config: &Config{
				NodeDownMin: 2,
				Chains: map[string]*ChainConfig{
					"test": {
						ChainId: "test-1",
					},
				},
			},
			expectWarning: true,
			description:   "NodeDownMin < 3 should produce warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set default values that might be modified by validateConfig
			if tt.config.GovernanceAlertsReminderInterval == 0 {
				tt.config.GovernanceAlertsReminderInterval = 0 // Let validateConfig set it
			}

			fatal, problems := validateConfig(tt.config)

			if tt.expectFatal && !fatal {
				t.Errorf("%s: Expected fatal error but got none. Problems: %v", tt.description, problems)
			}
			if !tt.expectFatal && fatal {
				t.Errorf("%s: Expected no fatal error but got one. Problems: %v", tt.description, problems)
			}
			if tt.expectWarning && len(problems) == 0 {
				t.Errorf("%s: Expected warnings but got none", tt.description)
			}

			// Check that governance reminder interval is set to default when invalid
			if tt.config.GovernanceAlertsReminderInterval <= 0 {
				if tt.config.GovernanceAlertsReminderInterval != 6 {
					t.Errorf("Expected GovernanceAlertsReminderInterval to be set to 6, got %d", tt.config.GovernanceAlertsReminderInterval)
				}
			}
		})
	}
}

// TestChainConfigMkUpdate tests the mkUpdate method
func TestChainConfigMkUpdate(t *testing.T) {
	cc := &ChainConfig{
		name:    "test-chain",
		ChainId: "test-chain-1",
		valInfo: &ValInfo{
			Moniker: "test-validator",
		},
	}

	update := cc.mkUpdate(metricNodeDownSeconds, 123.45, "http://node1.example.com")

	if update.metric != metricNodeDownSeconds {
		t.Errorf("Expected metric to be metricNodeDownSeconds")
	}
	if update.counter != 123.45 {
		t.Errorf("Expected counter to be 123.45, got %f", update.counter)
	}
	if update.name != "test-chain" {
		t.Errorf("Expected name to be 'test-chain', got '%s'", update.name)
	}
	if update.chainId != "test-chain-1" {
		t.Errorf("Expected chainId to be 'test-chain-1', got '%s'", update.chainId)
	}
	if update.moniker != "test-validator" {
		t.Errorf("Expected moniker to be 'test-validator', got '%s'", update.moniker)
	}
	if update.endpoint != "http://node1.example.com" {
		t.Errorf("Expected endpoint to be 'http://node1.example.com', got '%s'", update.endpoint)
	}
}
