# LDAP to Google Workspace Sync

A lightweight Go application that synchronizes users, groups, and organizational units from OpenLDAP to Google Workspace using the Admin SDK.

## Features

- **User synchronization**: create, update, suspend, or delete
- **Group synchronization**: create, update, delete, and manage members
- **Organizational Unit (OU) synchronization**: automatic creation
- **Daemon mode**: run continuously with configurable intervals
- **Exponential backoff**: automatic retry for Google API rate limits
- **Dry-run mode**: test safely before applying changes
- **Optimized Docker image**: scratch base (~10MB)
- **Graceful shutdown**: handles SIGTERM/SIGINT signals

## Quick Start

```bash
# Build the Docker image
docker build -t ldap-google-sync:latest .

# Run with minimal configuration (dry-run mode)
docker run --rm \
  -v /path/to/credentials.json:/secrets/credentials.json:ro \
  -e LDAP_HOST=ldap.example.com \
  -e LDAP_BASE_DN=dc=example,dc=com \
  -e GOOGLE_CREDENTIALS_FILE=/secrets/credentials.json \
  -e GOOGLE_ADMIN_EMAIL=admin@example.com \
  -e GOOGLE_DOMAIN=example.com \
  ldap-google-sync:latest
```

## Prerequisites

### Google Cloud Platform Setup

