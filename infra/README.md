# bondcalc Infrastructure

AWS ECS Fargate deployment. Single stateless service behind an ALB. No database.

## Prerequisites

- AWS CLI configured (`aws sts get-caller-identity` works)
- Terraform 1.9+
- GitHub OIDC provider created in your AWS account (one-time per account)

## One-time: GitHub OIDC Provider

```bash
aws iam create-open-id-connect-provider \
  --url https://token.actions.githubusercontent.com \
  --client-id-list sts.amazonaws.com \
  --thumbprint-list 6938fd4d98bab03faadb97b34396831e3780aea1
```

## Deploy

```bash
cd infra/terraform

terraform init

terraform plan \
  -var="jwt_secret=<your-secret>"

terraform apply \
  -var="jwt_secret=<your-secret>"
```

Note the outputs:

```
alb_dns_name          = "bondcalc-prod-alb-xxxx.us-east-1.elb.amazonaws.com"
ecr_api_url           = "123456789.dkr.ecr.us-east-1.amazonaws.com/bondcalc-prod-api"
ecs_cluster_name      = "bondcalc-prod-cluster"
ecs_api_service_name  = "bondcalc-prod-api"
ecs_api_task_family   = "bondcalc-prod-api"
github_actions_role_arn = "arn:aws:iam::123456789:role/bondcalc-prod-github-actions"
```

## GitHub Secrets

Set these in the repo settings (Settings > Secrets > Actions):

| Secret | Where to get it |
|---|---|
| `AWS_ROLE_ARN` | `github_actions_role_arn` output |
| `ECR_API_REPO` | `ecr_api_url` output |
| `ECS_TASK_FAMILY` | `ecs_api_task_family` output |
| `ECS_SERVICE` | `ecs_api_service_name` output |
| `ECS_CLUSTER` | `ecs_cluster_name` output |
| `PRODUCTION_URL` | `http://<alb_dns_name>` |

## First Image Push

After `terraform apply` and GitHub secrets are set, push a commit to `main`. CI runs tests, builds the Docker image, pushes to ECR, and deploys to ECS automatically.

For the first deploy only you may need to push the image manually since ECS starts with `latest` as a placeholder:

```bash
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin <account_id>.dkr.ecr.us-east-1.amazonaws.com

docker build -t <ecr_api_url>:latest .
docker push <ecr_api_url>:latest

aws ecs update-service \
  --cluster bondcalc-prod-cluster \
  --service bondcalc-prod-api \
  --force-new-deployment
```

## Rollback

```bash
# Find the previous task definition revision
aws ecs list-task-definitions --family-prefix bondcalc-prod-api --sort DESC

# Update service to use it
aws ecs update-service \
  --cluster bondcalc-prod-cluster \
  --service bondcalc-prod-api \
  --task-definition bondcalc-prod-api:<previous-revision>
```

## Cost Estimate

| Resource | Monthly |
|---|---|
| ECS Fargate 256 CPU / 512 MB (1 task) | ~$7 |
| ALB | ~$18 |
| NAT Gateway | ~$5 |
| ECR storage | ~$1 |
| CloudWatch logs | ~$1 |
| **Total** | **~$32/mo** |

Shut down the NAT Gateway and task when not demoing to minimize cost.
