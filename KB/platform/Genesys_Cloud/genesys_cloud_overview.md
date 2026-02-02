# Genesys Cloud Platform Overview

## What is Genesys Cloud?
Genesys Cloud is a cloud-based contact center platform (CCaaS - Contact Center as a Service) that provides:
- **Voice & Digital Channels**: Inbound/outbound calling, chat, email, SMS
- **Agent Workspace**: Routing, presence, interaction handling
- **Telephony Infrastructure**: SIP trunks, Edge servers (on-premise gateways)
- **Analytics**: Call quality (MOS scores), conversation analytics
- **Security & Compliance**: Audit logging, user authentication, role-based access control
- **Integrations**: CRM, ticketing, workforce management systems

## Platform Architecture
Genesys Cloud operates as a multi-tenant SaaS platform with regional deployments. Key architectural components:

- **Edge Servers**: On-premise gateways providing local PSTN connectivity and survivability. These bridge between the cloud platform and local telephony infrastructure.
- **SIP Trunks**: Telephony connections between Edge servers and carriers or PBX systems. Each trunk carries voice traffic for inbound and outbound calls.
- **Agents**: Contact center operators who handle customer interactions across voice, chat, email, and other channels.
- **Routing**: The system that distributes incoming interactions to available agents based on skills, queues, and priority.
- **Presence & Routing Status**: Agents have a presence status (Available, Away, Busy, etc.) and a routing status (On Queue, Off Queue, Interacting, etc.) that determine their availability.

## Monitoring Context
This Grafana instance monitors a Genesys Cloud contact center platform using a custom Prometheus exporter. The exporter collects metrics across infrastructure, call quality, security, billing, and user activity dimensions.

All metrics use the prefix `genesyscloud_` followed by the collector name and metric name:
```
genesyscloud_{collector}_{metric_name}
```

Example: `genesyscloud_edge_cpu_percent`, `genesyscloud_audit_events_total`

## Key Monitoring Dimensions

### Infrastructure
- **Edge Servers**: CPU, memory, disk, network utilization of on-premise gateways
- **Trunks**: SIP trunk status, call counts, QoS mismatches

### Call Quality
- **MOS (Mean Opinion Score)**: Voice quality ratings from Excellent (4.3-5.0) to Bad (<3.0)
- **Session Tracking**: Total voice sessions analyzed, quality distribution

### Security
- **Audit Events**: Configuration changes, authentication events, high-risk operations
- **User Security**: Dormant accounts, failed logins, MFA enrollment status
- **OAuth Clients**: Client lifecycle, secret rotation, expiry tracking

### Operations
- **Agent Activity**: Routing status, presence distribution, geographic spread
- **Billing**: License consumption by division and type
- **Integrations**: Third-party integration health and status
- **API Usage**: Rate limits, request counts, throttling detection

## Geographic Distribution
User metrics include `region` and `country` labels for workforce distribution analysis, regional performance monitoring, and compliance with data residency requirements.

## Time Zone Handling
- All timestamps are in UTC
- Billing periods use organization-configured time zone
- Audit events include timezone-aware timestamps
- Use Grafana's time zone conversion for local display
