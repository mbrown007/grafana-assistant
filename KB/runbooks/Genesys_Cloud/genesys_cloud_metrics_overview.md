# Genesys Cloud Monitoring Context for Grafana Agent

## Overview
This Grafana instance monitors a Genesys Cloud contact center platform using a custom Prometheus exporter. The exporter collects metrics across infrastructure, call quality, security, billing, and user activity dimensions.

## What is Genesys Cloud?
Genesys Cloud is a cloud-based contact center platform (CCaaS - Contact Center as a Service) that provides:
- **Voice & Digital Channels**: Inbound/outbound calling, chat, email, SMS
- **Agent Workspace**: Routing, presence, interaction handling
- **Telephony Infrastructure**: SIP trunks, Edge servers (on-premise gateways)
- **Analytics**: Call quality (MOS scores), conversation analytics
- **Security & Compliance**: Audit logging, user authentication, role-based access control
- **Integrations**: CRM, ticketing, workforce management systems

## Metric Namespace
All metrics use the prefix `genesyscloud_` followed by the collector name and metric name:
```
genesyscloud_{collector}_{metric_name}
```

Example: `genesyscloud_edge_cpu_percent`, `genesyscloud_audit_events_total`

---

## Available Collectors & Metrics

### 1. Edge Collector (`edge`)
**Purpose**: Monitors on-premise Edge servers that provide local PSTN connectivity and survivability

**Key Metrics**:
- `genesyscloud_edge_status` - Edge server operational state (labels: `edge_name`, `state`)
- `genesyscloud_edge_cpu_percent` - CPU utilization by core (labels: `edge_name`, `cpu_id`, `state` [idle/active/privileged/user])
- `genesyscloud_edge_memory_available_bytes` - Available memory (labels: `edge_name`, `type`)
- `genesyscloud_edge_memory_total_bytes` - Total memory capacity
- `genesyscloud_edge_disk_available_bytes` - Disk space available (labels: `edge_name`, `partition`)
- `genesyscloud_edge_disk_total_bytes` - Total disk capacity
- `genesyscloud_edge_network_received_bytes_per_second` - Network ingress rate (labels: `edge_name`, `interface`)
- `genesyscloud_edge_network_sent_bytes_per_second` - Network egress rate
- `genesyscloud_edge_network_used_percent` - Network utilization percentage

**Scrape Frequency**: 1 minute  
**Use Cases**: Infrastructure health, capacity planning, survivability monitoring

---

### 2. Trunk Collector (`trunk`)
**Purpose**: Monitors SIP trunk status and call volume across telephony connections

**Key Metrics**:
- `genesyscloud_trunk_inbound_calls` - Inbound call count (labels: `edge_name`, `trunk_name`)
- `genesyscloud_trunk_outbound_calls` - Outbound call count
- `genesyscloud_trunk_qos_mismatch` - QoS mismatches indicating network issues
- `genesyscloud_trunk_in_service` - Trunk operational status (1=in service, 0=out of service)
- `genesyscloud_trunk_enabled` - Trunk administrative state (1=enabled, 0=disabled)
- `genesyscloud_trunk_connected_status` - SIP connection status (1=connected, 0=disconnected)

**Scrape Frequency**: 1 minute  
**Use Cases**: Call routing health, trunk capacity utilization, SIP connectivity monitoring

---

### 3. User Collector (`user`)
**Purpose**: Tracks agent presence, routing status, and geographic distribution

**Key Metrics**:
- `genesyscloud_user_routing_status` - Agent routing availability (labels: `user_id`, `email`, `status`, `region`, `country`)
  - Status values: `ON_QUEUE`, `OFF_QUEUE`, `IDLE`, `INTERACTING`, `NOT_RESPONDING`
- `genesyscloud_user_presence_status` - Agent presence indicator (labels: `user_id`, `email`, `status`, `region`, `country`)
  - Status values: `AVAILABLE`, `AWAY`, `BUSY`, `BREAK`, `MEAL`, `MEETING`, `OFFLINE`, `TRAINING`

**Scrape Frequency**: 5 minutes  
**Use Cases**: Agent availability monitoring, workforce distribution, capacity forecasting

---

### 4. MOS Collector (`mos`)
**Purpose**: Monitors voice call quality using Mean Opinion Score (MOS) ratings

**Key Metrics**:
- `genesyscloud_mos_voice_session_latest` - Latest call quality distribution (labels: `rating`)
  - Ratings: `EXCELLENT` (4.3-5.0), `GOOD` (4.0-4.3), `FAIR` (3.5-4.0), `POOR` (3.0-3.5), `BAD` (<3.0)
