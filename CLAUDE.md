# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## AI Guidance

* Ignore GEMINI.md and GEMINI-*.md files
* To save main context space, for code searches, inspections, troubleshooting or analysis, use code-searcher subagent where appropriate - giving the subagent full context background for the task(s) you assign it.
* After receiving tool results, carefully reflect on their quality and determine optimal next steps before proceeding. Use your thinking to plan and iterate based on this new information, and then take the best next action.
* For maximum efficiency, whenever you need to perform multiple independent operations, invoke all relevant tools simultaneously rather than sequentially.
* Before you finish, please verify your solution
* Do what has been asked; nothing more, nothing less.
* NEVER create files unless they're absolutely necessary for achieving your goal.
* ALWAYS prefer editing an existing file to creating a new one.
* NEVER proactively create documentation files (*.md) or README files. Only create documentation files if explicitly requested by the User.
* When you update or modify core context files, also update markdown documentation and memory bank
* When asked to commit changes, exclude CLAUDE.md and CLAUDE-*.md referenced memory bank system files from any commits. Never delete these files.

## Memory Bank System

This project uses a structured memory bank system with specialized context files. Always check these files for relevant information before starting work:

### Core Context Files

* **CLAUDE-activeContext.md** - Current session state, goals, and progress (if exists)
* **CLAUDE-patterns.md** - Established code patterns and conventions (if exists)
* **CLAUDE-decisions.md** - Architecture decisions and rationale (if exists)
* **CLAUDE-troubleshooting.md** - Common issues and proven solutions (if exists)
* **CLAUDE-config-variables.md** - Configuration variables reference (if exists)
* **CLAUDE-temp.md** - Temporary scratch pad (only read when referenced)

**Important:** Always reference the active context file first to understand what's currently being worked on and maintain session continuity.

### Memory Bank System Backups

When asked to backup Memory Bank System files, you will copy the core context files above and @.claude settings directory to directory @/path/to/backup-directory. If files already exist in the backup directory, you will overwrite them.



## Visual Development

### Design Principles
- Comprehensive design checklist in `/context/design-principles.md`
- Brand style guide in `/context/style-guide.md`
- When making visual (front-end, UI/UX) changes, always refer to these files for guidance

### Quick Visual Check
IMMEDIATELY after implementing any front-end change:
1. **Identify what changed** - Review the modified components/pages
2. **Navigate to affected pages** - Use `mcp__playwright__browser_navigate` to visit each changed view
3. **Verify design compliance** - Compare against `/context/design-principles.md` and `/context/style-guide.md`
4. **Validate feature implementation** - Ensure the change fulfills the user's specific request
5. **Check acceptance criteria** - Review any provided context files or requirements
6. **Capture evidence** - Take full page screenshot at desktop viewport (1440px) of each changed view
7. **Check for errors** - Run `mcp__playwright__browser_console_messages`

This verification ensures changes meet design standards and user requirements.

### Comprehensive Design Review
Invoke the `@agent-design-review` subagent for thorough design validation when:
- Completing significant UI/UX features
- Before finalizing PRs with visual changes
- Needing comprehensive accessibility and responsiveness testing


## Project Overview

### Purpose
Local testing environment for Apache Pulsar with Kubernetes (Minikube) and comprehensive performance testing tools.

### Current Status
- **Phase**: Test tools complete, infrastructure operational
- **Completed**:
  - Monitoring infrastructure (Prometheus + 6 Grafana dashboards)
  - Minikube Helm values
  - Performance testing tools (producer + consumer in Go)

### Technology Stack
- **Orchestration**: Kubernetes (Minikube)
- **Message Broker**: Apache Pulsar
- **Monitoring**: Prometheus + Grafana
- **Testing Tools**: Go 1.25.1+ with pulsar-client-go, tview terminal UI

### Key Files
- `helm/` - Self-contained Helm chart for Pulsar deployment
  - `helm/Chart.yaml` - Chart definition with Pulsar dependency
  - `helm/values.yaml` - Minikube-optimized configuration
- `monitoring/` - Observability configurations (Prometheus + Grafana)
- `test-tools/` - Go-based performance testing tools
  - `test-tools/cmd/producer/` - Producer CLI application
  - `test-tools/cmd/consumer/` - Consumer CLI application
  - `test-tools/internal/` - Core implementation (config, metrics, workers, UI)
  - `test-tools/pkg/ratelimit/` - Reusable rate limiter
- `CLAUDE-*.md` - Memory bank system files (exclude from commits)

### Quick Commands

#### Infrastructure
```bash
# Download Helm dependencies (one-time)
cd helm && helm dependency update && cd ..

# Deploy to Minikube
helm install pulsar ./helm --namespace pulsar --create-namespace

# Access Grafana dashboards
kubectl port-forward svc/grafana 3000:3000
```

#### Test Tools
```bash
# Build test tools
cd test-tools && make build

# Run producer with default profile
./bin/producer

# Run consumer with high-throughput profile
./bin/consumer -profile high-throughput

# Run with custom config
./bin/producer -profile sustained -config myconfig.json

# Run tests
make test

# Run benchmarks
make bench
```

### Development Guidelines
1. Optimize for learning and experimentation
2. Document architectural decisions in CLAUDE-decisions.md
3. Update memory bank files when making significant changes
4. Test tools use lock-free concurrency patterns for high performance
5. Use performance profiles for common testing scenarios
