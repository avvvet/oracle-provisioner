# OCI ARM A1 Instance Provisioner (Go)

Automatically retries creating an Oracle Cloud ARM Ampere A1 instance until capacity becomes available.

## Prerequisites

- Go 1.21+
- OCI CLI configured (`~/.oci/config`) with your credentials
- An existing VCN + Subnet in your OCI tenancy

## Setup

### 1. Install Go on your Oracle AMD VM
```bash
sudo apt update && sudo apt install golang-go -y
# or for latest Go:
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### 2. Set up OCI config (if not already done)
```bash
oci setup config
# Follow the prompts - you'll need your tenancy OCID, user OCID, and region
```

### 3. Clone / copy this project
```bash
mkdir oci-arm-provisioner && cd oci-arm-provisioner
# Copy main.go, go.mod, .env.example here
```

### 4. Get dependencies
```bash
go mod tidy
```

### 5. Configure environment variables
```bash
cp .env.example .env
nano .env   # Fill in your values
source .env
```

#### Finding your values in OCI Console:

| Variable | Where to find it |
|---|---|
| `OCI_COMPARTMENT_ID` | Identity > Compartments (or Tenancy Details for root) |
| `OCI_SUBNET_ID` | Networking > Virtual Cloud Networks > your VCN > Subnets |
| `OCI_IMAGE_ID` | Compute > Images > Platform Images > filter "aarch64" |
| `OCI_SSH_PUBLIC_KEY` | Contents of `~/.ssh/id_rsa.pub` on your local machine |
| `OCI_AVAILABILITY_DOMAINS` | OCI Console > top-right region selector shows AD names |

### 6. Run it
```bash
# Test once
go run main.go

# Run in background (keeps retrying until success)
nohup go run main.go > provisioner.log 2>&1 &

# Watch the log
tail -f provisioner.log
```

### 7. Run as a cron job instead (alternative)
```bash
# Build binary first
go build -o oci-arm-provisioner main.go

# Add to crontab (runs every 5 minutes)
crontab -e
# Add this line:
*/5 * * * * source /home/ubuntu/oci-arm-provisioner/.env && /home/ubuntu/oci-arm-provisioner/oci-arm-provisioner >> /home/ubuntu/provisioner.log 2>&1
```

## Notes

- The script tries **all availability domains** on each attempt
- Default retry interval is **5 minutes** (configurable)
- Most users get their instance within **24–72 hours**
- Once created, check OCI Console > Compute > Instances for the public IP
- Max free tier: 4 OCPUs + 24 GB RAM total across all A1 instances