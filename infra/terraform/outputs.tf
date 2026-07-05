output "alb_dns_name" {
  value       = aws_alb.main.dns_name
  description = "ALB DNS name -- point your CNAME here."
}

output "ecr_api_url" {
  value = aws_ecr_repository.api.repository_url
}

output "ecs_cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "ecs_api_service_name" {
  value = aws_ecs_service.api.name
}

output "ecs_api_task_family" {
  value = aws_ecs_task_definition.api.family
}

output "app_secret_arn" {
  value     = aws_secretsmanager_secret.app.arn
  sensitive = true
}

output "github_actions_role_arn" {
  value = aws_iam_role.github_actions.arn
}