- `genesyscloud_mos_voice_session_total` - Total voice sessions analyzed (counter)
- `genesyscloud_mos_voice_session_api_success_time_seconds` - Last successful API response timestamp
- `genesyscloud_mos_voice_session_api_failure_count` - API failure counter

**Scrape Frequency**: 1 minute  
**Use Cases**: Call quality monitoring, network issue detection, customer experience tracking

---

### 5. Audit Collector (`audit`)
**Purpose**: Monitors security audit events and configuration changes

**Key Metrics**:
- `genesyscloud_audit_events_total` - Audit event count (labels: `service_name`, `entity_type`, `action`, `status`)
  - Entity types: `MFAVerifier`, `Role`, `OAuthClient`, `User`, `Group`, `Division`, `Integration`
  - Actions: `CREATE`, `UPDATE`, `DELETE`, `READ`, `AUTHENTICATE`
  - Status: `SUCCESS`, `FAILURE`
- `genesyscloud_audit_high_risk_changes_total` - High-risk configuration changes (labels: `change_type`, `user_id`, `target_id`)
  - High-risk changes: OAuth client creation, role modifications, MFA disabling, privilege escalation
- `genesyscloud_audit_failed_access_attempts_total` - Failed access attempts (labels: `user_id`, `resource`, `outcome`)

**Scrape Frequency**: Configurable (default 5 minutes with 3-minute API lag compensation)  
**Use Cases**: Security monitoring, compliance auditing, breach detection, change tracking

---

### 6. Billing Collector (`billing`)
**Purpose**: Tracks license consumption by division

**Key Metrics**:
- `genesyscloud_billing_resource` - License usage count (labels: `name` [license type], `division`)
  - License types: `COMMUNICATOR`, `COLLABORATE`, `COMMUNICATE`, `COLLABORATE_AND_SHARE`, etc.
- `genesyscloud_billing_start_day` - Billing period start day of month

**Scrape Frequency**: 60 minutes  
**Use Cases**: License utilization, cost allocation, chargeback reporting

---

### 7. Trustee Billing Collector (`trusteebilling`)
**Purpose**: Monitors billing across trusted organization relationships (multi-org deployments)

**Key Metrics**:
- `genesyscloud_trusteebilling_resource` - Cross-org license usage (labels: `name`, `organization`)

**Scrape Frequency**: 60 minutes  
**Use Cases**: Multi-tenant billing, shared service cost tracking

---

### 8. Authorization Collector (`authorization`)
**Purpose**: Tracks role assignments and permission grants

**Key Metrics**:
- `genesyscloud_authorization_role_assignments` - User-to-role mappings (labels: `role_name`, `user_id`, `division`)
- `genesyscloud_authorization_permission_grants` - Permission policies (labels: `permission`, `role_id`)

**Scrape Frequency**: 30 minutes  
**Use Cases**: Access control auditing, least privilege compliance, role sprawl detection

---

### 9. OAuth Clients Collector (`oauth_clients`)
**Purpose**: Monitors OAuth client lifecycle and security posture

**Key Metrics**:
- `genesyscloud_oauth_client_info` - OAuth client metadata (labels: `client_id`, `name`, `state`, `authorized_grant_type`)
- `genesyscloud_oauth_client_days_since_secret_rotation` - Days since last secret change
- `genesyscloud_oauth_client_days_until_expiry` - Days until client expires

**Scrape Frequency**: 60 minutes  
**Use Cases**: Integration health, credential rotation compliance, stale client detection

---

### 10. User Security Collector (`usersecurity`)
**Purpose**: Identifies security risks in user accounts

**Key Metrics**:
- `genesyscloud_usersecurity_dormant_accounts` - Inactive user accounts (labels: `user_id`, `email`, `days_since_last_login`)
- `genesyscloud_usersecurity_failed_login_attempts` - Failed authentication events (labels: `user_id`, `ip_address`)
- `genesyscloud_usersecurity_mfa_status` - MFA enrollment status (labels: `user_id`, `email`, `mfa_enabled`)

**Scrape Frequency**: 60 minutes  
**Use Cases**: Account hygiene, brute force detection, MFA compliance monitoring

---

### 11. Organization Trust Collector (`orgtrust`)
**Purpose**: Monitors cross-organization trust relationships

**Key Metrics**:
- `genesyscloud_orgtrust_relationship_status` - Trust relationship state (labels: `trustor_org`, `trustee_org`, `status`)
- `genesyscloud_orgtrust_user_access` - Cross-org user access grants (labels: `user_id`, `org_id`, `role`)

