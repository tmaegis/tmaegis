// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tmaegis/tmaegis/internal/engine"
)

type Config struct {
	Provider string // "ollama", "claude", "openai", "gemini"
	APIKey   string
	Model    string
	Endpoint string
	// HostContext is an optional pre-flight snapshot (running services, open ports) that
	// lets the model flag remediations likely to disrupt live workloads.
	HostContext string
}

type Analyzer interface {
	Name() string
	AnalyzeScanReport(ctx context.Context, report *engine.ScanReport) (string, error)
}

func NewAnalyzer(cfg Config) (Analyzer, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "ollama"
	}

	cfg.Provider = provider // normalised, so callers and Name() never see ""

	switch provider {
	case "ollama":
		if cfg.Endpoint == "" {
			cfg.Endpoint = "http://localhost:11434"
		}
		if cfg.Model == "" {
			cfg.Model = "qwen3.5:9b"
		}
		return &MultiLLMConnector{cfg: cfg}, nil
	case "claude", "openai", "gemini":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider '%s' requires TMAEGIS_AI_API_KEY (or use 'ollama' for 100%% free local AI)", provider)
		}
		return &MultiLLMConnector{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", provider)
	}
}

type MultiLLMConnector struct {
	cfg Config
}

func (m *MultiLLMConnector) Name() string {
	return fmt.Sprintf("%s (%s)", strings.ToUpper(m.cfg.Provider), m.cfg.Model)
}

// RedactSensitiveData strips private IPs, passwords, and keys before sending prompts to any LLM.
func RedactSensitiveData(input string) string {
	ipRegex := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	redacted := ipRegex.ReplaceAllString(input, "[REDACTED_IP]")

	secretRegex := regexp.MustCompile(`(?i)(password|secret|token|key)\s*[:=]\s*[^\s"']+`)
	redacted = secretRegex.ReplaceAllString(redacted, "$1=[REDACTED_SECRET]")
	return redacted
}

func (m *MultiLLMConnector) AnalyzeScanReport(ctx context.Context, report *engine.ScanReport) (string, error) {
	var failedSummaries []string
	for _, res := range report.Results {
		if res.Status == engine.StatusFail {
			failedSummaries = append(failedSummaries, fmt.Sprintf("- %s (%s): %s", res.Rule.ID, res.Rule.Title, res.Details))
		}
	}

	if len(failedSummaries) == 0 {
		return "All evaluated CIS controls passed! No remediation risk detected.", nil
	}

	rawPrompt := fmt.Sprintf(
		"You are TMAegis SecOps AI. Analyze these failed CIS Benchmark checks on %s:\n%s\n\n",
		report.OS, strings.Join(failedSummaries, "\n"),
	)
	if m.cfg.HostContext != "" {
		rawPrompt += "Pre-flight snapshot of this host (use it to spot services a fix could break):\n" + m.cfg.HostContext + "\n\n"
	}
	rawPrompt += "Provide: 1) Operational impact if remediated in production (flag any running service that depends on the setting), " +
		"2) Safe remediation order, 3) Verification commands."
	safePrompt := RedactSensitiveData(rawPrompt)

	switch strings.ToLower(m.cfg.Provider) {
	case "ollama":
		return m.callOllama(ctx, safePrompt)
	case "openai":
		return m.callOpenAI(ctx, safePrompt)
	case "claude":
		return m.callClaude(ctx, safePrompt)
	case "gemini":
		return m.callGemini(ctx, safePrompt)
	default:
		return "", fmt.Errorf("provider not implemented")
	}
}

func (m *MultiLLMConnector) callOllama(ctx context.Context, prompt string) (string, error) {
	url := strings.TrimRight(m.cfg.Endpoint, "/") + "/api/generate"
	body, _ := json.Marshal(map[string]interface{}{
		"model":  m.cfg.Model,
		"prompt": prompt,
		"stream": false,
	})
	return doJSONPost(ctx, url, body, nil, func(respMap map[string]interface{}) (string, error) {
		if r, ok := respMap["response"].(string); ok {
			return r, nil
		}
		return "", fmt.Errorf("unexpected ollama response format")
	})
}

func (m *MultiLLMConnector) callOpenAI(ctx context.Context, prompt string) (string, error) {
	model := m.cfg.Model
	if model == "" {
		model = "gpt-6-astra"
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	headers := map[string]string{"Authorization": "Bearer " + m.cfg.APIKey}
	return doJSONPost(ctx, "https://api.openai.com/v1/chat/completions", body, headers, func(respMap map[string]interface{}) (string, error) {
		choices, ok := respMap["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			return "", fmt.Errorf("empty choices from OpenAI")
		}
		msg := choices[0].(map[string]interface{})["message"].(map[string]interface{})
		return msg["content"].(string), nil
	})
}

func (m *MultiLLMConnector) callClaude(ctx context.Context, prompt string) (string, error) {
	model := m.cfg.Model
	if model == "" {
		model = "claude-sonnet-5-5"
	}
	body, _ := json.Marshal(map[string]interface{}{
		"model":      model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})
	headers := map[string]string{
		"x-api-key":         m.cfg.APIKey,
		"anthropic-version": "2023-06-01",
	}
	return doJSONPost(ctx, "https://api.anthropic.com/v1/messages", body, headers, func(respMap map[string]interface{}) (string, error) {
		content, ok := respMap["content"].([]interface{})
		if !ok || len(content) == 0 {
			return "", fmt.Errorf("empty content from Claude")
		}
		return content[0].(map[string]interface{})["text"].(string), nil
	})
}

func (m *MultiLLMConnector) callGemini(ctx context.Context, prompt string) (string, error) {
	model := m.cfg.Model
	if model == "" {
		model = "gemini-3.8-flash"
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	body, _ := json.Marshal(map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": prompt}}},
		},
	})
	// The key goes in a header, not the URL, so it never lands in proxy or access logs.
	headers := map[string]string{"x-goog-api-key": m.cfg.APIKey}
	return doJSONPost(ctx, url, body, headers, func(respMap map[string]interface{}) (string, error) {
		cands, ok := respMap["candidates"].([]interface{})
		if !ok || len(cands) == 0 {
			return "", fmt.Errorf("empty candidates from Gemini")
		}
		parts := cands[0].(map[string]interface{})["content"].(map[string]interface{})["parts"].([]interface{})
		return parts[0].(map[string]interface{})["text"].(string), nil
	})
}

func doJSONPost(ctx context.Context, url string, payload []byte, headers map[string]string, extractor func(map[string]interface{}) (string, error)) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", err
	}
	return extractor(parsed)
}
