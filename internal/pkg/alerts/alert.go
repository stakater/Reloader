package alert

import (
	"fmt"
	"os"
	"strings"

	"github.com/parnurzeal/gorequest"
	"github.com/sirupsen/logrus"
)

type AlertSink string

const (
	AlertSinkSlack      AlertSink = "slack"
	AlertSinkTeams      AlertSink = "teams"
	AlertSinkGoogleChat AlertSink = "gchat"
	AlertSinkRaw        AlertSink = "raw"
)

// function to send alert msg to webhook service
func SendWebhookAlert(msg string) {
	webhook_url, ok := os.LookupEnv("ALERT_WEBHOOK_URL")
	if !ok {
		logrus.Error("ALERT_WEBHOOK_URL env variable not provided")
		return
	}
	webhook_url = strings.TrimSpace(webhook_url)
	alert_sink := os.Getenv("ALERT_SINK")
	alert_sink = strings.ToLower(strings.TrimSpace(alert_sink))

	// Provision to add Proxy to reach webhook server if required
	webhook_proxy := os.Getenv("ALERT_WEBHOOK_PROXY")
	webhook_proxy = strings.TrimSpace(webhook_proxy)

	// Provision to add Additional information in the alert. e.g ClusterName
	alert_additional_info, ok := os.LookupEnv("ALERT_ADDITIONAL_INFO")
	if ok {
		alert_additional_info = strings.TrimSpace(alert_additional_info)
		msg = fmt.Sprintf("%s : %s", alert_additional_info, msg)
	}

	switch AlertSink(alert_sink) {
	case AlertSinkSlack:
		sendSlackAlert(webhook_url, webhook_proxy, msg)
	case AlertSinkTeams:
		sendTeamsAlert(webhook_url, webhook_proxy, msg)
	case AlertSinkGoogleChat:
		sendGoogleChatAlert(webhook_url, webhook_proxy, msg)
	default:
		msg = strings.ReplaceAll(msg, "*", "")
		sendRawWebhookAlert(webhook_url, webhook_proxy, msg)
	}
}

// function to handle server redirection
func redirectPolicy(req gorequest.Request, via []gorequest.Request) error {
	return fmt.Errorf("incorrect token (redirection)")
}

// function to send alert to slack
func sendSlackAlert(webhookUrl string, proxy string, msg string) []error {
	attachment := Attachment{
		Text:       msg,
		Color:      "good",
		AuthorName: "Reloader",
	}

	payload := WebhookMessage{
		Attachments: []Attachment{attachment},
	}

	request := gorequest.New().Proxy(proxy)
	resp, _, err := request.
		Post(webhookUrl).
		RedirectPolicy(redirectPolicy).
		Send(payload).
		End()

	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return []error{fmt.Errorf("error sending msg. status: %v", resp.Status)}
	}

	return nil
}

// function to send alert to Microsoft Teams webhook
func sendTeamsAlert(webhookUrl string, proxy string, msg string) []error {
	attachment := Attachment{
		Text: msg,
	}

	request := gorequest.New().Proxy(proxy)
	resp, _, err := request.
		Post(webhookUrl).
		RedirectPolicy(redirectPolicy).
		Send(attachment).
		End()

	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return []error{fmt.Errorf("error sending msg. status: %v", resp.Status)}
	}

	return nil
}

// function to send alert to Google Chat webhook
func sendGoogleChatAlert(webhookUrl string, proxy string, msg string) []error {
	payload := map[string]interface{}{
		"text": msg,
	}

	request := gorequest.New().Proxy(proxy)
	resp, _, err := request.
		Post(webhookUrl).
		RedirectPolicy(redirectPolicy).
		Send(payload).
		End()

	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return []error{fmt.Errorf("error sending msg. status: %v", resp.Status)}
	}

	return nil
}

// function to send alert to webhook service as text
func sendRawWebhookAlert(webhookUrl string, proxy string, msg string) []error {
	request := gorequest.New().Proxy(proxy)
	resp, _, err := request.
		Post(webhookUrl).
		Type("text").
		RedirectPolicy(redirectPolicy).
		Send(msg).
		End()

	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return []error{fmt.Errorf("error sending msg. status: %v", resp.Status)}
	}

	return nil
}
