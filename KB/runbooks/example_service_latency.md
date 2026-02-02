# Example Runbook: Service Latency Spike

## Summary

Use this runbook when p95 latency for the API service increases sharply.

## Signals

- Alert: `api_latency_p95_high`
- Dashboard: "API Service Overview"
- Symptom: elevated 5xx rate or timeouts

## Quick Triage

1. Confirm the spike duration and scope (single region vs all regions).
2. Check deploy history for the last 2 hours.
3. Compare p50, p95, and p99 to identify tail-latency shifts.

## Common Causes

- Downstream dependency slowdown (database, cache, external API)
- Saturated worker pool or thread pool
- Slow queries or missing indexes
- Traffic surge or hot partition

## Investigation Steps

### Dependency Health

- Check dependency dashboards for latency and error spikes.
- Look for correlated alerts (DB, cache, queue).

### Resource Saturation

- CPU > 85% sustained for 10+ minutes
- Memory pressure or OOM restarts
- Queue depth growth or backlog

### Recent Changes

- Review deployments, feature flags, or config changes.
- Roll back if latency spike starts immediately after a change.

## Remediation

- Scale service replicas if saturation is confirmed.
- Mitigate slow dependency (failover, reduce load, increase timeouts).
- Roll back the most recent deploy if correlated with the spike.

## Validation

- Latency returns to baseline (p95 within 10% of typical)
- Error rate returns to normal
- No new alerts for 15 minutes

## Escalation

- If latency persists > 30 minutes after remediation, page on-call lead.
- If dependency outage is confirmed, engage the owning team.
