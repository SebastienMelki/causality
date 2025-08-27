# Security Policy

## Supported Versions

The following versions of Causality are currently being supported with security updates:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of Causality seriously. If you discover a security vulnerability, please follow these steps:

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Email your findings to **sebastienmelki@gmail.com** or use GitHub's private vulnerability reporting:
   - Go to the [Security tab](https://github.com/SebastienMelki/causality/security)
   - Click "Report a vulnerability"
   - Provide detailed information about the vulnerability

### What to Include

When reporting a vulnerability, please include:

- Description of the vulnerability and its potential impact
- Steps to reproduce the issue
- Affected components (TCP server, mobile SDK, WASM, analysis engine)
- Version of Causality where the vulnerability was discovered
- Any proof-of-concept code or exploit details
- Your suggested fix or mitigation (if any)

### Response Timeline

- **Initial Response**: Within 48 hours of receipt
- **Status Update**: Within 5 business days
- **Resolution Target**: 
  - Critical vulnerabilities: Within 7 days
  - High severity: Within 14 days
  - Medium/Low severity: Within 30 days

### Responsible Disclosure

We kindly ask that you:

- Allow us reasonable time to address the issue before public disclosure
- Avoid exploiting the vulnerability beyond what's necessary for verification
- Not access, modify, or delete other users' data
- Not perform actions that could harm the service availability

### Recognition

We appreciate your efforts in keeping Causality secure. Contributors who report valid security issues will be:

- Acknowledged in our security advisories (unless you prefer to remain anonymous)
- Added to our Security Hall of Fame
- Considered for bug bounty rewards (once program is established)

## Security Best Practices

When using Causality in production:

### HTTP Server Security

- **Always use HTTPS** for API endpoints in production
- **Implement authentication** for client connections
- **Use rate limiting** to prevent DoS attacks
- **Validate all events** using Protocol Buffer validation
- **Monitor for anomalous request patterns**
- **Implement request timeouts** to prevent resource exhaustion

### Mobile SDK Security

- **Use certificate pinning** for server connections
- **Encrypt sensitive event data** before transmission
- **Implement app attestation** to verify client integrity
- **Store credentials securely** using platform keychains
- **Obfuscate SDK integration** to prevent reverse engineering
- **Sign SDK binaries** to prevent tampering

### WebAssembly Security

- **Validate all inputs** from JavaScript environment
- **Use Content Security Policy** headers appropriately
- **Implement origin validation** for cross-origin requests
- **Minimize exposed API surface** from WASM module
- **Handle memory safely** to prevent buffer overflows
- **Use SubResource Integrity** for WASM file loading

### Event Data Security

- **Sanitize event payloads** to prevent injection attacks
- **Implement field-level encryption** for sensitive data
- **Use data retention policies** to limit exposure
- **Anonymize user identifiers** where possible
- **Audit event access** for compliance
- **Implement data classification** for different sensitivity levels

### Analysis Engine Security

- **Validate all analysis rules** before execution
- **Sandbox custom detection algorithms**
- **Implement resource limits** for analysis operations
- **Protect ML models** from adversarial inputs
- **Audit all detection results** for false positives
- **Secure model updates** with signature verification

### Protocol Buffer Security

- **Set message size limits** to prevent memory exhaustion
- **Validate all field values** using buf.validate annotations
- **Avoid recursive message definitions** that could cause stack overflow
- **Use field masks** to limit data exposure
- **Version protocols** to maintain backward compatibility
- **Document security considerations** for each message type

## Security Features

Causality includes several built-in security features:

### Authentication & Authorization

- **Client certificate validation** for TCP connections
- **API key authentication** for SDK clients
- **Role-based access control** for analysis results
- **JWT token support** for web clients

### Data Protection

- **End-to-end encryption** for sensitive events
- **At-rest encryption** for stored events
- **Perfect forward secrecy** for connections
- **Data masking** for logs and debugging

### Monitoring & Auditing

- **Security event logging** for all operations
- **Anomaly detection** for system behavior
- **Audit trails** for configuration changes
- **Real-time alerting** for security incidents

### Network Security

- **DDoS protection** through rate limiting
- **IP allowlisting/blocklisting** support
- **Geo-blocking** capabilities
- **Connection throttling** per client

## Security Checklist for Deployment

### Pre-Production

- [ ] Enable TLS for all connections
- [ ] Configure authentication mechanisms
- [ ] Set up rate limiting rules
- [ ] Review and update dependencies
- [ ] Perform security scanning
- [ ] Configure security headers
- [ ] Set up monitoring and alerting

### Production

- [ ] Regular security updates
- [ ] Continuous vulnerability scanning
- [ ] Security log monitoring
- [ ] Incident response plan in place
- [ ] Regular security audits
- [ ] Penetration testing (annually)
- [ ] Compliance verification

## Security Tools Integration

Causality integrates with common security tools:

- **SIEM Integration**: Export security events to SIEM systems
- **Vulnerability Scanners**: Compatible with common scanners
- **WAF Support**: Works behind Web Application Firewalls
- **IDS/IPS**: Provides hooks for intrusion detection
- **Secret Management**: Integrates with vault systems

## Compliance

Causality can be configured to meet various compliance requirements:

- **GDPR**: Data protection and privacy controls
- **HIPAA**: Healthcare data security features
- **PCI DSS**: Payment card industry standards
- **SOC 2**: Security and availability controls
- **ISO 27001**: Information security management

## Contact

For security concerns, contact:

- **Email**: sebastienmelki@gmail.com
- **GitHub Security Advisories**: [Report a vulnerability](https://github.com/SebastienMelki/causality/security/advisories/new)

For general security questions or to discuss security best practices, please use [GitHub Discussions](https://github.com/SebastienMelki/causality/discussions).

## Security Updates

Security updates and advisories will be posted to:

- GitHub Security Advisories
- Project releases page
- Security mailing list (subscribe via project settings)

Thank you for helping keep Causality and its users safe!