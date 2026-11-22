# bondcalc Infrastructure

AWS deployment using Terraform. Single EC2 instance running the container directly, no ALB, no NAT Gateway, no database.

Terraform declares a `t3.micro` (`terraform/ec2.tf`). The instance actually serving this is a `t3.small` sharing the box with five other demos behind the same Caddy, so the config and the running instance disagree on size. Reconcile that before relying on a `terraform apply` here.

## Prerequisites

- AWS CLI configured (`aws sts get-caller-identity` works)
- Terraform 1.9+
- Docker + ECR access

## First-time setup

### 1. Create the S3 backend bucket

```bash
aws s3api create-bucket \
  --bucket coreystevensdev-tfstate \
  --region us-east-1
aws s3api put-bucket-versioning \
  --bucket coreystevensdev-tfstate \
  --versioning-configuration Status=Enabled
```

Skip this step if the bucket already exists from another project's deploy.

### 2. Create the GitHub OIDC provider (one-time per AWS account)

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

Skip this step if it already exists from another project's deploy.

### 3. Create terraform.tfvars

```bash
cd infra/terraform
cp terraform.tfvars.example terraform.tfvars
# fill in jwt_secret and ecr_image_uri
```

### 4. Apply

```bash
terraform init
terraform plan
terraform apply
```

Note the outputs:

```
api_public_ip           = "54.x.x.x"
ecr_repository_url      = "123456789.dkr.ecr.us-east-1.amazonaws.com/bondcalc"
instance_id             = "i-0abc123def456"
github_actions_role_arn = "arn:aws:iam::123456789:role/bondcalc-github-actions"
```

### 5. Push the first image

The instance boots with `var.ecr_image_uri` as its initial image, so push that tag before the first CI deploy:

```bash
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin <account_id>.dkr.ecr.us-east-1.amazonaws.com

docker build -t <ecr_repository_url>:latest .
docker push <ecr_repository_url>:latest
```

### 6. Add GitHub secrets

Set these in the repo settings (Settings > Secrets > Actions):

| Secret | Where to get it |
|---|---|
| `AWS_ROLE_ARN` | `github_actions_role_arn` output |
| `ECR_API_REPO` | `ecr_repository_url` output |
| `EC2_INSTANCE_ID` | `instance_id` output |
| `PRODUCTION_URL` | `https://bondcalc.coreystevens.dev` (the deploy smoke-tests `$PRODUCTION_URL/health`, and 8080 is closed at the security group, so an `http://<ip>:8080` value fails that step) |

The JWT secret isn't a GitHub secret. It lives in SSM Parameter Store (`/bondcalc/jwt-secret`, set from `terraform.tfvars` at apply time) and `/usr/local/bin/deploy.sh` on the instance reads it directly, so it never appears in a workflow log or an SSM command string.

After that the deploy workflow runs on a `workflow_run` of CI against `main`, gated on CI having succeeded, so a commit whose tests failed never reaches the box. A merge alone does not deploy; a green CI does. The workflow builds and pushes a new image, then uses SSM to invoke `deploy.sh` with the new tag. `workflow_dispatch` is still there with a `force` input for deploying a commit CI has not passed.

## Rollback

```bash
aws ssm send-command \
  --instance-ids <instance_id> \
  --document-name "AWS-RunShellScript" \
  --parameters 'commands=["/usr/local/bin/deploy.sh <ecr_repository_url>:sha-<previous-sha>"]'
```

## Cost Estimate

| Resource | Monthly |
|---|---|
| EC2 t3.small, on-demand, us-east-1 | about $15 |
| ECR storage (free tier, 500 MB for 12 months) | $0 |

That is the whole box, not bondcalc's share: six demos run on it, so the per-service figure is roughly a sixth of the instance line. The earlier `$0 on the free tier` estimate assumed a `t3.micro` on a new account, and neither half of that holds now: the instance is a `t3.small`, which is not free-tier eligible, and the account is on the paid plan. `terraform destroy` still tears down everything Terraform owns.
