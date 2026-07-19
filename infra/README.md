# bondcalc Infrastructure

AWS deployment using Terraform. Single EC2 t2.micro running the container directly, no ALB, no NAT Gateway, no database. Free-tier eligible for 12 months on a new AWS account.

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
| `PRODUCTION_URL` | `http://<api_public_ip>:8080` |
| `JWT_SECRET` | same value as `terraform.tfvars` |

After that, every push to `main` triggers the deploy workflow automatically: it builds and pushes a new image, then uses SSM to pull and restart the container on the instance.

## Rollback

```bash
aws ssm send-command \
  --instance-ids <instance_id> \
  --document-name "AWS-RunShellScript" \
  --parameters 'commands=["docker stop api", "docker rm api", "docker run -d --name api --restart=always -p 8080:8080 -e JWT_SECRET=<secret> <ecr_repository_url>:sha-<previous-sha>"]'
```

## Cost Estimate

| Resource | Monthly |
|---|---|
| EC2 t2.micro (free tier, 750 hrs/mo for 12 months) | $0 |
| ECR storage (free tier, 500 MB for 12 months) | $0 |
| **Total** | **~$0/mo for the first 12 months** |

After the free-tier window, EC2 t2.micro runs about $8/mo. `terraform destroy` tears everything down between demos.
