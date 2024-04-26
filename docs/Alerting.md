# Alerting on Reload

Reloader can alert when it triggers a rolling upgrade on Deployments or StatefulSets. Webhook notification alert would be sent to the configured webhook server with all the required information.

## Enabling

In-order to enable this feature, you need to update the `reloader.env.secret` section of `values.yaml` providing the information needed for alert:

```yaml
      ALERT_ON_RELOAD: [ true/false ] Default: false
      ALERT_SINK: [ slack/teams/webhook ] Default: webhook
      ALERT_WEBHOOK_URL: Required if ALERT_ON_RELOAD is true
      ALERT_ADDITIONAL_INFO: Any additional information to be added to alert
```

## Slack Incoming-Webhook Creation Docs

[Sending messages using Incoming Webhooks](https://api.slack.com/messaging/webhooks)