**Scrape Frequency**: 60 minutes  
**Use Cases**: Multi-org security, privilege escalation detection, trust boundary monitoring

---

### 12. Integration Collector (`integrations`)
**Purpose**: Tracks third-party integration health and status

**Key Metrics**:
- `genesyscloud_integration_status` - Integration operational state (labels: `integration_id`, `name`, `type`, `status`)
- `genesyscloud_integration_last_execution` - Last execution timestamp (labels: `integration_id`)

**Scrape Frequency**: 15 minutes  
**Use Cases**: Integration availability, automation health, third-party dependency monitoring

---

### 13. Organization Collector (`organization`)
**Purpose**: Provides organization-level metadata and configuration

**Key Metrics**:
- `genesyscloud_organization_info` - Org metadata (labels: `org_id`, `name`, `region`, `version`)
- `genesyscloud_organization_feature_enabled` - Feature flag status (labels: `feature_name`, `enabled`)

**Scrape Frequency**: 60 minutes  
**Use Cases**: Configuration tracking, feature adoption, multi-region monitoring

---

### 14. Conversation Collector (`conversations`)
**Purpose**: Aggregates conversation/call metrics

**Key Metrics**:
- `genesyscloud_conversation_count` - Conversation totals (labels: `media_type`, `direction`, `status`)
- `genesyscloud_conversation_duration_seconds` - Interaction duration (labels: `media_type`)

**Scrape Frequency**: 5 minutes  
**Use Cases**: Call volume tracking, channel utilization, SLA monitoring

---

### 15. Recording Collector (`recording`)
**Purpose**: Detects orphaned call recordings

**Key Metrics**:
- `genesyscloud_recording_orphaned_count` - Orphaned recordings detected (labels: `recording_id`, `age_days`)

**Scrape Frequency**: 60 minutes  
**Use Cases**: Storage waste detection, compliance cleanup

---

### 16. API Collector (`api`)
**Purpose**: Monitors platform API usage and rate limits

**Key Metrics**:
- `genesyscloud_api_requests_total` - API call count (labels: `endpoint`, `method`, `status_code`)
- `genesyscloud_api_rate_limit_remaining` - Rate limit headroom (labels: `endpoint`)

**Scrape Frequency**: 5 minutes  
**Use Cases**: API quota management, throttling detection, usage forecasting

---

### 17. Usage Collector (`usage`)
**Purpose**: Tracks resource consumption and quota limits

**Key Metrics**:
- `genesyscloud_usage_quota_limit` - Resource quota ceiling (labels: `resource_type`)
- `genesyscloud_usage_quota_used` - Current resource consumption

**Scrape Frequency**: 30 minutes  
**Use Cases**: Capacity planning, quota alerting, growth trending

---

## Common Query Patterns

### Agent Availability
```promql
# Count of agents currently on queue
count(genesyscloud_user_routing_status{status="ON_QUEUE"})

# Presence distribution
sum by (status) (genesyscloud_user_presence_status)
```

### Call Quality
```promql
# Percentage of poor quality calls
sum(genesyscloud_mos_voice_session_latest{rating=~"POOR|BAD"})
/ sum(genesyscloud_mos_voice_session_latest) * 100

# Call quality trend over time
rate(genesyscloud_mos_voice_session_latest{rating="EXCELLENT"}[5m])
```

### Infrastructure Health
```promql
# Edge server CPU usage
100 - genesyscloud_edge_cpu_percent{state="idle"}

# Trunk utilization rate
rate(genesyscloud_trunk_inbound_calls[5m]) + rate(genesyscloud_trunk_outbound_calls[5m])

# Disk space remaining
(genesyscloud_edge_disk_available_bytes / genesyscloud_edge_disk_total_bytes) * 100
```

### Security Monitoring
```promql
# High-risk changes in last hour
increase(genesyscloud_audit_high_risk_changes_total[1h])

# Failed login attempts
rate(genesyscloud_usersecurity_failed_login_attempts[5m])

# Dormant accounts without MFA
count(genesyscloud_usersecurity_dormant_accounts{mfa_enabled="false"})
```

### Billing & Licensing
```promql
# Total licenses in use
sum(genesyscloud_billing_resource)

# License usage by type
sum by (name) (genesyscloud_billing_resource)

# License utilization trend
rate(genesyscloud_billing_resource[1h])
```

---

