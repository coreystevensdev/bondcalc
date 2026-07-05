resource "aws_secretsmanager_secret" "app" {
  name                    = "${local.name_prefix}/app"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "app" {
  secret_id = aws_secretsmanager_secret.app.id
  secret_string = jsonencode({
    JWT_SECRET = var.jwt_secret
  })

  # Prevent Terraform from overwriting secrets updated outside of IaC.
  lifecycle {
    ignore_changes = [secret_string]
  }
}
