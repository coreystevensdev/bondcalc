output "api_public_ip" {
  description = "EC2 public IP for the API"
  value       = aws_instance.api.public_ip
}

output "ecr_repository_url" {
  description = "ECR repository URL for pushing images"
  value       = aws_ecr_repository.api.repository_url
}

output "instance_id" {
  description = "EC2 instance ID for the deploy workflow's EC2_INSTANCE_ID secret"
  value       = aws_instance.api.id
}

output "github_actions_role_arn" {
  value = aws_iam_role.github_actions.arn
}