## Alert Rules Location
Alert rule definitions are located in the `alerts/` directory:
- `agents.yaml` - Agent availability and staffing alerts
- `api.yaml` - API rate limit and availability alerts
- `audit.yaml` - Audit event and compliance alerts
- `audit-security-alerts.yaml` - Security-specific audit alerts
- `authorization.yaml` - Role and permission alerts
- `billing.yaml` - License usage and cost alerts
- `edge.yaml` - Edge server health alerts
- `integrations-alerts.yaml` - Integration status alerts
- `mos.yaml` - Call quality alerts
- `oauth-clients-alerts.yaml` - OAuth client security alerts
- `organization.yaml` - Organization configuration alerts
- `orgtrust.yaml` - Cross-org trust relationship alerts
- `trunk.yaml` - Trunk connectivity and capacity alerts
- `usersecurity.yaml` - User account security alerts

---

## Dashboard Categories
Dashboards are organized in the `dashboards/` directory:

**Security Dashboards**:
- `security-dashboard.json` - Overall security posture
- `security-overview.json` - Security metrics summary
- `security-posture-dashboard.json` - Compliance and risk scoring
- `audit-events-investigation.json` - Audit trail analysis
- `oauth-clients-dashboard.json` - OAuth client management
- `organization-trust-dashboard.json` - Cross-org trust monitoring

**Infrastructure Dashboards**:
- `infrastructure-health.json` - Edge and trunk health
- `trunk_status_overview.json` - Trunk connectivity
- `trunks.json` - Detailed trunk metrics
- `cpu.json`, `memory.json`, `disk.json`, `network.json` - Resource utilization

**Call Quality Dashboards**:
- `call-quality-dashboard.json` - MOS and network quality
- `conversationmos.json` - Conversation-level MOS analysis

**Billing Dashboards**:
- `licenses.json` - License consumption
- `subscriptions-billing.json` - Billing overview
- `subscriptions-billing-division.json` - Division-level billing
- `licences-billing-prepay-usage-table.json` - Prepaid usage tracking
- `licences-cx-ai-overview.json` - CX/AI license dashboard

**Operations Dashboards**:
- `overview.json` - System-wide overview
- `exporter_performance.json` - Exporter health metrics
- `integrations-dashboard.json` - Integration status
- `authorization-dashboard.json` - Role and permission analysis

---

## Common Troubleshooting Scenarios

### "No data in dashboard"
1. Check if collector is enabled: `genesyscloud_<collector>_up` metric
2. Verify exporter is running: `up{job="genesys_cloud_exporter"}`
3. Check scrape frequency: Some collectors run hourly (billing, oauth_clients)

### "Call quality alerts firing"
- Query: `genesyscloud_mos_voice_session_latest{rating=~"POOR|BAD"}`
- Check: Edge network metrics, trunk QoS mismatches
- Correlate: `genesyscloud_trunk_qos_mismatch`, `genesyscloud_edge_network_used_percent`

### "Security alerts triggered"
- High-risk changes: `genesyscloud_audit_high_risk_changes_total`
- Failed logins: `genesyscloud_usersecurity_failed_login_attempts`
- Dormant accounts: `genesyscloud_usersecurity_dormant_accounts`

### "Trunk down"
- Check: `genesyscloud_trunk_connected_status == 0`
- Verify: `genesyscloud_trunk_in_service`, `genesyscloud_trunk_enabled`
- Edge status: `genesyscloud_edge_status{state!="ONLINE"}`

### "License quota exceeded"
- Query: `genesyscloud_billing_resource` by division
- Compare: Historical usage trends
- Identify: Unused/dormant accounts consuming licenses

---

## API Rate Limiting Considerations
- Genesys Cloud enforces rate limits per OAuth client
- The exporter implements configurable scrape frequencies to manage API consumption
- High-frequency collectors (edge, trunk, mos): 1-5 minute intervals
- Low-frequency collectors (billing, oauth_clients): 30-60 minute intervals
- Monitor: `genesyscloud_api_rate_limit_remaining` to prevent throttling

---

## Geographic Distribution
User metrics include `region` and `country` labels for:
- Workforce distribution analysis
- Regional performance monitoring
- Compliance with data residency requirements

---

## Time Zone Handling
- All timestamps are in UTC
- Billing periods use organization-configured time zone
- Audit events include timezone-aware timestamps
- Use Grafana's time zone conversion for local display

---

## Metric Cardinality Notes
**High Cardinality** (use carefully in queries):
- `user_id` - Can scale to thousands of agents
- `email` - Same cardinality as users
- `edge_name`, `trunk_name` - Typically 1-50 edges, 10-200 trunks
- `cpu_id`, `partition`, `interface` - Multiplies by number of edges

**Low Cardinality** (safe for aggregation):
- `status`, `state`, `rating` - Fixed enumerations
- `division`, `role_name` - Organizational structure (typically <100)
- `license_name`, `resource_type` - Product SKUs (typically <50)
---