1. **Create a project** in [Google Cloud Console](https://console.cloud.google.com)

2. **Enable the Admin SDK API**:
   ```bash
   gcloud services enable admin.googleapis.com
   ```

3. **Create a Service Account**:
   - Go to IAM & Admin → Service Accounts → Create
   - Download the JSON key file

4. **Configure Domain-Wide Delegation**:
   - Go to [Google Admin Console](https://admin.google.com) → Security → API Controls → Domain-wide delegation
   - Click "Add new" and enter the Service Account Client ID
   - Add the following OAuth scopes:
     ```
     https://www.googleapis.com/auth/admin.directory.user
     https://www.googleapis.com/auth/admin.directory.group
     https://www.googleapis.com/auth/admin.directory.orgunit
     ```

## Configuration

All configuration is done through environment variables.

### Sync Interval

| Variable | Description | Default |
|----------|-------------|---------|
| `SYNC_INTERVAL` | How often to run sync (e.g., `1h`, `30m`, `2h30m`). Set to `0` or omit for single run | `0` (once) |

### LDAP Connection

| Variable | Description | Default |
|----------|-------------|---------|
| `LDAP_HOST` | LDAP server hostname | `localhost` |
| `LDAP_PORT` | LDAP server port | `389` |
| `LDAP_USE_TLS` | Use direct TLS connection | `false` |
| `LDAP_BIND_DN` | DN for authentication | - |
| `LDAP_BIND_PASSWORD` | Bind password | - |
| `LDAP_BASE_DN` | Base DN for searches | **required** |
| `LDAP_USER_FILTER` | LDAP filter for users | `(objectClass=inetOrgPerson)` |
| `LDAP_GROUP_FILTER` | LDAP filter for groups | `(|(objectClass=groupOfNames)(objectClass=posixGroup))` |
| `LDAP_GROUP_BASE_DN` | Base DN for groups | same as `LDAP_BASE_DN` |

### LDAP User Attributes

| Variable | Description | Default |
|----------|-------------|---------|
| `LDAP_ATTR_UID` | Username attribute | `uid` |
| `LDAP_ATTR_EMAIL` | Email attribute | `mail` |
| `LDAP_ATTR_FIRSTNAME` | First name attribute | `givenName` |
| `LDAP_ATTR_LASTNAME` | Last name attribute | `sn` |
| `LDAP_ATTR_PHONE` | Phone attribute | `telephoneNumber` |
| `LDAP_ATTR_DEPARTMENT` | Department attribute | `departmentNumber` |
| `LDAP_ATTR_TITLE` | Job title attribute | `title` |
| `LDAP_ATTR_ORG_UNIT` | Organizational unit attribute | `ou` |

### LDAP Group Attributes

| Variable | Description | Default |
|----------|-------------|---------|
| `LDAP_ATTR_GROUP_NAME` | Group name attribute | `cn` |
| `LDAP_ATTR_GROUP_EMAIL` | Group email attribute | `mail` |
| `LDAP_ATTR_GROUP_DESC` | Group description attribute | `description` |
| `LDAP_ATTR_GROUP_MEMBER` | Group member attribute | `memberUid` |

### Google Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `GOOGLE_CREDENTIALS_FILE` | Path to service account JSON | **required** |
| `GOOGLE_ADMIN_EMAIL` | Admin email for impersonation | **required** |
| `GOOGLE_DOMAIN` | Google Workspace domain | **required** |

### Sync Options

| Variable | Description | Default |
|----------|-------------|---------|
| `SYNC_DRY_RUN` | Simulate changes without applying | `true` |

### User Sync Options

| Variable | Description | Default |
|----------|-------------|---------|
| `SYNC_USERS` | Enable user synchronization | `true` |
| `SYNC_CREATE_USERS` | Create new users in Google | `true` |
| `SYNC_UPDATE_USERS` | Update existing users | `true` |
| `SYNC_SUSPEND_MISSING_USERS` | Suspend users not in LDAP | `false` |
| `SYNC_DELETE_INSTEAD_OF_SUSPEND` | Delete instead of suspend | `false` |
| `SYNC_DEFAULT_ORG_UNIT` | Default OU for new users | `/` |

### Group Sync Options

| Variable | Description | Default |
|----------|-------------|---------|
| `SYNC_GROUPS` | Enable group synchronization | `false` |
| `SYNC_CREATE_GROUPS` | Create new groups | `true` |
| `SYNC_UPDATE_GROUPS` | Update existing groups | `true` |
| `SYNC_DELETE_MISSING_GROUPS` | Delete groups not in LDAP | `false` |
| `SYNC_GROUP_MEMBERS` | Synchronize group members | `true` |
| `SYNC_GROUP_EMAIL_SUFFIX` | Email suffix for groups | `@GOOGLE_DOMAIN` |

### Organizational Unit Sync Options

| Variable | Description | Default |
|----------|-------------|---------|
| `SYNC_ORG_UNITS` | Enable OU synchronization | `false` |
| `SYNC_CREATE_ORG_UNITS` | Create missing OUs | `true` |

## Usage Examples

### Single Execution (Users Only)

```bash
docker run --rm \
  -v /path/to/credentials.json:/secrets/credentials.json:ro \
  -e LDAP_HOST=ldap.example.com \
  -e LDAP_BASE_DN=ou=users,dc=example,dc=com \
  -e GOOGLE_CREDENTIALS_FILE=/secrets/credentials.json \
  -e GOOGLE_ADMIN_EMAIL=admin@example.com \
  -e GOOGLE_DOMAIN=example.com \
  -e SYNC_DRY_RUN=false \
  ldap-google-sync:latest
```

### Daemon Mode (Run Every Hour)

```bash
docker run -d \
  --name ldap-google-sync \
  --restart unless-stopped \
  -v /path/to/credentials.json:/secrets/credentials.json:ro \
  -e SYNC_INTERVAL=1h \
  -e LDAP_HOST=ldap.example.com \
  -e LDAP_BASE_DN=dc=example,dc=com \
  -e GOOGLE_CREDENTIALS_FILE=/secrets/credentials.json \
  -e GOOGLE_ADMIN_EMAIL=admin@example.com \
  -e GOOGLE_DOMAIN=example.com \
  -e SYNC_DRY_RUN=false \
  ldap-google-sync:latest
```

### Full Sync (Users + Groups + OUs)

```bash
docker run --rm \
  -v /path/to/credentials.json:/secrets/credentials.json:ro \
  -e LDAP_HOST=ldap.example.com \
  -e LDAP_BASE_DN=dc=example,dc=com \
  -e GOOGLE_CREDENTIALS_FILE=/secrets/credentials.json \
  -e GOOGLE_ADMIN_EMAIL=admin@example.com \
  -e GOOGLE_DOMAIN=example.com \
  -e SYNC_DRY_RUN=false \
  -e SYNC_GROUPS=true \
  -e SYNC_ORG_UNITS=true \
  ldap-google-sync:latest
```

### Kubernetes CronJob (Alternative to Daemon Mode)

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: ldap-google-sync
spec:
  schedule: "0 * * * *"  # Every hour
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: sync
            image: ldap-google-sync:latest
            envFrom:
            - secretRef:
                name: ldap-google-sync-config
            volumeMounts:
            - name: google-credentials
              mountPath: /secrets
              readOnly: true
          volumes:
          - name: google-credentials
            secret:
              secretName: google-credentials
          restartPolicy: OnFailure
```

### Local Development

```bash
# Build
go build -o ldap-google-sync ./cmd/sync

# Run once
./ldap-google-sync

# Run every 30 minutes
SYNC_INTERVAL=30m ./ldap-google-sync
```

## Sync Interval Formats

The `SYNC_INTERVAL` variable accepts Go duration format:

| Value | Description |
|-------|-------------|
| `0` | Run once and exit |
| `30m` | Every 30 minutes |
| `1h` | Every hour |
| `2h30m` | Every 2 hours and 30 minutes |
| `24h` | Every 24 hours |

Minimum interval: `1m` (1 minute)

## Supported LDAP Group Types

The application supports both common LDAP group types:

| Group Type | Member Attribute | Member Format |
|------------|------------------|---------------|
| `posixGroup` | `memberUid` | Username only (e.g., `john.doe`) |
| `groupOfNames` | `member` | Full DN (e.g., `cn=john,ou=users,dc=example,dc=com`) |

The default configuration uses `memberUid` for `posixGroup`. If you use `groupOfNames`, set:

```bash
LDAP_GROUP_FILTER=(objectClass=groupOfNames)
LDAP_ATTR_GROUP_MEMBER=member
```

## Protected Accounts

The following accounts are automatically protected from suspension/deletion:

- `admin@*`
- `administrator@*`
- `postmaster@*`
- `abuse@*`
- `security@*`
- `*@*.iam.gserviceaccount.com` (Google service accounts)

## Google API Quotas

- **Rate limit**: 1,500 queries per 100 seconds
- **Automatic retry**: Exponential backoff for `rateLimitExceeded` and `503` errors
- **Max retries**: 5 attempts per operation

## Building

### Docker

```bash
docker build -t ldap-google-sync:latest .
```

### Binary

```bash
# Build for current platform
go build -o ldap-google-sync ./cmd/sync

# Build for Linux (for containers)
GOOS=linux GOARCH=amd64 go build -o ldap-google-sync ./cmd/sync
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run with verbose output
go test ./... -v
```

## License

MIT
